package worker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestWorkerAcceptsImageAtPixelLimit(t *testing.T) {
	fixture, photo, _ := pendingJPEG(t, 80, 40)
	fixture.worker.config.MaxImagePixels = 3200

	if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
		t.Fatalf("process image at configured limit: %v", err)
	}

	saved, err := fixture.repository.Get(t.Context(), photo.ID)
	if err != nil {
		t.Fatalf("read processed photo: %v", err)
	}
	if saved.Status != "ready" {
		t.Errorf("photo status = %q, want ready", saved.Status)
	}
}

func TestWorkerRejectsImageAbovePixelLimit(t *testing.T) {
	fixture, photo, data := pendingJPEG(t, 80, 40)
	fixture.worker.config.MaxImagePixels = 3199
	// Keep the dimensions but omit scan data: rejection must precede full decoding.
	scan := bytes.Index(data, []byte{0xff, 0xda})
	if scan < 0 {
		t.Fatal("generated JPEG has no scan marker")
	}
	data = data[:scan+4]
	if _, err := jpeg.DecodeConfig(bytes.NewReader(data)); err != nil {
		t.Fatalf("read dimensions from JPEG header fixture: %v", err)
	}
	photo.Size = int64(len(data))
	fixture.repository.records[photo.ID] = photo
	fixture.objects.data[photo.OriginalKey] = data

	if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
		t.Fatalf("record image limit failure: %v", err)
	}

	requireFailedPhoto(t, fixture, photo.ID)
	saved, err := fixture.repository.Get(t.Context(), photo.ID)
	if err != nil {
		t.Fatalf("read rejected photo: %v", err)
	}
	if !strings.Contains(saved.Error, "pixel limit") {
		t.Errorf("photo error = %q, want pixel limit rejection before decoding", saved.Error)
	}
}

func TestWorkerAppliesEXIFOrientation(t *testing.T) {
	colors := []color.NRGBA{
		{R: 240, A: 255}, {G: 240, A: 255}, {B: 240, A: 255}, {R: 240, G: 240, A: 255},
	}
	tests := []struct {
		name        string
		orientation uint16
		corners     [4]int
	}{
		{name: "normal", orientation: 1, corners: [4]int{0, 1, 2, 3}},
		{name: "mirror horizontal", orientation: 2, corners: [4]int{1, 0, 3, 2}},
		{name: "rotate 180", orientation: 3, corners: [4]int{3, 2, 1, 0}},
		{name: "mirror vertical", orientation: 4, corners: [4]int{2, 3, 0, 1}},
		{name: "transpose", orientation: 5, corners: [4]int{0, 2, 1, 3}},
		{name: "rotate clockwise", orientation: 6, corners: [4]int{2, 0, 3, 1}},
		{name: "transverse", orientation: 7, corners: [4]int{3, 1, 2, 0}},
		{name: "rotate counterclockwise", orientation: 8, corners: [4]int{1, 3, 0, 2}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newWorkerFixture()
			photo := fixture.seed("pending")
			data := orientedJPEG(t, test.orientation, colors)
			photo.Size = int64(len(data))
			fixture.repository.records[photo.ID] = photo
			fixture.objects.data[photo.OriginalKey] = data

			if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
				t.Fatalf("process oriented JPEG: %v", err)
			}
			thumbnail, err := jpeg.Decode(bytes.NewReader(fixture.objects.data[photo.ThumbnailKey]))
			if err != nil {
				t.Fatalf("decode thumbnail: %v", err)
			}
			width, height := 80, 40
			if test.orientation >= 5 {
				width, height = height, width
			}
			if thumbnail.Bounds().Dx() != width || thumbnail.Bounds().Dy() != height {
				t.Fatalf("thumbnail bounds = %v, want %d by %d", thumbnail.Bounds(), width, height)
			}
			points := []image.Point{
				{X: width / 4, Y: height / 4},
				{X: 3 * width / 4, Y: height / 4},
				{X: width / 4, Y: 3 * height / 4},
				{X: 3 * width / 4, Y: 3 * height / 4},
			}
			for corner, point := range points {
				got := color.NRGBAModel.Convert(thumbnail.At(point.X, point.Y)).(color.NRGBA)
				want := colors[test.corners[corner]]
				if abs(int(got.R)-int(want.R)) > 15 || abs(int(got.G)-int(want.G)) > 15 || abs(int(got.B)-int(want.B)) > 15 {
					t.Errorf("corner %d = %v, want approximately %v", corner, got, want)
				}
			}
		})
	}
}

func orientedJPEG(t *testing.T, orientation uint16, colors []color.NRGBA) []byte {
	t.Helper()
	frame := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := range 40 {
		for x := range 80 {
			frame.SetNRGBA(x, y, colors[(y/20)*2+x/40])
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, frame, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode oriented fixture: %v", err)
	}
	// A synthetic EXIF IFD containing only the orientation tag; no camera metadata.
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(exif[24:26], orientation)
	data := append([]byte{0xff, 0xd8, 0xff, 0xe1}, byte((len(exif)+2)>>8), byte(len(exif)+2))
	data = append(data, exif...)
	return append(data, encoded.Bytes()[2:]...)
}

func abs(value int) int {
	return max(value, -value)
}

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
