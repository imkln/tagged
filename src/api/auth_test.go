package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestLoginUsesGoogleWithPKCEAndNonce(t *testing.T) {
	fixture := newAuthFixture(t)
	flow := fixture.beginLogin(t)

	if got := flow.parameters.Get("identity_provider"); got != "Google" {
		t.Errorf("identity_provider = %q, want Google", got)
	}
	if got := flow.parameters.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
	if flow.parameters.Get("code_challenge") == "" {
		t.Error("login is missing a PKCE challenge")
	}
	if flow.parameters.Get("nonce") == "" {
		t.Error("login is missing a nonce")
	}
}

func TestOwnerReceivesSecureSession(t *testing.T) {
	fixture := newAuthFixture(t)

	cookie := fixture.signIn(t)

	if !cookie.Secure {
		t.Error("session cookie is missing Secure")
	}
	if !cookie.HttpOnly {
		t.Error("session cookie is missing HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Path != "/" || cookie.Domain != "" {
		t.Error("session cookie must be host-only with path /")
	}
}

func TestCallbackRejectsInvalidIdentity(t *testing.T) {
	tests := []struct {
		name  string
		claim string
		value any
	}{
		{name: "another account", claim: "email", value: "other@example.test"},
		{name: "unverified email", claim: "email_verified", value: false},
		{name: "expired token", claim: "exp", value: time.Now().Add(-time.Hour).Unix()},
		{name: "wrong audience", claim: "aud", value: "another-client"},
		{name: "wrong issuer", claim: "iss", value: "https://attacker.example.test"},
		{name: "wrong nonce", claim: "nonce", value: "another-nonce"},
		{name: "access token", claim: "token_use", value: "access"},
		{name: "missing Google identity", claim: "identities", value: nil},
		{name: "missing subject", claim: "sub", value: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newAuthFixture(t)
			flow := fixture.beginLogin(t)
			fixture.claims[test.claim] = test.value

			response := fixture.callback(flow)

			requireStatus(t, response, http.StatusForbidden)
			requireNoSession(t, response)
		})
	}
}

func TestCallbackRejectsForgedSignature(t *testing.T) {
	fixture := newAuthFixture(t)
	flow := fixture.beginLogin(t)
	fixture.tamperSignature = true

	response := fixture.callback(flow)

	requireStatus(t, response, http.StatusForbidden)
	requireNoSession(t, response)
}

func TestCallbackRejectsInvalidStateBeforeExchangingCode(t *testing.T) {
	tests := []struct {
		name   string
		change func(*loginFlow)
	}{
		{
			name: "missing flow cookie",
			change: func(flow *loginFlow) {
				flow.cookies = nil
			},
		},
		{
			name: "tampered flow cookie",
			change: func(flow *loginFlow) {
				cookie := *flow.cookies[0]
				cookie.Value += "tampered"
				flow.cookies = []*http.Cookie{&cookie}
			},
		},
		{
			name: "mismatched state",
			change: func(flow *loginFlow) {
				flow.state = "another-state"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newAuthFixture(t)
			flow := fixture.beginLogin(t)
			test.change(&flow)

			response := fixture.callback(flow)

			requireStatus(t, response, http.StatusBadRequest)
			if count := fixture.exchanges.Load(); count != 0 {
				t.Errorf("exchanged %d codes despite invalid state", count)
			}
		})
	}
}

func TestLogoutClearsSession(t *testing.T) {
	fixture := newAuthFixture(t)
	cookie := fixture.signIn(t)
	request := httptest.NewRequest(http.MethodPost, testOrigin+"/auth/logout", nil)
	request.Header.Set("Origin", testOrigin)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()

	fixture.handler.ServeHTTP(response, request)

	requireStatus(t, response, http.StatusSeeOther)
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse logout redirect: %v", err)
	}
	if location.String() == "" || location.Scheme+"://"+location.Host != fixture.issuer.URL || location.Path != "/logout" {
		t.Errorf("expected provider logout redirect, got %q", location)
	}
	if got := location.Query().Get("logout_uri"); got != testOrigin+"/" {
		t.Errorf("logout_uri = %q, want %s/", got, testOrigin)
	}

	for _, clearedCookie := range response.Result().Cookies() {
		if clearedCookie.Name == cookie.Name && clearedCookie.MaxAge < 0 {
			return
		}
	}
	t.Error("logout did not clear the session cookie")
}

func TestLogoutRejectsCrossSiteRequests(t *testing.T) {
	for _, origin := range []string{"", "https://attacker.example.test"} {
		t.Run("origin="+origin, func(t *testing.T) {
			fixture := newAuthFixture(t)
			cookie := fixture.signIn(t)
			request := httptest.NewRequest(http.MethodPost, testOrigin+"/auth/logout", nil)
			request.Header.Set("Origin", origin)
			request.AddCookie(cookie)
			response := httptest.NewRecorder()

			fixture.handler.ServeHTTP(response, request)

			requireStatus(t, response, http.StatusForbidden)
		})
	}
}

func requireNoSession(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge > 0 {
			t.Error("rejected login received a session cookie")
		}
	}
}
