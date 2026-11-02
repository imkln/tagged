package api

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var photoRoutes = []struct {
	method string
	path   string
}{
	{method: http.MethodPost, path: "/api/uploads"},
	{method: http.MethodGet, path: "/api/uploads/photo-test"},
	{method: http.MethodGet, path: "/api/photos"},
	{method: http.MethodGet, path: "/api/photos/photo-test/original"},
	{method: http.MethodGet, path: "/api/photos/photo-test/thumbnail"},
}

func TestPrivatePhotoRoutesRequireOwner(t *testing.T) {
	fixture := newAuthFixture(t)
	cookie := fixture.signIn(t)
	tampered := *cookie
	tampered.Value += "tampered"
	visitor := visitorSession(t, fixture)

	tests := []struct {
		name   string
		cookie *http.Cookie
		status int
	}{
		{name: "signed out", cookie: nil, status: http.StatusUnauthorized},
		{name: "tampered session", cookie: &tampered, status: http.StatusUnauthorized},
		{name: "different account", cookie: visitor, status: http.StatusForbidden},
	}

	for _, route := range photoRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					response := photoRequest(fixture.handler, route.method, route.path, `{}`, testOrigin, test.cookie)

					requireStatus(t, response, test.status)
					if location := response.Header().Get("Location"); location != "" {
						t.Errorf("unauthorized request returned a location: %q", location)
					}
					if cache := response.Header().Get("Cache-Control"); cache != "no-store" {
						t.Errorf("Cache-Control = %q, want no-store", cache)
					}
				})
			}
		})
	}
}

func TestUploadRejectsCrossOriginRequests(t *testing.T) {
	fixture := newAuthFixture(t)
	cookie := fixture.signIn(t)

	for _, origin := range []string{"", "null", "https://other.example.test"} {
		t.Run("origin="+origin, func(t *testing.T) {
			response := photoRequest(fixture.handler, http.MethodPost, "/api/uploads", `{}`, origin, cookie)

			requireStatus(t, response, http.StatusForbidden)
		})
	}
}

func TestUploadAuthorizationRejectsInvalidFiles(t *testing.T) {
	fixture := newAuthFixture(t)
	cookie := fixture.signIn(t)

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{`},
		{name: "missing fields", body: `{}`},
		{name: "unsupported type", body: `{"filename":"drawing.png","contentType":"image/png","size":100}`},
		{name: "empty filename", body: `{"filename":" ","contentType":"image/jpeg","size":100}`},
		{name: "empty file", body: `{"filename":"garden.jpg","contentType":"image/jpeg","size":0}`},
		{name: "negative size", body: `{"filename":"garden.jpg","contentType":"image/jpeg","size":-1}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := photoRequest(fixture.handler, http.MethodPost, "/api/uploads", test.body, testOrigin, cookie)

			requireStatus(t, response, http.StatusBadRequest)

			var result struct {
				Error string `json:"error"`
			}

			decodeResponse(t, response, &result)

			if result.Error == "" {
				t.Error("invalid upload did not return a useful error")
			}
		})
	}
}

func TestOwnerCanCreateUpload(t *testing.T) {
	photos := newPhotoFixture()
	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)

	response := photoRequest(fixture.handler, http.MethodPost, "/api/uploads",
		`{"filename":"garden.jpg","contentType":"image/jpeg","size":100}`, testOrigin, cookie)

	requireStatus(t, response, http.StatusCreated)

	var upload struct {
		ID        string `json:"id"`
		PhotoID   string `json:"photoId"`
		Status    string `json:"status"`
		UploadURL string `json:"uploadUrl"`
	}

	decodeResponse(t, response, &upload)

	if upload.ID == "" || upload.PhotoID == "" || upload.Status != "pending" {
		t.Errorf("upload = %+v, want identifiers and pending status", upload)
	}

	if location := response.Header().Get("Location"); location != "/api/uploads/"+upload.ID {
		t.Errorf("Location = %q, want created upload resource", location)
	}

	location, err := url.Parse(upload.UploadURL)
	if err != nil || location.Scheme != "https" || location.Host != "storage.example.test" {
		t.Errorf("upload URL = %q, want authorized storage URL", upload.UploadURL)
	}

	if len(photos.objects.uploads) != 1 {
		t.Fatalf("upload authorizations = %d, want 1", len(photos.objects.uploads))
	}

	signature := photos.objects.uploads[0]
	if signature.key == "" || signature.contentType != "image/jpeg" || signature.size != 100 || signature.ttl != photos.config.UploadTTL {
		t.Errorf("authorization = %+v, want one object, selected file metadata, and configured expiry", signature)
	}
}

func TestSameFilenameDoesNotOverwritePreviousUpload(t *testing.T) {
	photos := newPhotoFixture()
	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)

	for range 2 {
		response := photoRequest(fixture.handler, http.MethodPost, "/api/uploads",
			`{"filename":"garden.jpg","contentType":"image/jpeg","size":100}`, testOrigin, cookie)

		requireStatus(t, response, http.StatusCreated)
	}

	if len(photos.repository.records) != 2 || len(photos.objects.uploads) != 2 {
		t.Fatalf("records = %d, authorizations = %d; want two of each", len(photos.repository.records), len(photos.objects.uploads))
	}

	if photos.objects.uploads[0].key == photos.objects.uploads[1].key {
		t.Error("two uploads reused the same storage key")
	}
}

func TestOwnerCanReadUploadStatus(t *testing.T) {
	for _, status := range []string{"pending", "processing", "ready", "failed"} {
		t.Run(status, func(t *testing.T) {
			photos := newPhotoFixture()
			photo := photos.seed(status)
			fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
			cookie := fixture.signIn(t)

			response := photoRequest(fixture.handler, http.MethodGet, "/api/uploads/"+photo.ID, "", testOrigin, cookie)

			requireStatus(t, response, http.StatusOK)

			var upload struct {
				Status string `json:"status"`
			}

			decodeResponse(t, response, &upload)

			if upload.Status != status {
				t.Errorf("upload status = %q, want %q", upload.Status, status)
			}
		})
	}
}

func TestOwnerCanListPhotos(t *testing.T) {
	tests := []struct {
		name     string
		hasPhoto bool
	}{
		{name: "empty library", hasPhoto: false},
		{name: "saved photo", hasPhoto: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			photos := newPhotoFixture()
			if test.hasPhoto {
				photos.seed("ready")
			}
			fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
			cookie := fixture.signIn(t)

			response := photoRequest(fixture.handler, http.MethodGet, "/api/photos", "", testOrigin, cookie)

			requireStatus(t, response, http.StatusOK)

			var library struct {
				Photos []struct {
					ID           string `json:"id"`
					Filename     string `json:"filename"`
					OriginalURL  string `json:"originalUrl"`
					ThumbnailURL string `json:"thumbnailUrl"`
				} `json:"photos"`
				Total int `json:"total"`
			}

			decodeResponse(t, response, &library)

			wantTotal := 0
			if test.hasPhoto {
				wantTotal = 1
			}

			if library.Photos == nil || len(library.Photos) != wantTotal || library.Total != wantTotal {
				t.Fatalf("library = %+v, want %d photos", library, wantTotal)
			}

			if test.hasPhoto {
				saved := library.Photos[0]
				if saved.ID != "photo-garden" || saved.Filename != "garden.jpg" {
					t.Errorf("saved photo = %+v, want garden.jpg", saved)
				}

				if saved.OriginalURL != "/api/photos/photo-garden/original" || saved.ThumbnailURL != "/api/photos/photo-garden/thumbnail" {
					t.Errorf("photo URLs = %+v, want owner-authorized app resources", saved)
				}
			}
		})
	}
}

func TestOwnerCanViewPhotoVariants(t *testing.T) {
	for _, variant := range []string{"original", "thumbnail"} {
		t.Run(variant, func(t *testing.T) {
			photos := newPhotoFixture()
			photo := photos.seed("ready")
			fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
			cookie := fixture.signIn(t)

			response := photoRequest(fixture.handler, http.MethodGet, "/api/photos/"+photo.ID+"/"+variant, "", testOrigin, cookie)

			requireStatus(t, response, http.StatusTemporaryRedirect)
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Error("private image redirect is cacheable")
			}

			if len(photos.objects.downloads) != 1 {
				t.Fatalf("download authorizations = %d, want 1", len(photos.objects.downloads))
			}

			wantKey := photo.OriginalKey
			if variant == "thumbnail" {
				wantKey = photo.ThumbnailKey
			}

			signature := photos.objects.downloads[0]
			if signature.key != wantKey || signature.ttl != photos.config.DownloadTTL {
				t.Errorf("download authorization = %+v, want requested image and configured expiry", signature)
			}

			location, err := url.Parse(response.Header().Get("Location"))
			if err != nil || location.Scheme != "https" || location.Host != "storage.example.test" {
				t.Errorf("Location = %q, want authorized storage URL", response.Header().Get("Location"))
			}
		})
	}
}

func TestUnvalidatedPhotoCannotBeViewed(t *testing.T) {
	for _, status := range []string{"pending", "processing", "failed"} {
		t.Run(status, func(t *testing.T) {
			photos := newPhotoFixture()
			photo := photos.seed(status)
			fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
			cookie := fixture.signIn(t)

			for _, variant := range []string{"original", "thumbnail"} {
				response := photoRequest(fixture.handler, http.MethodGet, "/api/photos/"+photo.ID+"/"+variant, "", testOrigin, cookie)

				requireStatus(t, response, http.StatusConflict)
				if response.Header().Get("Location") != "" {
					t.Error("unvalidated photo returned a download location")
				}
			}
		})
	}
}

func TestUnknownPhotoResourcesReturnNotFound(t *testing.T) {
	photos := newPhotoFixture()
	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)

	for _, path := range []string{"/api/uploads/missing", "/api/photos/missing/original", "/api/photos/missing/thumbnail"} {
		t.Run(path, func(t *testing.T) {
			response := photoRequest(fixture.handler, http.MethodGet, path, "", testOrigin, cookie)

			requireStatus(t, response, http.StatusNotFound)
		})
	}
}

func TestUploadEnforcesConfiguredSizeLimit(t *testing.T) {
	tests := []struct {
		name   string
		delta  int64
		status int
	}{
		{name: "at limit", delta: 0, status: http.StatusCreated},
		{name: "over limit", delta: 1, status: http.StatusRequestEntityTooLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			photos := newPhotoFixture()
			fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
			cookie := fixture.signIn(t)
			body, err := json.Marshal(map[string]any{
				"filename": "garden.jpg", "contentType": "image/jpeg", "size": photos.config.MaxUploadBytes + test.delta,
			})
			if err != nil {
				t.Fatalf("marshal upload metadata: %v", err)
			}

			response := photoRequest(fixture.handler, http.MethodPost, "/api/uploads", string(body), testOrigin, cookie)

			requireStatus(t, response, test.status)
		})
	}
}

func TestStorageFailureDoesNotIssueUploadAuthorization(t *testing.T) {
	photos := newPhotoFixture()
	photos.repository.failure = errors.New("fictional database unavailable")
	fixture := newAuthFixture(t, WithPhotoLibrary(photos.library))
	cookie := fixture.signIn(t)

	response := photoRequest(fixture.handler, http.MethodPost, "/api/uploads",
		`{"filename":"garden.jpg","contentType":"image/jpeg","size":100}`, testOrigin, cookie)

	requireStatus(t, response, http.StatusServiceUnavailable)
	if strings.Contains(response.Body.String(), "uploadUrl") || strings.Contains(response.Body.String(), photos.repository.failure.Error()) {
		t.Error("failed upload exposed authorization or an internal database error")
	}
}

func photoRequest(handler http.Handler, method, path, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, testOrigin+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	if origin != "" {
		request.Header.Set("Origin", origin)
	}

	if cookie != nil {
		request.AddCookie(cookie)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	return response
}

func visitorSession(t *testing.T, fixture *authFixture) *http.Cookie {
	t.Helper()

	claims := make(map[string]any, len(fixture.claims))
	maps.Copy(claims, fixture.claims)

	claims["email"] = "visitor@example.test"
	claims["sub"] = "visitor-id"

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal visitor identity: %v", err)
	}

	signed, err := fixture.signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign visitor identity: %v", err)
	}

	token, err := signed.CompactSerialize()
	if err != nil {
		t.Fatalf("serialize visitor identity: %v", err)
	}

	store, err := newCookieStore(validConfig().CookieKey, true, 60*60)
	if err != nil {
		t.Fatalf("create test cookie store: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, testOrigin, nil)
	session, err := store.New(request, sessionCookie)
	if err != nil {
		t.Fatalf("create visitor session: %v", err)
	}

	session.Values = map[any]any{"token": token}
	response := httptest.NewRecorder()
	if err := session.Save(request, response); err != nil {
		t.Fatalf("save visitor session: %v", err)
	}

	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookie {
			return cookie
		}
	}

	t.Fatal("visitor session did not produce a cookie")
	return nil
}
