package crypto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"
)

func newTestHandler(t *testing.T, origins []string, maxRate int) *SessionKeyHandler {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	logger := slog.Default()
	return NewSessionKeyHandler(key, 30*time.Second, maxRate, origins, logger)
}

func TestSessionKey_Success(t *testing.T) {
	h := newTestHandler(t, nil, 100)

	req := httptest.NewRequest(http.MethodGet, "/rep/session-key", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SessionKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Key == "" {
		t.Error("key should not be empty")
	}
	if resp.ExpiresAt == "" {
		t.Error("expires_at should not be empty")
	}
	if resp.Nonce == "" {
		t.Error("nonce should not be empty")
	}

	// Verify expires_at is valid RFC3339.
	if _, err := time.Parse(time.RFC3339, resp.ExpiresAt); err != nil {
		t.Errorf("expires_at is not valid RFC3339: %v", err)
	}
}

func TestSessionKey_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(t, nil, 100)

	req := httptest.NewRequest(http.MethodPost, "/rep/session-key", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestSessionKey_OriginRejected(t *testing.T) {
	h := newTestHandler(t, []string{"https://allowed.com"}, 100)

	req := httptest.NewRequest(http.MethodGet, "/rep/session-key", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestSessionKey_OriginAllowed(t *testing.T) {
	h := newTestHandler(t, []string{"https://allowed.com"}, 100)

	req := httptest.NewRequest(http.MethodGet, "/rep/session-key", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSessionKey_RateLimit(t *testing.T) {
	maxRate := 3
	h := newTestHandler(t, nil, maxRate)

	for i := 0; i < maxRate; i++ {
		req := httptest.NewRequest(http.MethodGet, "/rep/session-key", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// Next request should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/rep/session-key", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestSessionKey_NoCacheHeaders(t *testing.T) {
	h := newTestHandler(t, nil, 100)

	req := httptest.NewRequest(http.MethodGet, "/rep/session-key", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-store, no-cache, must-revalidate" {
		t.Errorf("expected no-store cache-control, got %q", cc)
	}
}

func TestSessionKey_CORSHeaders(t *testing.T) {
	h := newTestHandler(t, []string{"https://app.com"}, 100)

	req := httptest.NewRequest(http.MethodGet, "/rep/session-key", nil)
	req.Header.Set("Origin", "https://app.com")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.com" {
		t.Errorf("expected CORS origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestSessionKey_CORSPreflight(t *testing.T) {
	h := newTestHandler(t, []string{"https://app.com"}, 100)

	req := httptest.NewRequest(http.MethodOptions, "/rep/session-key", nil)
	req.Header.Set("Origin", "https://app.com")
	rec := httptest.NewRecorder()

	h.CORSPreflight(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.com" {
		t.Error("expected CORS origin in preflight response")
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Allow-Methods in preflight response")
	}
}

func TestExtractIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")

	ip := extractIP(req)
	if ip != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", ip)
	}
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Del("X-Forwarded-For")
	req.RemoteAddr = "192.168.1.1:12345"

	ip := extractIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", ip)
	}
}
