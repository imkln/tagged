package api

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestOwnerCanFindExistingTagsByName(t *testing.T) {
	photos := newPhotoFixture()
	seedTaggedPhotos(t, photos)
	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)

	for _, test := range []struct {
		query string
		want  []string
	}{
		{query: "", want: []string{"Cat", "Exampleland", "Garden"}},
		{query: "  gAR  ", want: []string{"Garden"}},
		{query: "missing", want: []string{}},
	} {
		t.Run(test.query, func(t *testing.T) {
			response := serveRequest(fixture.handler, http.MethodGet, "/api/tags?q="+url.QueryEscape(test.query), cookie)

			requireStatus(t, response, http.StatusOK)

			var body struct {
				Tags []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"tags"`
			}

			decodeResponse(t, response, &body)
			got := make([]string, 0, len(body.Tags))
			for _, tag := range body.Tags {
				if tag.ID == "" {
					t.Error("tag suggestion has no identity")
				}

				got = append(got, tag.Name)
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("suggestions = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPhotoSearchMatchesEverySelectedTag(t *testing.T) {
	photos := newPhotoFixture()
	seedTaggedPhotos(t, photos)
	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)

	for _, test := range []struct {
		query string
		want  []string
	}{
		{query: "", want: []string{"garden-cat", "garden", "cat", "untagged"}},
		{query: "?tag=tag-country", want: []string{"garden-cat", "garden"}},
		{query: "?tag=tag-country&tag=tag-city", want: []string{"garden-cat", "garden"}},
		{query: "?tag=tag-city&tag=tag-cat", want: []string{"garden-cat"}},
		{query: "?tag=tag-city&tag=tag-city", want: []string{"garden-cat", "garden"}},
		{query: "?tag=unknown", want: []string{}},
	} {
		t.Run(test.query, func(t *testing.T) {
			response := serveRequest(fixture.handler, http.MethodGet, "/api/photos"+test.query, cookie)

			requireStatus(t, response, http.StatusOK)

			var body struct {
				Photos []struct {
					ID string `json:"id"`
				} `json:"photos"`
				Total int `json:"total"`
			}

			decodeResponse(t, response, &body)
			got := make([]string, 0, len(body.Photos))
			for _, photo := range body.Photos {
				got = append(got, photo.ID)
			}

			if !reflect.DeepEqual(got, test.want) || body.Total != len(test.want) {
				t.Errorf("photos = %v, total = %d; want %v", got, body.Total, test.want)
			}
		})
	}
}

func TestTagSearchPaginatesMatchingPhotos(t *testing.T) {
	photos := newPhotoFixture()
	seedTaggedPhotos(t, photos)
	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)
	path := "/api/photos?tag=tag-city&limit=1"
	ids := make([]string, 0, 2)

	for page := range 2 {
		response := serveRequest(fixture.handler, http.MethodGet, path, cookie)

		requireStatus(t, response, http.StatusOK)

		var body struct {
			Photos []struct {
				ID string `json:"id"`
			} `json:"photos"`
			Total      int    `json:"total"`
			NextCursor string `json:"nextCursor"`
		}

		decodeResponse(t, response, &body)
		if len(body.Photos) != 1 || body.Total != 2 {
			t.Fatalf("page %d = %+v, want one match and total two", page, body)
		}

		ids = append(ids, body.Photos[0].ID)
		if page == 0 && body.NextCursor == "" {
			t.Fatal("first page has no next cursor")
		}

		if page == 1 && body.NextCursor != "" {
			t.Error("last page has a next cursor")
		}

		path = "/api/photos?tag=tag-city&limit=1&cursor=" + url.QueryEscape(body.NextCursor)
	}

	if !reflect.DeepEqual(ids, []string{"garden-cat", "garden"}) {
		t.Errorf("paginated matches = %v, want newest-first matching photos without duplicates", ids)
	}
}

func TestTagCatalogAndSearchRequireOwner(t *testing.T) {
	photos := newPhotoFixture()
	seedTaggedPhotos(t, photos)
	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	visitor := visitorSession(t, fixture)

	for _, path := range []string{"/api/tags?q=Garden", "/api/photos?tag=tag-city"} {
		signedOut := serveRequest(fixture.handler, http.MethodGet, path)

		requireStatus(t, signedOut, http.StatusUnauthorized)

		denied := serveRequest(fixture.handler, http.MethodGet, path, visitor)

		requireStatus(t, denied, http.StatusForbidden)
	}
}

func seedTaggedPhotos(t *testing.T, photos *photoFixture) {
	t.Helper()

	country := map[string]any{"id": "tag-country", "name": "Exampleland", "sources": []string{"metadata"}}
	city := map[string]any{"id": "tag-city", "name": "Garden", "sources": []string{"metadata"}}
	cat := map[string]any{"id": "tag-cat", "name": "Cat", "sources": []string{"manual"}}

	for _, photo := range []map[string]any{
		{"id": "garden-cat", "capturedAt": "2026-07-03T12:00:00", "tags": []any{country, city, cat}},
		{"id": "garden", "capturedAt": "2026-07-02T12:00:00", "tags": []any{country, city}},
		{"id": "cat", "capturedAt": "2026-07-01T12:00:00", "tags": []any{cat}},
		{"id": "untagged"},
	} {
		seedMetadataPhoto(t, photos, photo)
	}
}
