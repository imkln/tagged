package api

import (
	"encoding/hex"
	"errors"
	"net"
	"net/mail"
	"net/url"
	"strings"
)

type Config struct {
	Origin         string
	Issuer         string
	ClientID       string
	AuthDomain     string
	OwnerEmail     string
	OwnerSubject   string
	CookieKey      string
	PhotoBucket    string
	PhotoTable     string
	MaxUploadBytes int64
}

func (config Config) Validate() error {
	if (config.PhotoBucket == "") != (config.PhotoTable == "") {
		return errors.New("photo bucket and photo table must be configured together")
	}

	if config.MaxUploadBytes < 0 {
		return errors.New("maximum upload size must be positive")
	}

	address, err := mail.ParseAddress(config.OwnerEmail)
	if err != nil || address.Address != config.OwnerEmail {
		return errors.New("a valid owner email is required")
	}

	if strings.TrimSpace(config.ClientID) == "" {
		return errors.New("an OAuth client ID is required")
	}

	if err := validateURL(config.Origin, false); err != nil {
		return errors.New("origin must be an HTTPS origin or HTTP loopback origin")
	}

	if err := validateURL(config.Issuer, true); err != nil {
		return errors.New("a valid OIDC issuer URL is required")
	}

	if err := validateURL(config.AuthDomain, false); err != nil {
		return errors.New("a valid authentication domain URL is required")
	}

	key, err := hex.DecodeString(config.CookieKey)
	if err != nil || len(key) != 32 {
		return errors.New("cookie key must contain 32 hex-encoded bytes")
	}

	return nil
}

func validateURL(raw string, allowPath bool) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid URL")
	}

	if !allowPath && parsed.Path != "" && parsed.Path != "/" {
		return errors.New("origin cannot contain a path")
	}

	if parsed.Scheme == "https" {
		return nil
	}

	ip := net.ParseIP(parsed.Hostname())
	if parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || ip != nil && ip.IsLoopback()) {
		return nil
	}

	return errors.New("HTTPS required outside loopback")
}
