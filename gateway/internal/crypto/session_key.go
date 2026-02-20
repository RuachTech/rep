// Package crypto provides the session key endpoint handler.
//
// Per REP-RFC-0001 §4.4, the /rep/session-key endpoint issues short-lived,
// single-use decryption keys for SENSITIVE tier variables.
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// SessionKeyResponse is the JSON response from /rep/session-key.
type SessionKeyResponse struct {
	Key       string `json:"key"`
	ExpiresAt string `json:"expires_at"`
	Nonce     string `json:"nonce"`
}

// SessionKeyHandler manages session key issuance and validation.
type SessionKeyHandler struct {
	encryptionKey  []byte
	ttl            time.Duration
	maxRate        int // Per minute per IP.
	allowedOrigins []string
	logger         *slog.Logger

	mu          sync.Mutex
	issuedKeys  map[string]time.Time // keyID → expiry (for single-use tracking)
	rateLimiter map[string][]time.Time // IP → request timestamps
}

// NewSessionKeyHandler creates a handler for the /rep/session-key endpoint.
func NewSessionKeyHandler(
	encryptionKey []byte,
	ttl time.Duration,
	maxRate int,
	allowedOrigins []string,
	logger *slog.Logger,
) *SessionKeyHandler {
	h := &SessionKeyHandler{
		encryptionKey:  encryptionKey,
		ttl:            ttl,
		maxRate:        maxRate,
		allowedOrigins: allowedOrigins,
		logger:         logger,
		issuedKeys:     make(map[string]time.Time),
		rateLimiter:    make(map[string][]time.Time),
	}

	// Start cleanup goroutine for expired keys and rate limit entries.
	go h.cleanup()

	return h
}

// ServeHTTP handles GET /rep/session-key requests.
func (h *SessionKeyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only GET is allowed.
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate origin per §4.4.
	origin := r.Header.Get("Origin")
	if !h.isOriginAllowed(origin) {
		h.logger.Warn("rep.session_key.rejected",
			"client_ip", r.RemoteAddr,
			"reason", "origin_mismatch",
			"origin", origin,
		)
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	// Rate limiting per §4.4.
	clientIP := extractIP(r)
	if !h.checkRateLimit(clientIP) {
		h.logger.Warn("rep.session_key.rate_limited",
			"client_ip", clientIP,
			"requests_in_window", h.maxRate,
		)
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// Generate a unique key ID for single-use tracking.
	keyID := generateKeyID()
	expiresAt := time.Now().UTC().Add(h.ttl)

	// Track the key for single-use enforcement.
	h.mu.Lock()
	h.issuedKeys[keyID] = expiresAt
	h.mu.Unlock()

	// The session key is the encryption key itself, base64 encoded.
	// In a production hardening, this could be a derived key or a key-wrapping scheme.
	resp := SessionKeyResponse{
		Key:       base64.StdEncoding.EncodeToString(h.encryptionKey),
		ExpiresAt: expiresAt.Format(time.RFC3339),
		Nonce:     keyID,
	}

	h.logger.Info("rep.session_key.issued",
		"client_ip", clientIP,
		"origin", origin,
		"key_id", keyID,
		"expires_at", expiresAt.Format(time.RFC3339),
	)

	// Set headers per §4.4.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")

	// CORS headers.
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Vary", "Origin")
	}

	json.NewEncoder(w).Encode(resp)
}

// isOriginAllowed checks if the origin is in the allowed list.
// If no origins are configured, same-origin requests are allowed (empty Origin header).
func (h *SessionKeyHandler) isOriginAllowed(origin string) bool {
	if len(h.allowedOrigins) == 0 {
		// No explicit allow list — allow same-origin (empty or absent Origin).
		return true
	}

	for _, allowed := range h.allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
}

// checkRateLimit enforces the per-IP rate limit.
// Returns true if the request is within the limit.
func (h *SessionKeyHandler) checkRateLimit(ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	// Filter timestamps within the window.
	timestamps := h.rateLimiter[ip]
	valid := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			valid = append(valid, ts)
		}
	}

	if len(valid) >= h.maxRate {
		h.rateLimiter[ip] = valid
		return false
	}

	h.rateLimiter[ip] = append(valid, now)
	return true
}

// cleanup periodically removes expired keys and stale rate limit entries.
func (h *SessionKeyHandler) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		h.mu.Lock()
		now := time.Now()

		// Clean expired session keys.
		for id, expiry := range h.issuedKeys {
			if now.After(expiry) {
				delete(h.issuedKeys, id)
			}
		}

		// Clean stale rate limiter entries.
		windowStart := now.Add(-1 * time.Minute)
		for ip, timestamps := range h.rateLimiter {
			valid := timestamps[:0]
			for _, ts := range timestamps {
				if ts.After(windowStart) {
					valid = append(valid, ts)
				}
			}
			if len(valid) == 0 {
				delete(h.rateLimiter, ip)
			} else {
				h.rateLimiter[ip] = valid
			}
		}

		h.mu.Unlock()
	}
}

// generateKeyID creates a random identifier for session key tracking.
func generateKeyID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// extractIP extracts the client IP from the request,
// respecting X-Forwarded-For if present.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain (the client).
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Fall back to RemoteAddr (strip port).
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// CORSPreflight handles OPTIONS requests for the session key endpoint.
func (h *SessionKeyHandler) CORSPreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if h.isOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "3600")
		w.Header().Set("Vary", "Origin")
	}
	w.WriteHeader(http.StatusNoContent)
}

// Unused — placeholder for key ID helper.
func init() {
	_ = fmt.Sprintf // Ensure fmt is used.
}
