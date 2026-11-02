package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"golang.org/x/image/draw"
)

var (
	errPhotoNotFound = errors.New("photo not found")
	errPhotoConflict = errors.New("photo state changed")
)

type photoRecord struct {
	ID           string `dynamodbav:"id"`
	Filename     string `dynamodbav:"filename"`
	ContentType  string `dynamodbav:"contentType"`
	Size         int64  `dynamodbav:"size"`
	Status       string `dynamodbav:"status"`
	OriginalKey  string `dynamodbav:"originalKey"`
	ThumbnailKey string `dynamodbav:"thumbnailKey"`
	Error        string `dynamodbav:"error,omitempty"`
}

type workerConfig struct {
	Bucket        string
	ThumbnailEdge int
}

type photoRepository interface {
	Get(context.Context, string) (photoRecord, error)
	Update(context.Context, photoRecord, string) error
}

type photoObjects interface {
	Get(context.Context, string) ([]byte, error)
	Put(context.Context, string, string, []byte) error
}

type thumbnailWorker struct {
	repository photoRepository
	objects    photoObjects
	config     workerConfig
}

func newThumbnailWorker(repository photoRepository, objects photoObjects, config workerConfig) *thumbnailWorker {
	return &thumbnailWorker{repository: repository, objects: objects, config: config}
}

func (worker *thumbnailWorker) HandleS3Event(ctx context.Context, event events.S3Event) error {
	for _, record := range event.Records {
		if record.EventSource != "aws:s3" || record.S3.Bucket.Name != worker.config.Bucket || !strings.HasPrefix(record.EventName, "ObjectCreated:") {
			continue
		}

		key, err := url.QueryUnescape(record.S3.Object.Key)
		if err != nil {
			return fmt.Errorf("decode object key: %w", err)
		}

		id, original := strings.CutPrefix(key, "originals/")
		if !original || id == "" || strings.Contains(id, "/") {
			continue
		}

		photo, err := worker.repository.Get(ctx, id)
		if errors.Is(err, errPhotoNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read uploaded photo: %w", err)
		}

		if photo.OriginalKey != key || photo.Status != "pending" {
			continue
		}

		// Claim the record atomically so concurrent S3 deliveries cannot both process it.
		photo.Status = "processing"
		if err := worker.repository.Update(ctx, photo, "pending"); errors.Is(err, errPhotoConflict) {
			continue
		} else if err != nil {
			return fmt.Errorf("claim uploaded photo: %w", err)
		}

		if err := worker.process(ctx, photo); err != nil {
			return err
		}
	}

	return nil
}

func (worker *thumbnailWorker) process(ctx context.Context, photo photoRecord) error {
	data, err := worker.objects.Get(ctx, photo.OriginalKey)
	if err != nil {
		return worker.fail(ctx, photo, "Unable to read the uploaded image.", err)
	}

	if int64(len(data)) != photo.Size {
		return worker.fail(ctx, photo, "Uploaded size does not match the selected file.", nil)
	}

	original, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return worker.fail(ctx, photo, "The uploaded file is not a valid JPEG.", nil)
	}

	width, height := original.Bounds().Dx(), original.Bounds().Dy()
	if width > worker.config.ThumbnailEdge || height > worker.config.ThumbnailEdge {
		if width >= height {
			height = max(1, height*worker.config.ThumbnailEdge/width)
			width = worker.config.ThumbnailEdge
		} else {
			width = max(1, width*worker.config.ThumbnailEdge/height)
			height = worker.config.ThumbnailEdge
		}
	}

	thumbnail := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(thumbnail, thumbnail.Bounds(), original, original.Bounds(), draw.Src, nil)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, thumbnail, &jpeg.Options{Quality: 85}); err != nil {
		return worker.fail(ctx, photo, "Unable to generate the thumbnail.", err)
	}

	if err := worker.objects.Put(ctx, photo.ThumbnailKey, "image/jpeg", encoded.Bytes()); err != nil {
		return worker.fail(ctx, photo, "Unable to store the thumbnail.", err)
	}

	photo.Status = "ready"
	photo.Error = ""
	if err := worker.repository.Update(ctx, photo, "processing"); err != nil {
		return fmt.Errorf("save completed photo: %w", err)
	}

	return nil
}

func (worker *thumbnailWorker) fail(ctx context.Context, photo photoRecord, message string, cause error) error {
	photo.Status = "failed"
	photo.Error = message
	return errors.Join(cause, worker.repository.Update(ctx, photo, "processing"))
}
