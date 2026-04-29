// Package inject provides the HTML injection middleware.
//
// Per REP-RFC-0001 §4.3, the gateway intercepts HTML responses and injects
// a <script id="__rep__" type="application/json"> block before </head>.
package inject

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const (
	// compressMinBytes is the smallest response body we'll bother gzipping.
	// Below this, gzip overhead can exceed savings.
	compressMinBytes = 1024

	// cacheMaxEntries bounds the in-memory cache so a runaway URL space
	// can't blow up memory. Static exports rarely exceed a few hundred
	// HTML routes, so this is generous.
	cacheMaxEntries = 1000
)

// Middleware wraps an http.Handler and injects the REP script tag into HTML responses.
type Middleware struct {
	// next is the upstream handler (reverse proxy or file server).
	next http.Handler

	// scriptTag is the pre-rendered <script> block to inject.
	scriptTag []byte

	// mu protects scriptTag from concurrent read/write during hot reload.
	mu sync.RWMutex

	logger *slog.Logger

	// cache stores fully-processed (injected, optionally gzipped) responses
	// keyed by request path. nil means caching is disabled — the default.
	// Per REP-RFC-0001 §4.3 the gateway MUST NOT cache when SENSITIVE vars
	// are present (the encrypted blob may rotate), so the server only opts
	// in via EnableCache when it's safe.
	cache   map[string]*cacheEntry
	cacheMu sync.RWMutex
}

// cacheEntry is a fully-processed response stored under a path key.
// Both encodings are pre-computed so cache hits never re-compress.
type cacheEntry struct {
	statusCode int
	headers    http.Header
	identity   []byte // pre-injected identity-encoded bytes
	gzipped    []byte // pre-compressed bytes (nil if compression wasn't worthwhile)
}

// New creates a new injection middleware. Caching is off by default;
// callers opt in via EnableCache when no SENSITIVE variables are present.
func New(next http.Handler, scriptTag string, logger *slog.Logger) *Middleware {
	return &Middleware{
		next:      next,
		scriptTag: []byte(scriptTag),
		logger:    logger,
	}
}

// EnableCache turns on response caching for processed HTML.
//
// Per REP-RFC-0001 §4.3, the gateway MUST NOT cache injected HTML when
// SENSITIVE variables are present (the encrypted blob may rotate). Callers
// must only enable this when no SENSITIVE vars are configured, and should
// also leave it off when hot-reload is active.
func (m *Middleware) EnableCache() {
	m.cacheMu.Lock()
	if m.cache == nil {
		m.cache = make(map[string]*cacheEntry)
	}
	m.cacheMu.Unlock()
}

// UpdateScriptTag replaces the script tag (used during hot reload) and
// invalidates any cached responses (they contain the previous tag).
func (m *Middleware) UpdateScriptTag(scriptTag string) {
	m.mu.Lock()
	m.scriptTag = []byte(scriptTag)
	m.mu.Unlock()

	m.cacheMu.Lock()
	if m.cache != nil {
		m.cache = make(map[string]*cacheEntry)
	}
	m.cacheMu.Unlock()
}

// ServeHTTP intercepts HTML responses and injects the REP payload.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// WebSocket upgrade requests must bypass the recorder entirely.
	// The recorder doesn't implement http.Hijacker, which the proxy needs
	// to upgrade the connection (e.g. Vite HMR, live-reload sockets).
	if isWebSocketUpgrade(r) {
		m.next.ServeHTTP(w, r)
		return
	}

	// Capture the client's preferred response encoding before we strip it.
	// We strip Accept-Encoding so the upstream always returns identity (we
	// need to byte-search for </head>); on the way back out we honour the
	// client's original preference and re-compress if appropriate.
	clientAccepts := r.Header.Get("Accept-Encoding")
	r.Header.Del("Accept-Encoding")

	// Cache lookup — only for GET, only when caching is enabled, only for
	// requests that don't carry per-user identity (Cookie/Authorization).
	// The cache is keyed by request URI (path + query) so URLs that vary
	// by query don't collide.
	cacheKey := r.URL.RequestURI()
	if r.Method == http.MethodGet && requestIsCacheable(r) {
		if entry := m.cacheGet(cacheKey); entry != nil {
			m.writeCached(w, entry, clientAccepts)
			m.logger.Debug("rep.inject.cache_hit", "path", cacheKey)
			return
		}
	}

	// Wrap the response writer to capture the response.
	rec := &responseRecorder{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		header:         make(http.Header),
		statusCode:     http.StatusOK,
	}

	// Serve the request to the upstream handler.
	m.next.ServeHTTP(rec, r)

	// Statuses that MUST NOT carry a body (RFC 9110 §15) — pass the
	// upstream response through unmodified. Injecting into a 304 / 204 /
	// 1xx would generate a non-empty body and a Content-Length, violating
	// HTTP semantics and breaking downstream conditional-request flows.
	if isBodylessStatus(rec.statusCode) {
		copyHeaders(w.Header(), rec.header)
		w.WriteHeader(rec.statusCode)
		if _, err := w.Write(rec.body.Bytes()); err != nil {
			m.logger.Debug("rep.inject.write_error", "path", r.URL.Path, "error", err)
		}
		return
	}

	// Check if the response is HTML.
	contentType := rec.header.Get("Content-Type")
	if !isHTML(contentType) {
		// Not HTML — write the response as-is.
		copyHeaders(w.Header(), rec.header)
		w.WriteHeader(rec.statusCode)
		if _, err := w.Write(rec.body.Bytes()); err != nil {
			m.logger.Debug("rep.inject.write_error", "path", r.URL.Path, "error", err)
		}
		return
	}

	// Decompress the body if the upstream ignored our Accept-Encoding removal.
	body := rec.body.Bytes()
	encoding := rec.header.Get("Content-Encoding")
	if encoding != "" {
		decompressed, err := decompressBody(body, encoding)
		if err != nil {
			// Cannot decompress — pass through unmodified.
			m.logger.Warn("rep.inject.skip",
				"path", r.URL.Path,
				"reason", "unsupported Content-Encoding: "+encoding,
			)
			copyHeaders(w.Header(), rec.header)
			w.WriteHeader(rec.statusCode)
			if _, err := w.Write(body); err != nil {
				m.logger.Debug("rep.inject.write_error", "path", r.URL.Path, "error", err)
			}
			return
		}
		body = decompressed
	}

	// Copy the script tag under a read lock to avoid a data race with UpdateScriptTag.
	m.mu.RLock()
	tag := make([]byte, len(m.scriptTag))
	copy(tag, m.scriptTag)
	m.mu.RUnlock()

	// Inject the REP script tag into the HTML.
	injected := injectIntoHTML(body, tag)

	// Compute the gzipped variant only when we'll actually use it:
	//   - this client accepts gzip (we'll send it now), OR
	//   - caching is enabled (we may send it to a future client that does).
	// Otherwise the work would be thrown away.
	var gzipped []byte
	if len(injected) >= compressMinBytes {
		needsGzip := acceptsGzip(clientAccepts) || m.cacheActive()
		if needsGzip {
			var err error
			gzipped, err = gzipCompress(injected)
			if err != nil {
				m.logger.Debug("rep.inject.gzip_error", "path", r.URL.Path, "error", err)
				gzipped = nil
			}
		}
	}

	// Build the response headers we'll send to the client. Strip
	// Content-Encoding/Length (we own them now). Strip ETag and
	// Last-Modified because the upstream computed them for the
	// pre-injection body — keeping them would mislead conditional-request
	// flows. Announce that the body varies on Accept-Encoding so caches
	// don't serve the wrong form.
	respHeader := make(http.Header)
	copyHeaders(respHeader, rec.header)
	respHeader.Del("Content-Encoding")
	respHeader.Del("Content-Length")
	respHeader.Del("ETag")
	respHeader.Del("Last-Modified")
	addVary(respHeader, "Accept-Encoding")

	// Cache eligibility — many guards because the cache is keyed by URI
	// only and content can be per-user in proxy mode:
	//
	//   - GET only
	//   - 200 OK only
	//   - request has no Cookie/Authorization (would otherwise be per-user)
	//   - response has no Set-Cookie (per-user state being established)
	//   - response is not marked Cache-Control: private/no-store/no-cache
	//   - response doesn't Vary by Cookie/Authorization
	if r.Method == http.MethodGet &&
		rec.statusCode == http.StatusOK &&
		requestIsCacheable(r) &&
		responseIsCacheable(rec.header) {
		m.cachePut(cacheKey, &cacheEntry{
			statusCode: rec.statusCode,
			headers:    respHeader.Clone(),
			identity:   injected,
			gzipped:    gzipped,
		})
	}

	// Pick the encoding the client wants and ship it.
	outBody, outEncoding := pickVariant(injected, gzipped, clientAccepts)
	if outEncoding != "" {
		respHeader.Set("Content-Encoding", outEncoding)
	}
	respHeader.Set("Content-Length", strconv.Itoa(len(outBody)))

	dst := w.Header()
	for k := range dst {
		dst.Del(k)
	}
	for k, values := range respHeader {
		for _, value := range values {
			dst.Add(k, value)
		}
	}
	w.WriteHeader(rec.statusCode)
	if _, err := w.Write(outBody); err != nil {
		m.logger.Debug("rep.inject.write_error", "path", r.URL.Path, "error", err)
	}

	m.logger.Debug("rep.inject.html",
		"path", r.URL.Path,
		"original_size", len(body),
		"injected_size", len(injected),
		"sent_size", len(outBody),
		"encoding", outEncoding,
	)
}

// writeCached emits a cached entry, picking identity or gzip per the
// client's Accept-Encoding.
func (m *Middleware) writeCached(w http.ResponseWriter, entry *cacheEntry, clientAccepts string) {
	body, encoding := pickVariant(entry.identity, entry.gzipped, clientAccepts)

	dst := w.Header()
	for k := range dst {
		dst.Del(k)
	}
	for k, values := range entry.headers {
		for _, value := range values {
			dst.Add(k, value)
		}
	}
	if encoding != "" {
		dst.Set("Content-Encoding", encoding)
	}
	dst.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(entry.statusCode)
	_, _ = w.Write(body)
}

// cacheActive reports whether caching is enabled.
func (m *Middleware) cacheActive() bool {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	return m.cache != nil
}

func (m *Middleware) cacheGet(path string) *cacheEntry {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	if m.cache == nil {
		return nil
	}
	return m.cache[path]
}

func (m *Middleware) cachePut(path string, entry *cacheEntry) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if m.cache == nil {
		return
	}
	if len(m.cache) >= cacheMaxEntries {
		// Bounded; drop new additions until the next UpdateScriptTag clears.
		// In practice this never trips for static exports.
		return
	}
	m.cache[path] = entry
}

// pickVariant chooses identity or gzipped based on the client's Accept-Encoding.
// Falls back to identity if we didn't pre-compute gzip for this response.
func pickVariant(identity, gzipped []byte, accept string) (body []byte, encoding string) {
	if len(gzipped) > 0 && acceptsGzip(accept) {
		return gzipped, "gzip"
	}
	return identity, ""
}

// acceptsGzip parses Accept-Encoding and returns true if gzip is acceptable.
//
// Per RFC 9110 §12.5.3, an explicit coding parameter takes precedence over
// the `*` wildcard. So `gzip;q=0, *;q=0.5` rejects gzip even though `*`
// would otherwise allow it.
func acceptsGzip(accept string) bool {
	if accept == "" {
		return false
	}

	var (
		explicitGzipSeen bool
		explicitGzipQ    float64
		wildcardSeen     bool
		wildcardQ        float64
	)

	for _, part := range strings.Split(accept, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		name, params, _ := strings.Cut(token, ";")
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "gzip" && name != "*" {
			continue
		}
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			p = strings.TrimSpace(p)
			if k, v, ok := strings.Cut(p, "="); ok && strings.EqualFold(strings.TrimSpace(k), "q") {
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					q = parsed
				}
			}
		}
		if name == "gzip" {
			explicitGzipSeen = true
			explicitGzipQ = q
		} else { // "*"
			wildcardSeen = true
			wildcardQ = q
		}
	}

	switch {
	case explicitGzipSeen:
		return explicitGzipQ > 0
	case wildcardSeen:
		return wildcardQ > 0
	default:
		return false
	}
}

// addVary appends a token to the Vary header if it isn't already present.
// Handles both repeated `Vary:` headers and single comma-separated values
// (`Vary: Origin, Accept-Encoding`), so we never duplicate a token.
func addVary(h http.Header, value string) {
	for _, v := range h.Values("Vary") {
		for _, existing := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), value) {
				return
			}
		}
	}
	h.Add("Vary", value)
}

// gzipCompress returns the gzip-encoded form of body.
func gzipCompress(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(body); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decompressBody decompresses a response body based on Content-Encoding.
// Returns an error for unsupported encodings (e.g., brotli — no stdlib support).
func decompressBody(body []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer func() { _ = reader.Close() }()
		return io.ReadAll(reader)
	case "identity", "":
		return body, nil
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", encoding)
	}
}

// injectIntoHTML inserts the script tag into the HTML document.
//
// Injection priority per §4.3:
//  1. Before </head>
//  2. After <head> (if no </head>)
//  3. Prepend to body (if neither exists)
func injectIntoHTML(html, scriptTag []byte) []byte {
	// Try inserting before </head>, skipping any occurrences inside HTML comments.
	headClose := findOutsideComments(html, []byte("</head>"))
	if headClose != -1 {
		result := make([]byte, 0, len(html)+len(scriptTag)+1)
		result = append(result, html[:headClose]...)
		result = append(result, '\n')
		result = append(result, scriptTag...)
		result = append(result, '\n')
		result = append(result, html[headClose:]...)
		return result
	}

	// Try inserting after <head>.
	headOpen := bytes.Index(html, []byte("<head"))
	if headOpen != -1 {
		// Find the end of the <head> tag (handle <head> and <head ...>).
		tagEnd := bytes.IndexByte(html[headOpen:], '>')
		if tagEnd != -1 {
			insertAt := headOpen + tagEnd + 1
			result := make([]byte, 0, len(html)+len(scriptTag)+1)
			result = append(result, html[:insertAt]...)
			result = append(result, '\n')
			result = append(result, scriptTag...)
			result = append(result, html[insertAt:]...)
			return result
		}
	}

	// Fallback: prepend to the entire body.
	result := make([]byte, 0, len(scriptTag)+1+len(html))
	result = append(result, scriptTag...)
	result = append(result, '\n')
	result = append(result, html...)
	return result
}

// findOutsideComments returns the index of the first occurrence of target
// in html that is NOT inside an HTML comment (<!-- ... -->). Returns -1 if
// no match is found outside a comment.
func findOutsideComments(html, target []byte) int {
	commentOpen := []byte("<!--")
	commentClose := []byte("-->")
	offset := 0

	for offset < len(html) {
		// Find the next occurrence of target.
		idx := bytes.Index(html[offset:], target)
		if idx == -1 {
			return -1
		}
		absIdx := offset + idx

		// Check if absIdx falls inside a comment by scanning from the start
		// of the remaining window for comment boundaries.
		if !isInsideComment(html, absIdx, commentOpen, commentClose) {
			return absIdx
		}

		// Skip past this occurrence and keep searching.
		offset = absIdx + len(target)
	}
	return -1
}

// isInsideComment reports whether position pos in html falls between a
// <!-- opener and its corresponding --> closer.
func isInsideComment(html []byte, pos int, open, close []byte) bool {
	// Walk through all comment regions before pos.
	i := 0
	for i < pos {
		start := bytes.Index(html[i:], open)
		if start == -1 || i+start >= pos {
			return false
		}
		start += i // absolute position of <!--

		end := bytes.Index(html[start+len(open):], close)
		if end == -1 {
			// Unclosed comment — everything after <!-- is inside.
			return pos >= start
		}
		end = start + len(open) + end + len(close) // absolute position after -->

		if pos >= start && pos < end {
			return true
		}
		i = end
	}
	return false
}

// isBodylessStatus reports whether an HTTP status code MUST NOT carry a
// response body, per RFC 9110 §15. The middleware bypasses injection,
// compression, and caching for these so we don't fabricate a body that
// breaks downstream conditional-request flows.
func isBodylessStatus(status int) bool {
	if status >= 100 && status < 200 {
		return true
	}
	switch status {
	case http.StatusNoContent, http.StatusNotModified:
		return true
	}
	return false
}

// requestIsCacheable reports whether a request can safely use the path-
// keyed in-memory cache. Skipped if the request carries identity headers
// that would normally personalise the response.
func requestIsCacheable(r *http.Request) bool {
	if r.Header.Get("Cookie") != "" {
		return false
	}
	if r.Header.Get("Authorization") != "" {
		return false
	}
	return true
}

// responseIsCacheable reports whether the upstream response can be
// stored in the in-memory cache. Honours upstream Cache-Control
// directives and rejects responses that vary by per-user headers.
func responseIsCacheable(h http.Header) bool {
	for _, v := range h.Values("Set-Cookie") {
		_ = v
		return false // any Set-Cookie disqualifies
	}
	for _, v := range h.Values("Cache-Control") {
		for _, directive := range strings.Split(v, ",") {
			d := strings.ToLower(strings.TrimSpace(directive))
			if d == "private" || d == "no-store" || d == "no-cache" {
				return false
			}
		}
	}
	for _, v := range h.Values("Vary") {
		for _, token := range strings.Split(v, ",") {
			t := strings.ToLower(strings.TrimSpace(token))
			if t == "cookie" || t == "authorization" || t == "*" {
				return false
			}
		}
	}
	return true
}

// isWebSocketUpgrade reports whether the request is a WebSocket upgrade.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// isHTML checks if a Content-Type header indicates an HTML response.
func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html")
}

// responseRecorder captures the upstream response for inspection.
type responseRecorder struct {
	http.ResponseWriter
	header      http.Header
	body        *bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.wroteHeader = true
	// Don't forward to the real writer yet — we need to inspect first.
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

// Flush implements http.Flusher for streaming support.
func (r *responseRecorder) Flush() {
	// Intentionally do nothing. The middleware buffers the full upstream response
	// before deciding whether to inject, so flushing here would prematurely commit
	// headers/body to the client.
}

// ReadFrom implements io.ReaderFrom for efficient copies.
func (r *responseRecorder) ReadFrom(src io.Reader) (int64, error) {
	return r.body.ReadFrom(src)
}

func copyHeaders(dst, src http.Header) {
	for k := range dst {
		dst.Del(k)
	}
	for k, values := range src {
		for _, value := range values {
			dst.Add(k, value)
		}
	}
}
