package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type loginState struct {
	State    string
	Nonce    string
	Verifier string
}

type identityClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	TokenUse      string `json:"token_use"`
	Identities    []struct {
		Provider string `json:"providerName"`
	} `json:"identities"`
}

func (app *server) login(w http.ResponseWriter, r *http.Request) {
	flow := loginState{
		State:    randomValue(),
		Nonce:    randomValue(),
		Verifier: oauth2.GenerateVerifier(),
	}
	if err := app.cookies.set(w, flowCookie, flow, time.Now().Add(10*time.Minute)); err != nil {
		http.Error(w, "Unable to start sign-in", http.StatusInternalServerError)
		return
	}
	location := app.oauth.AuthCodeURL(flow.State,
		oauth2.S256ChallengeOption(flow.Verifier),
		oidc.Nonce(flow.Nonce),
		oauth2.SetAuthURLParam("identity_provider", "Google"),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func (app *server) callback(w http.ResponseWriter, r *http.Request) {
	var flow loginState
	err := app.cookies.read(r, flowCookie, &flow)
	app.cookies.clear(w, flowCookie)
	if err != nil || flow.State == "" || subtle.ConstantTimeCompare([]byte(flow.State), []byte(r.URL.Query().Get("state"))) != 1 {
		http.Error(w, "Sign-in expired or was invalid. Please try again.", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tokens, err := app.oauth.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(flow.Verifier))
	if err != nil {
		http.Error(w, "Unable to complete sign-in. Please try again.", http.StatusBadGateway)
		return
	}
	rawToken, ok := tokens.Extra("id_token").(string)
	if !ok {
		http.Error(w, "Sign-in did not return an identity", http.StatusBadGateway)
		return
	}
	token, _, err := app.verifyIdentity(ctx, rawToken)
	if err != nil || flow.Nonce == "" || subtle.ConstantTimeCompare([]byte(token.Nonce), []byte(flow.Nonce)) != 1 {
		app.cookies.clear(w, sessionCookie)
		app.pageWithStatus(w, r, http.StatusForbidden)
		return
	}
	if err := app.cookies.set(w, sessionCookie, rawToken, token.Expiry); err != nil {
		http.Error(w, "Unable to save your session", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/gallery", http.StatusSeeOther)
}

func (app *server) verifyIdentity(ctx context.Context, rawToken string) (*oidc.IDToken, identityClaims, error) {
	var claims identityClaims
	token, err := app.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, claims, err
	}
	if err := token.Claims(&claims); err != nil {
		return nil, claims, err
	}
	if token.Subject == "" || claims.TokenUse != "id" || !claims.EmailVerified || !strings.EqualFold(claims.Email, app.config.OwnerEmail) {
		return nil, claims, errors.New("identity is not authorized")
	}
	if app.config.OwnerSubject != "" && token.Subject != app.config.OwnerSubject {
		return nil, claims, errors.New("subject is not authorized")
	}
	for _, identity := range claims.Identities {
		if identity.Provider == "Google" {
			return token, claims, nil
		}
	}
	return nil, claims, errors.New("Google identity is required")
}

func (app *server) identity(r *http.Request) (identityClaims, int) {
	var rawToken string
	if err := app.cookies.read(r, sessionCookie, &rawToken); err != nil {
		return identityClaims{}, http.StatusUnauthorized
	}
	_, claims, err := app.verifyIdentity(r.Context(), rawToken)
	if err != nil {
		return identityClaims{}, http.StatusForbidden
	}
	return claims, http.StatusOK
}

func (app *server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != app.config.Origin {
		http.Error(w, "Invalid request origin", http.StatusForbidden)
		return
	}
	app.cookies.clear(w, sessionCookie)
	app.cookies.clear(w, flowCookie)
	query := url.Values{
		"client_id":  {app.config.ClientID},
		"logout_uri": {app.config.Origin + "/"},
	}
	http.Redirect(w, r, app.config.AuthDomain+"/logout?"+query.Encode(), http.StatusSeeOther)
}

func (app *server) pageWithStatus(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	app.page(w, r)
}

func randomValue() string {
	value := make([]byte, 32)
	rand.Read(value)
	return base64.RawURLEncoding.EncodeToString(value)
}
