package api

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
)

func TestLibrarySortsByCaptureDateWithUploadFallback(t *testing.T) {
	photos := newPhotoFixture()
	for _, fields := range []map[string]any{
		{"id": "unknown-b"},
		{"id": "old", "capturedAt": "2020-01-01T12:00:00", "uploadedAt": "2026-10-01T12:00:00Z"},
		{"id": "fallback", "uploadedAt": "2026-08-01T12:00:00Z"},
		{"id": "unknown-a"},
		{"id": "tie-b", "capturedAt": "2026-09-01T12:00:00+02:00"},
		{"id": "tie-a", "capturedAt": "2026-09-01T12:00:00+02:00"},
		{"id": "invalid-capture", "capturedAt": "invalid", "uploadedAt": "2026-07-01T12:00:00Z"},
	} {
		seedMetadataPhoto(t, photos, fields)
	}

	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)

	response := serveRequest(fixture.handler, http.MethodGet, "/api/photos", cookie)

	requireStatus(t, response, http.StatusOK)

	var body struct {
		Photos []struct {
			ID string `json:"id"`
		} `json:"photos"`
	}

	decodeResponse(t, response, &body)

	var got []string
	for _, photo := range body.Photos {
		got = append(got, photo.ID)
	}

	want := []string{"tie-a", "tie-b", "fallback", "invalid-capture", "old", "unknown-a", "unknown-b"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestLibraryReturnsPrivateMetadataAndOmitsUnavailableFields(t *testing.T) {
	photos := newPhotoFixture()
	seedMetadataPhoto(t, photos, map[string]any{
		"id":          "complete",
		"capturedAt":  "2026-07-12T09:30:00",
		"uploadedAt":  "2026-08-01T12:00:00Z",
		"width":       80,
		"height":      40,
		"cameraMake":  "Example Camera",
		"cameraModel": "Model One",
	})
	seedMetadataPhoto(t, photos, map[string]any{"id": "missing"})

	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)

	response := serveRequest(fixture.handler, http.MethodGet, "/api/photos", cookie)

	requireStatus(t, response, http.StatusOK)

	var body struct {
		Photos []map[string]any `json:"photos"`
	}

	decodeResponse(t, response, &body)

	if len(body.Photos) != 2 {
		t.Fatalf("photos = %v, want two", body.Photos)
	}

	complete, missing := body.Photos[0], body.Photos[1]
	for key, want := range map[string]any{
		"capturedAt":  "2026-07-12T09:30:00",
		"uploadedAt":  "2026-08-01T12:00:00Z",
		"width":       float64(80),
		"height":      float64(40),
		"cameraMake":  "Example Camera",
		"cameraModel": "Model One",
	} {
		if complete[key] != want {
			t.Errorf("%s = %v, want %v", key, complete[key], want)
		}

		if _, exists := missing[key]; exists {
			t.Errorf("missing photo exposes %s = %v", key, missing[key])
		}
	}

	for _, path := range []string{"/api/photos"} {
		denied := serveRequest(fixture.handler, http.MethodGet, path)
		requireStatus(t, denied, http.StatusUnauthorized)
		if strings.Contains(denied.Body.String(), "Example Camera") {
			t.Error("metadata disclosed without session")
		}
	}

	fixture.claims["email"] = "visitor@example.test"
	denied := fixture.callback(fixture.beginLogin(t))
	requireStatus(t, denied, http.StatusForbidden)
}

func TestUploadRecordsServerDate(t *testing.T) {
	photos := newPhotoFixture()

	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)
	before := time.Now().UTC().Truncate(time.Second)

	response := photoRequest(fixture.handler, http.MethodPost, "/api/uploads", `{"filename":"garden.jpg","contentType":"image/jpeg","size":100}`, testOrigin, cookie)

	requireStatus(t, response, http.StatusCreated)
	if len(photos.repository.records) != 1 {
		t.Fatalf("records = %d, want one", len(photos.repository.records))
	}

	for _, photo := range photos.repository.records {
		item, err := attributevalue.MarshalMap(photo)
		if err != nil {
			t.Fatalf("marshal upload: %v", err)
		}

		var fields map[string]any
		if err := attributevalue.UnmarshalMap(item, &fields); err != nil {
			t.Fatalf("decode upload: %v", err)
		}

		value, _ := fields["uploadedAt"].(string)
		uploaded, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatalf("uploadedAt = %q: %v", value, err)
		}

		if uploaded.Before(before) || uploaded.After(time.Now().UTC()) {
			t.Errorf("upload date outside request interval: %s", value)
		}
	}
}

func seedMetadataPhoto(t *testing.T, fixture *photoFixture, fields map[string]any) {
	t.Helper()

	fields["filename"] = fields["id"].(string) + ".jpg"
	fields["status"] = "ready"
	fields["originalKey"] = "originals/" + fields["id"].(string)
	fields["thumbnailKey"] = "thumbnails/" + fields["id"].(string) + ".jpg"

	item, err := attributevalue.MarshalMap(fields)
	if err != nil {
		t.Fatalf("marshal metadata fixture: %v", err)
	}

	var photo photoRecord
	if err := attributevalue.UnmarshalMap(item, &photo); err != nil {
		t.Fatalf("decode metadata fixture: %v", err)
	}

	fixture.repository.records[photo.ID] = photo
}
