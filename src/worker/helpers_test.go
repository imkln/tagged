package worker

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"sync"
	"testing"
)

type memoryPhotoRepository struct {
	mu      sync.Mutex
	records map[string]photoRecord
}

func (repository *memoryPhotoRepository) Get(_ context.Context, id string) (photoRecord, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()

	photo, exists := repository.records[id]
	if !exists {
		return photoRecord{}, errPhotoNotFound
	}

	return photo, nil
}

func (repository *memoryPhotoRepository) Update(_ context.Context, photo photoRecord, expectedStatus string) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()

	previous, exists := repository.records[photo.ID]
	if !exists {
		return errPhotoNotFound
	}

	if previous.Status != expectedStatus {
		return errPhotoConflict
	}

	repository.records[photo.ID] = photo
	return nil
}

type memoryPhotoObjects struct {
	mu          sync.Mutex
	data        map[string][]byte
	contentType map[string]string
	writes      []string
	putFailure  error
}

func (objects *memoryPhotoObjects) Get(_ context.Context, key string) ([]byte, error) {
	objects.mu.Lock()
	defer objects.mu.Unlock()

	data, exists := objects.data[key]
	if !exists {
		return nil, errors.New("object not found")
	}

	return bytes.Clone(data), nil
}

func (objects *memoryPhotoObjects) Put(_ context.Context, key, contentType string, data []byte) error {
	objects.mu.Lock()
	defer objects.mu.Unlock()

	if objects.putFailure != nil {
		return objects.putFailure
	}

	objects.data[key] = bytes.Clone(data)
	objects.contentType[key] = contentType
	objects.writes = append(objects.writes, key)
	return nil
}

type workerFixture struct {
	repository *memoryPhotoRepository
	objects    *memoryPhotoObjects
	worker     *thumbnailWorker
	config     workerConfig
}

func newWorkerFixture() *workerFixture {
	fixture := &workerFixture{
		repository: &memoryPhotoRepository{records: make(map[string]photoRecord)},
		objects: &memoryPhotoObjects{
			data:        make(map[string][]byte),
			contentType: make(map[string]string),
		},
		config: workerConfig{
			Bucket:         "fictional-photo-bucket",
			ThumbnailEdge:  320,
			MaxImagePixels: DefaultMaxImagePixels,
		},
	}
	fixture.worker = newThumbnailWorker(fixture.repository, fixture.objects, fixture.config, nil)
	return fixture
}

func (fixture *workerFixture) seed(status string) photoRecord {
	photo := photoRecord{
		ID:           "photo-garden",
		Filename:     "garden.jpg",
		ContentType:  "image/jpeg",
		Size:         100,
		Status:       status,
		OriginalKey:  "originals/photo-garden",
		ThumbnailKey: "thumbnails/photo-garden.jpg",
	}
	fixture.repository.records[photo.ID] = photo
	return photo
}

func fictionalJPEG(t *testing.T, width, height int) []byte {
	t.Helper()

	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, frame, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode fictional image: %v", err)
	}

	return encoded.Bytes()
}
