package app

import (
	"context"
	"io/fs"
	"net/http"
)

type Config struct {
	Origin       string
	Issuer       string
	ClientID     string
	AuthDomain   string
	OwnerEmail   string
	OwnerSubject string
	CookieKey    string
}

func (c Config) Validate() error {
	return nil
}

func New(ctx context.Context, c Config, assets fs.FS) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	}), nil
}
