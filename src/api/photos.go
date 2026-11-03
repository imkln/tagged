package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/imkln/tagged/src/tags"
)

var (
	errPhotoNotFound = errors.New("photo not found")
	errPhotoConflict = errors.New("photo is not ready")
)

type photoRecord struct {
	Attributions []attribution `dynamodbav:"attributions,omitempty"`
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

type photoConfig struct {
	Bucket         string
	MaxUploadBytes int64
	UploadTTL      time.Duration
	DownloadTTL    time.Duration
	StorageOrigin  string
}

type photoRepository interface {
	Create(context.Context, photoRecord) error
	Get(context.Context, string) (photoRecord, error)
	List(context.Context) ([]photoRecord, error)
}

type photoObjects interface {
	PresignUpload(context.Context, string, string, int64, time.Duration) (string, error)
	PresignDownload(context.Context, string, time.Duration) (string, error)
}

type photoLibrary struct {
	repository photoRepository
	objects    photoObjects
	config     photoConfig
}

func newPhotoLibrary(repository photoRepository, objects photoObjects, config photoConfig) *photoLibrary {
	return &photoLibrary{repository: repository, objects: objects, config: config}
}

type attribution struct {
	Name string `json:"name" dynamodbav:"name"`
	URL  string `json:"url" dynamodbav:"url"`
}

type photoResponse struct {
	Attributions []attribution `json:"attributions,omitempty"`
	Tags         []tags.Tag    `json:"tags,omitempty"`
	ID           string        `json:"id"`
	Filename     string        `json:"filename"`
	Status       string        `json:"status"`
	Error        string        `json:"error,omitempty"`
	ThumbnailURL string        `json:"thumbnailUrl,omitempty"`
	OriginalURL  string        `json:"originalUrl,omitempty"`
	UploadedAt   string        `json:"uploadedAt,omitempty"`
	CapturedAt   string        `json:"capturedAt,omitempty"`
	Width        int           `json:"width,omitempty"`
	Height       int           `json:"height,omitempty"`
	CameraMake   string        `json:"cameraMake,omitempty"`
	CameraModel  string        `json:"cameraModel,omitempty"`
	Latitude     *float64      `json:"latitude,omitempty"`
	Longitude    *float64      `json:"longitude,omitempty"`
}

func (app *server) photoRoute(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, status := app.identity(r); status != http.StatusOK {
			writeJSON(w, status, map[string]string{"error": http.StatusText(status)})
			return
		}

		if r.Method == http.MethodPost && r.Header.Get("Origin") != app.config.Origin {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Invalid request origin"})
			return
		}

		next(w, r)
	}
}

func (app *server) createUpload(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Send file metadata as JSON."})
		return
	}

	var request struct {
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
		Size        int64  `json:"size"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&request)
	if err == nil {
		err = decoder.Decode(new(any))
		if err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Send one file metadata object."})
			return
		}
	}

	if err != io.EOF || strings.TrimSpace(request.Filename) == "" || len(request.Filename) > 1024 || request.ContentType != "image/jpeg" || request.Size <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Select a non-empty JPEG with a filename."})
		return
	}

	if app.photos == nil {
		app.photoError(w, errors.New("photo storage is not configured"))
		return
	}

	if request.Size > app.photos.config.MaxUploadBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": fmt.Sprintf("Photo exceeds the %d MB upload limit.", app.photos.config.MaxUploadBytes/(1024*1024))})
		return
	}

	id := randomValue()
	photo := photoRecord{
		ID:           id,
		Filename:     request.Filename,
		ContentType:  request.ContentType,
		Size:         request.Size,
		UploadedAt:   time.Now().UTC().Format(time.RFC3339),
		Status:       "pending",
		OriginalKey:  "originals/" + id,
		ThumbnailKey: "thumbnails/" + id + ".jpg",
	}
	if err := app.photos.repository.Create(r.Context(), photo); err != nil {
		app.photoError(w, err)
		return
	}

	uploadURL, err := app.photos.objects.PresignUpload(r.Context(), photo.OriginalKey, photo.ContentType, photo.Size, app.photos.config.UploadTTL)
	if err != nil {
		app.photoError(w, err)
		return
	}

	w.Header().Set("Location", "/api/uploads/"+id)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": id, "photoId": id, "status": photo.Status, "uploadUrl": uploadURL,
		"expiresAt": time.Now().Add(app.photos.config.UploadTTL).UTC().Format(time.RFC3339),
	})
}

func (app *server) uploadStatus(w http.ResponseWriter, r *http.Request) {
	if app.photos == nil {
		app.photoError(w, errors.New("photo storage is not configured"))
		return
	}

	photo, err := app.photos.repository.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		app.photoError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id": photo.ID, "photoId": photo.ID, "filename": photo.Filename, "status": photo.Status, "error": photo.Error,
	})
}

func (app *server) photoList(w http.ResponseWriter, r *http.Request) {
	if app.photos == nil {
		app.photoError(w, errors.New("photo storage is not configured"))
		return
	}

	photos, err := app.photos.repository.List(r.Context())
	if err != nil {
		app.photoError(w, err)
		return
	}

	sort.Slice(photos, func(i, j int) bool {
		left, right := photoSortDate(photos[i]), photoSortDate(photos[j])
		if left.Equal(right) {
			return photos[i].ID < photos[j].ID
		}

		return left.After(right)
	})
	selected := r.URL.Query()["tag"]
	matches := photos[:0]
	for _, photo := range photos {
		if photo.Status != "pending" && matchesTags(photo, selected) {
			matches = append(matches, photo)
		}
	}

	total := len(matches)
	page, nextCursor, err := photoPage(matches, r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid photo page."})
		return
	}

	library := make([]photoResponse, 0, len(page))
	for _, photo := range page {
		if photo.Status == "pending" {
			continue
		}

		item := photoResponse{
			Attributions: photo.Attributions,
			Tags:         photo.Tags,
			ID:           photo.ID,
			Filename:     photo.Filename,
			Status:       photo.Status,
			Error:        photo.Error,
			UploadedAt:   validPhotoDate(photo.UploadedAt),
			CapturedAt:   validPhotoDate(photo.CapturedAt),
			Width:        photo.Width,
			Height:       photo.Height,
			CameraMake:   photo.CameraMake,
			CameraModel:  photo.CameraModel,
			Latitude:     photo.Latitude,
			Longitude:    photo.Longitude,
		}
		if photo.Status == "ready" {
			item.OriginalURL = "/api/photos/" + photo.ID + "/original"
			item.ThumbnailURL = "/api/photos/" + photo.ID + "/thumbnail"
		}

		library = append(library, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{"photos": library, "total": total, "nextCursor": nextCursor})
}

func (app *server) photoImage(variant string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.photos == nil {
			app.photoError(w, errors.New("photo storage is not configured"))
			return
		}

		photo, err := app.photos.repository.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			app.photoError(w, err)
			return
		}

		if photo.Status != "ready" {
			app.photoError(w, errPhotoConflict)
			return
		}

		key := photo.OriginalKey
		if variant == "thumbnail" {
			key = photo.ThumbnailKey
		}

		location, err := app.photos.objects.PresignDownload(r.Context(), key, app.photos.config.DownloadTTL)
		if err != nil {
			app.photoError(w, err)
			return
		}

		http.Redirect(w, r, location, http.StatusTemporaryRedirect)
	}
}

func (app *server) photoError(w http.ResponseWriter, err error) {
	status := http.StatusServiceUnavailable
	if errors.Is(err, errPhotoNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, errPhotoConflict) {
		status = http.StatusConflict
	}

	writeJSON(w, status, map[string]string{"error": http.StatusText(status)})
}

func validPhotoDate(value string) string {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999"} {
		if _, err := time.Parse(layout, value); err == nil {
			return value
		}
	}

	return ""
}

func photoSortDate(photo photoRecord) time.Time {
	for _, value := range []string{photo.CapturedAt, photo.UploadedAt} {
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999"} {
			if parsed, err := time.Parse(layout, value); err == nil {
				// Compare recorded clock times: a camera without an offset has no known UTC instant.
				return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), time.UTC)
			}
		}
	}

	return time.Time{}
}
