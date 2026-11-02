package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

type server struct {
	config         Config
	oauth          oauth2.Config
	verifier       *oidc.IDTokenVerifier
	loginCookies   *sessions.CookieStore
	sessionCookies *sessions.CookieStore
	assets         http.Handler
	index          []byte
	photos         *photoLibrary
}

type Option func(*server)

func WithPhotoLibrary(library *photoLibrary) Option {
	return func(app *server) { app.photos = library }
}

func New(ctx context.Context, config Config, assets fs.FS, options ...Option) (http.Handler, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	config.Origin = strings.TrimRight(config.Origin, "/")
	config.AuthDomain = strings.TrimRight(config.AuthDomain, "/")

	ctx = oidc.ClientContext(ctx, &http.Client{Timeout: 10 * time.Second})
	provider, err := oidc.NewProvider(ctx, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover identity provider: %w", err)
	}

	secure := strings.HasPrefix(config.Origin, "https://")
	loginCookies, err := newCookieStore(config.CookieKey, secure, 10*60)
	if err != nil {
		return nil, err
	}

	sessionCookies, err := newCookieStore(config.CookieKey, secure, 60*60)
	if err != nil {
		return nil, err
	}

	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read frontend build: %w", err)
	}

	endpoint := provider.Endpoint()
	endpoint.AuthStyle = oauth2.AuthStyleInParams

	app := &server{
		config: config,
		oauth: oauth2.Config{
			ClientID:    config.ClientID,
			Endpoint:    endpoint,
			RedirectURL: config.Origin + "/auth/callback",
			Scopes:      []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{
			ClientID:             config.ClientID,
			SupportedSigningAlgs: []string{oidc.RS256},
		}),
		loginCookies:   loginCookies,
		sessionCookies: sessionCookies,
		assets:         http.FileServerFS(assets),
		index:          index,
	}

	for _, option := range options {
		option(app)
	}

	if app.photos == nil && config.PhotoBucket != "" {
		app.photos, err = awsPhotoLibrary(ctx, config)
		if err != nil {
			return nil, err
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", app.page)
	mux.HandleFunc("GET /auth/login", app.login)
	mux.HandleFunc("GET /auth/callback", app.callback)
	mux.HandleFunc("POST /auth/logout", app.logout)
	mux.HandleFunc("GET /gallery", app.gallery)
	mux.HandleFunc("GET /api/session", app.session)
	mux.HandleFunc("GET /api/library", app.library)
	mux.HandleFunc("POST /api/uploads", app.photoRoute(app.createUpload))
	mux.HandleFunc("GET /api/uploads/{id}", app.photoRoute(app.uploadStatus))
	mux.HandleFunc("GET /api/photos", app.photoRoute(app.photoList))
	mux.HandleFunc("GET /api/photos/{id}/original", app.photoRoute(app.photoImage("original")))
	mux.HandleFunc("GET /api/photos/{id}/thumbnail", app.photoRoute(app.photoImage("thumbnail")))
	mux.Handle("GET /assets/", app.assets)
	storageOrigin := ""
	if app.photos != nil {
		storageOrigin = app.photos.config.StorageOrigin
	}

	return securityHeaders(mux, config.AuthDomain, storageOrigin), nil
}

func (app *server) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(app.index)
}

func (app *server) gallery(w http.ResponseWriter, r *http.Request) {
	if _, status := app.identity(r); status != http.StatusOK {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	app.page(w, r)
}

func (app *server) session(w http.ResponseWriter, r *http.Request) {
	identity, status := app.identity(r)
	if status != http.StatusOK {
		writeJSON(w, status, map[string]string{"error": http.StatusText(status)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"email": identity.Email})
}

func (app *server) library(w http.ResponseWriter, r *http.Request) {
	if _, status := app.identity(r); status != http.StatusOK {
		writeJSON(w, status, map[string]string{"error": http.StatusText(status)})
		return
	}

	if app.photos != nil {
		app.photoList(w, r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"photos": []any{}, "total": 0})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func securityHeaders(next http.Handler, authDomain, storageOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: "+storageOrigin+"; connect-src 'self' "+storageOrigin+"; frame-ancestors 'none'; base-uri 'none'; form-action 'self' "+authDomain)

		next.ServeHTTP(w, r)
	})
}
