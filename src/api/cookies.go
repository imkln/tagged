package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const (
	sessionCookie = "tagged_session"
	flowCookie    = "tagged_login"
)

type cookieStore struct {
	cipher cipher.AEAD
	secure bool
}

type cookieEnvelope struct {
	Expires time.Time       `json:"expires"`
	Value   json.RawMessage `json:"value"`
}

func newCookieStore(key string, secure bool) (*cookieStore, error) {
	decoded, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(decoded)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &cookieStore{cipher: aead, secure: secure}, nil
}

func (store *cookieStore) set(w http.ResponseWriter, name string, value any, expires time.Time) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	plaintext, err := json.Marshal(cookieEnvelope{Expires: expires, Value: payload})
	if err != nil {
		return err
	}
	nonce := make([]byte, store.cipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := store.cipher.Seal(nonce, nonce, plaintext, []byte(name))
	encoded := base64.RawURLEncoding.EncodeToString(sealed)
	if len(encoded) > 3800 {
		return errors.New("session exceeds browser cookie size limit")
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    encoded,
		Path:     "/",
		Secure:   store.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   max(1, int(time.Until(expires).Seconds())),
	})
	return nil
}

func (store *cookieStore) read(r *http.Request, name string, value any) error {
	cookie, err := r.Cookie(name)
	if err != nil {
		return err
	}
	sealed, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || len(sealed) < store.cipher.NonceSize() {
		return errors.New("invalid cookie")
	}
	nonce, ciphertext := sealed[:store.cipher.NonceSize()], sealed[store.cipher.NonceSize():]
	plaintext, err := store.cipher.Open(nil, nonce, ciphertext, []byte(name))
	if err != nil {
		return err
	}
	var envelope cookieEnvelope
	if err := json.Unmarshal(plaintext, &envelope); err != nil {
		return err
	}
	if !time.Now().Before(envelope.Expires) {
		return errors.New("expired cookie")
	}
	return json.Unmarshal(envelope.Value, value)
}

func (store *cookieStore) clear(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Path:     "/",
		Secure:   store.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}
