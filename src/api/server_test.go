package api

import (
	"net/http"
	"testing"
)

func TestSignedOutCannotAccessLibrary(t *testing.T) {
	fixture := newAuthFixture(t)

	for _, path := range []string{"/api/session", "/api/library"} {
		t.Run(path, func(t *testing.T) {
			response := serveRequest(fixture.handler, http.MethodGet, path)

			requireStatus(t, response, http.StatusUnauthorized)
		})
	}

	t.Run("gallery redirects to sign-in", func(t *testing.T) {
		response := serveRequest(fixture.handler, http.MethodGet, "/gallery")

		requireStatus(t, response, http.StatusSeeOther)
		if location := response.Header().Get("Location"); location != "/" {
			t.Errorf("redirect = %q, want /", location)
		}
	})
}

func TestSignInPageIsPublic(t *testing.T) {
	fixture := newAuthFixture(t)

	response := serveRequest(fixture.handler, http.MethodGet, "/")

	requireStatus(t, response, http.StatusOK)
}

func TestOwnerCanViewGallery(t *testing.T) {
	fixture := newAuthFixture(t)
	cookie := fixture.signIn(t)

	response := serveRequest(fixture.handler, http.MethodGet, "/gallery", cookie)

	requireStatus(t, response, http.StatusOK)
}

func TestSessionReturnsOwnerIdentityWithoutCaching(t *testing.T) {
	fixture := newAuthFixture(t)
	cookie := fixture.signIn(t)

	response := serveRequest(fixture.handler, http.MethodGet, "/api/session", cookie)

	requireStatus(t, response, http.StatusOK)
	var session struct {
		Email string `json:"email"`
	}
	decodeResponse(t, response, &session)
	if session.Email != "owner@example.test" {
		t.Errorf("email = %q, want owner@example.test", session.Email)
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cacheControl)
	}
}

func TestOwnerStartsWithEmptyLibrary(t *testing.T) {
	fixture := newAuthFixture(t)
	cookie := fixture.signIn(t)

	response := serveRequest(fixture.handler, http.MethodGet, "/api/library", cookie)

	requireStatus(t, response, http.StatusOK)
	var library struct {
		Photos []any `json:"photos"`
		Total  int   `json:"total"`
	}
	decodeResponse(t, response, &library)
	if library.Photos == nil || len(library.Photos) != 0 || library.Total != 0 {
		t.Errorf("expected an empty library, got %+v", library)
	}
}

func TestTamperedSessionCannotReadLibrary(t *testing.T) {
	fixture := newAuthFixture(t)
	cookie := fixture.signIn(t)
	cookie.Value += "tampered"

	response := serveRequest(fixture.handler, http.MethodGet, "/api/library", cookie)

	requireStatus(t, response, http.StatusUnauthorized)
}

func TestUnknownFilesAndRoutesAreNotExposed(t *testing.T) {
	fixture := newAuthFixture(t)

	for _, path := range []string{"/.env", "/go.mod", "/api/unknown", "/assets/missing.js"} {
		t.Run(path, func(t *testing.T) {
			response := serveRequest(fixture.handler, http.MethodGet, path)

			requireStatus(t, response, http.StatusNotFound)
		})
	}
}
