package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"golang.org/x/oauth2"
)

const testOrigin = "https://tagged.example.test"

type authFixture struct {
	handler         http.Handler
	issuer          *httptest.Server
	claims          map[string]any
	challenge       string
	signer          jose.Signer
	exchanges       atomic.Int32
	tamperSignature bool
}

type loginFlow struct {
	state      string
	cookies    []*http.Cookie
	parameters url.Values
}

func validConfig() Config {
	return Config{
		Origin:     testOrigin,
		Issuer:     "https://issuer.example.test",
		AuthDomain: "https://login.example.test",
		ClientID:   "test-client",
		OwnerEmail: "owner@example.test",
		CookieKey:  strings.Repeat("ab", 32),
	}
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithHeader("kid", "test"),
	)
	if err != nil {
		t.Fatalf("create token signer: %v", err)
	}

	fixture := &authFixture{signer: signer}
	provider := http.NewServeMux()
	provider.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(t, w, map[string]any{
			"issuer":                                fixture.issuer.URL,
			"authorization_endpoint":                fixture.issuer.URL + "/authorize",
			"token_endpoint":                        fixture.issuer.URL + "/token",
			"jwks_uri":                              fixture.issuer.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	provider.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(t, w, jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"},
			},
		})
	})
	provider.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		fixture.exchangeToken(t, w, r)
	})

	fixture.issuer = httptest.NewServer(provider)
	t.Cleanup(fixture.issuer.Close)
	fixture.claims = map[string]any{
		"iss":            fixture.issuer.URL,
		"aud":            "test-client",
		"sub":            "owner-id",
		"email":          "owner@example.test",
		"email_verified": true,
		"token_use":      "id",
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
		"identities":     []map[string]any{{"providerName": "Google"}},
	}

	config := validConfig()
	config.Issuer = fixture.issuer.URL
	config.AuthDomain = fixture.issuer.URL
	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<html>Tagged</html>")},
		"assets/app.js": {Data: []byte("export {}")},
	}
	fixture.handler, err = New(context.Background(), config, assets)
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	return fixture
}

func (fixture *authFixture) exchangeToken(t *testing.T, w http.ResponseWriter, r *http.Request) {
	fixture.exchanges.Add(1)
	if err := r.ParseForm(); err != nil {
		t.Errorf("parse token request: %v", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	verifier := r.Form.Get("code_verifier")
	if r.Form.Get("code") != "valid-code" || verifier == "" || oauth2.S256ChallengeFromVerifier(verifier) != fixture.challenge {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}

	claims, err := json.Marshal(fixture.claims)
	if err != nil {
		t.Errorf("marshal token claims: %v", err)
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}
	object, err := fixture.signer.Sign(claims)
	if err != nil {
		t.Errorf("sign token: %v", err)
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}
	token, err := object.CompactSerialize()
	if err != nil {
		t.Errorf("serialize token: %v", err)
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}
	if fixture.tamperSignature {
		parts := strings.Split(token, ".")
		if strings.HasPrefix(parts[2], "A") {
			parts[2] = "B" + parts[2][1:]
		} else {
			parts[2] = "A" + parts[2][1:]
		}
		token = strings.Join(parts, ".")
	}
	writeTestJSON(t, w, map[string]any{
		"id_token":     token,
		"access_token": "test-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}

func (fixture *authFixture) beginLogin(t *testing.T) loginFlow {
	t.Helper()

	response := serveRequest(fixture.handler, http.MethodGet, "/auth/login")
	requireStatus(t, response, http.StatusSeeOther)
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse login redirect: %v", err)
	}
	parameters := location.Query()
	cookies := response.Result().Cookies()
	if parameters.Get("state") == "" || len(cookies) == 0 {
		t.Fatal("login did not create an OAuth flow")
	}
	fixture.challenge = parameters.Get("code_challenge")
	fixture.claims["nonce"] = parameters.Get("nonce")

	return loginFlow{
		state:      parameters.Get("state"),
		cookies:    cookies,
		parameters: parameters,
	}
}

func (fixture *authFixture) callback(flow loginFlow) *httptest.ResponseRecorder {
	path := "/auth/callback?code=valid-code&state=" + url.QueryEscape(flow.state)
	return serveRequest(fixture.handler, http.MethodGet, path, flow.cookies...)
}

func (fixture *authFixture) signIn(t *testing.T) *http.Cookie {
	t.Helper()

	response := fixture.callback(fixture.beginLogin(t))
	requireStatus(t, response, http.StatusSeeOther)
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge > 0 {
			return cookie
		}
	}
	t.Fatal("sign-in did not create a session")
	return nil
}

func serveRequest(handler http.Handler, method, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, testOrigin+path, nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func requireStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, want, response.Body)
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("write provider response: %v", err)
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, value any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(value); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
