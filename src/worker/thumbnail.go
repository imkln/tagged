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
	"github.com/disintegration/imaging"
	"github.com/imkln/tagged/src/tags"
	"golang.org/x/image/draw"
)

const DefaultMaxImagePixels int64 = 40_000_000

var (
	errPhotoNotFound = errors.New("photo not found")
	errPhotoConflict = errors.New("photo state changed")
)

type photoRecord struct {
	Attributions []Attribution `dynamodbav:"attributions,omitempty"`
	Tags         []tags.Tag    `dynamodbav:"tags,omitempty"`
	ID           string        `dynamodbav:"id"`
	Filename     string        `dynamodbav:"filename"`
	ContentType  string        `dynamodbav:"contentType"`
	Size         int64         `dynamodbav:"size"`
	Status       string        `dynamodbav:"status"`
	OriginalKey  string        `dynamodbav:"originalKey"`
	ThumbnailKey string        `dynamodbav:"thumbnailKey"`
	Error        string        `dynamodbav:"error,omitempty"`
	UploadedAt   string        `dynamodbav:"uploadedAt,omitempty"`
	CapturedAt   string        `dynamodbav:"capturedAt,omitempty"`
	Width        int           `dynamodbav:"width,omitempty"`
	Height       int           `dynamodbav:"height,omitempty"`
	CameraMake   string        `dynamodbav:"cameraMake,omitempty"`
	CameraModel  string        `dynamodbav:"cameraModel,omitempty"`
	Latitude     *float64      `dynamodbav:"latitude,omitempty"`
	Longitude    *float64      `dynamodbav:"longitude,omitempty"`
}

type workerConfig struct {
	Bucket         string
	ThumbnailEdge  int
	MaxImagePixels int64
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
	places     PlaceResolver
	repository photoRepository
	objects    photoObjects
	config     workerConfig
}

func newThumbnailWorker(repository photoRepository, objects photoObjects, config workerConfig, places PlaceResolver) *thumbnailWorker {
	return &thumbnailWorker{repository: repository, objects: objects, config: config, places: places}
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

	dimensions, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return worker.fail(ctx, photo, "The uploaded file is not a valid JPEG.", nil)
	}

	if int64(dimensions.Width) > worker.config.MaxImagePixels/int64(dimensions.Height) {
		return worker.fail(ctx, photo, "The uploaded image exceeds the configured pixel limit.", nil)
	}

	original, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return worker.fail(ctx, photo, "The uploaded file is not a valid JPEG.", nil)
	}

	photo.Width, photo.Height = original.Bounds().Dx(), original.Bounds().Dy()
	embedded := extractMetadata(data, &photo)

	width, height := photo.Width, photo.Height
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

	if worker.places != nil && photo.Latitude != nil && photo.Longitude != nil && (embedded.Country == "" || embedded.City == "") {
		location, err := worker.places.Resolve(ctx, *photo.Latitude, *photo.Longitude)
		if err == nil {
			usedLookup := false
			if embedded.Country == "" && location.Country != "" {
				embedded.Country = location.Country
				usedLookup = true
			}

			if embedded.City == "" && location.City != "" {
				embedded.City = location.City
				usedLookup = true
			}

			photo.Tags = tags.ReplaceSource(photo.Tags, []string{embedded.Country, embedded.City}, "metadata")
			if usedLookup {
				photo.Attributions = location.Attributions
			}
		}
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
