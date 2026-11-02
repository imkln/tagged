package worker

import (
	"bytes"
	"errors"
	"image/jpeg"
	"sync"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestWorkerProcessesJPEG(t *testing.T) {
	tests := []struct {
		name            string
		width           int
		height          int
		thumbnailWidth  int
		thumbnailHeight int
	}{
		{name: "landscape", width: 800, height: 400, thumbnailWidth: 320, thumbnailHeight: 160},
		{name: "portrait", width: 400, height: 800, thumbnailWidth: 160, thumbnailHeight: 320},
		{name: "small image", width: 80, height: 40, thumbnailWidth: 80, thumbnailHeight: 40},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture, photo, _ := pendingJPEG(t, test.width, test.height)

			if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
				t.Fatalf("process uploaded image: %v", err)
			}

			thumbnail, err := jpeg.Decode(bytes.NewReader(fixture.objects.data[photo.ThumbnailKey]))
			if err != nil {
				t.Fatalf("decode generated thumbnail: %v", err)
			}

			if thumbnail.Bounds().Dx() != test.thumbnailWidth || thumbnail.Bounds().Dy() != test.thumbnailHeight {
				t.Errorf("thumbnail bounds = %v, want %d by %d", thumbnail.Bounds(), test.thumbnailWidth, test.thumbnailHeight)
			}

			if fixture.objects.contentType[photo.ThumbnailKey] != "image/jpeg" {
				t.Error("thumbnail was not stored as image/jpeg")
			}

			saved, err := fixture.repository.Get(t.Context(), photo.ID)
			if err != nil {
				t.Fatalf("read processed photo: %v", err)
			}

			if saved.Status != "ready" || saved.ThumbnailKey == "" {
				t.Errorf("processed photo = %+v, want ready with a thumbnail", saved)
			}
		})
	}
}

func TestWorkerPreservesOriginalBytes(t *testing.T) {
	fixture, photo, original := pendingJPEG(t, 800, 400)

	if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
		t.Fatalf("process uploaded image: %v", err)
	}

	if !bytes.Equal(fixture.objects.data[photo.OriginalKey], original) {
		t.Error("worker changed the original image")
	}
}

func TestWorkerRejectsInvalidJPEG(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "non-image bytes", data: []byte("fictional image placeholder")},
		{name: "truncated JPEG", data: []byte{0xff, 0xd8, 0xff}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newWorkerFixture()
			photo := fixture.seed("pending")
			photo.Size = int64(len(test.data))
			fixture.repository.records[photo.ID] = photo
			fixture.objects.data[photo.OriginalKey] = test.data

			if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
				t.Fatalf("record terminal image validation failure: %v", err)
			}

			requireFailedPhoto(t, fixture, photo.ID)
		})
	}
}

func TestWorkerRejectsUploadedSizeMismatch(t *testing.T) {
	fixture, photo, _ := pendingJPEG(t, 80, 40)
	photo.Size++
	fixture.repository.records[photo.ID] = photo

	if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
		t.Fatalf("record terminal image validation failure: %v", err)
	}

	requireFailedPhoto(t, fixture, photo.ID)
}

func TestWorkerReportsStorageFailure(t *testing.T) {
	fixture, photo, _ := pendingJPEG(t, 80, 40)
	fixture.objects.putFailure = errors.New("fictional storage unavailable")

	err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo))

	if err == nil {
		t.Error("worker swallowed thumbnail storage failure")
	}

	saved, err := fixture.repository.Get(t.Context(), photo.ID)
	if err != nil {
		t.Fatalf("read photo after storage failure: %v", err)
	}

	if saved.Status == "ready" {
		t.Error("photo became ready without a stored thumbnail")
	}
}

func TestWorkerHandlesDuplicateEvents(t *testing.T) {
	tests := []struct {
		name       string
		concurrent bool
	}{
		{name: "sequential delivery", concurrent: false},
		{name: "concurrent delivery", concurrent: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture, photo, _ := pendingJPEG(t, 80, 40)
			event := photoCreatedEvent(fixture, photo)
			results := make(chan error, 2)
			var workers sync.WaitGroup

			for range 2 {
				if test.concurrent {
					workers.Go(func() {
						results <- fixture.worker.HandleS3Event(t.Context(), event)
					})
				} else {
					results <- fixture.worker.HandleS3Event(t.Context(), event)
				}
			}
			workers.Wait()
			close(results)

			for err := range results {
				if err != nil {
					t.Errorf("duplicate event failed: %v", err)
				}
			}

			if len(fixture.objects.writes) != 1 || len(fixture.repository.records) != 1 {
				t.Errorf("thumbnail writes = %d; records = %d, want one of each", len(fixture.objects.writes), len(fixture.repository.records))
			}
		})
	}
}

func TestWorkerIgnoresUnrelatedEvents(t *testing.T) {
	tests := []struct {
		name   string
		change func(*events.S3EventRecord)
	}{
		{
			name: "another bucket",
			change: func(record *events.S3EventRecord) {
				record.S3.Bucket.Name = "other-bucket"
			},
		},
		{
			name: "thumbnail output",
			change: func(record *events.S3EventRecord) {
				record.S3.Object.Key = "thumbnails/photo-garden.jpg"
			},
		},
		{
			name: "removed object",
			change: func(record *events.S3EventRecord) {
				record.EventName = "ObjectRemoved:Delete"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture, photo, _ := pendingJPEG(t, 80, 40)
			event := photoCreatedEvent(fixture, photo)
			test.change(&event.Records[0])

			if err := fixture.worker.HandleS3Event(t.Context(), event); err != nil {
				t.Fatalf("ignore unrelated event: %v", err)
			}

			if len(fixture.objects.writes) != 0 {
				t.Error("unrelated event generated a thumbnail")
			}
		})
	}
}

func pendingJPEG(t *testing.T, width, height int) (*workerFixture, photoRecord, []byte) {
	t.Helper()

	fixture := newWorkerFixture()
	photo := fixture.seed("pending")
	original := fictionalJPEG(t, width, height)
	photo.Size = int64(len(original))
	fixture.repository.records[photo.ID] = photo
	fixture.objects.data[photo.OriginalKey] = original

	return fixture, photo, original
}

func photoCreatedEvent(fixture *workerFixture, photo photoRecord) events.S3Event {
	return events.S3Event{Records: []events.S3EventRecord{
		{
			EventSource: "aws:s3",
			EventName:   "ObjectCreated:Put",
			S3: events.S3Entity{
				Bucket: events.S3Bucket{Name: fixture.config.Bucket},
				Object: events.S3Object{Key: photo.OriginalKey, Size: photo.Size, Sequencer: "001"},
			},
		},
	}}
}

func requireFailedPhoto(t *testing.T, fixture *workerFixture, id string) {
	t.Helper()

	photo, err := fixture.repository.Get(t.Context(), id)
	if err != nil {
		t.Fatalf("read failed photo: %v", err)
	}

	if photo.Status != "failed" || photo.Error == "" {
		t.Errorf("photo = %+v, want failed with an explanation", photo)
	}

	if len(fixture.objects.writes) != 0 {
		t.Error("invalid image produced a thumbnail")
	}
}
