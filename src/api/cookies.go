package api

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gorilla/sessions"
)

const (
	sessionCookie = "tagged_session"
	flowCookie    = "tagged_login"
)

func newCookieStore(key string, secure bool, maxAge int) (*sessions.CookieStore, error) {
	decoded, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}

	keys, err := hkdf.Key(sha256.New, decoded, nil, "tagged-cookie-keys", 64)
	if err != nil {
		return nil, err
	}

	store := sessions.NewCookieStore(keys[:32], keys[32:])
	store.Options = &sessions.Options{
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	store.MaxAge(maxAge)

	return store, nil
}

func (app *server) clearCookie(w http.ResponseWriter, name string) {
	options := *app.sessionCookies.Options
	options.MaxAge = -1

	http.SetCookie(w, sessions.NewCookie(name, "", &options))
}
