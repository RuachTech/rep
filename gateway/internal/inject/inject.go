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

// Middleware wraps an http.Handler and injects the REP script tag into HTML responses.
type Middleware struct {
	// next is the upstream handler (reverse proxy or file server).
	next http.Handler

	// scriptTag is the pre-rendered <script> block to inject.
	scriptTag []byte

	// mu protects scriptTag from concurrent read/write during hot reload.
	mu sync.RWMutex

	logger *slog.Logger
}

// New creates a new injection middleware.
func New(next http.Handler, scriptTag string, logger *slog.Logger) *Middleware {
	return &Middleware{
		next:      next,
		scriptTag: []byte(scriptTag),
		logger:    logger,
	}
}

// UpdateScriptTag replaces the script tag (used during hot reload).
func (m *Middleware) UpdateScriptTag(scriptTag string) {
	m.mu.Lock()
	m.scriptTag = []byte(scriptTag)
	m.mu.Unlock()
}

// ServeHTTP intercepts HTML responses and injects the REP payload.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip Accept-Encoding from the request so the upstream always responds
	// with identity encoding. This ensures we can reliably search for </head>
	// in the response body for injection.
	r.Header.Del("Accept-Encoding")

	// Wrap the response writer to capture the response.
	rec := &responseRecorder{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
	}

	// Serve the request to the upstream handler.
	m.next.ServeHTTP(rec, r)

	// Check if the response is HTML.
	contentType := rec.Header().Get("Content-Type")
	if !isHTML(contentType) {
		// Not HTML — write the response as-is.
		w.WriteHeader(rec.statusCode)
		if _, err := w.Write(rec.body.Bytes()); err != nil {
			m.logger.Debug("rep.inject.write_error", "path", r.URL.Path, "error", err)
		}
		return
	}

	// Decompress the body if the upstream ignored our Accept-Encoding removal.
	body := rec.body.Bytes()
	encoding := rec.Header().Get("Content-Encoding")
	if encoding != "" {
		decompressed, err := decompressBody(body, encoding)
		if err != nil {
			// Cannot decompress — pass through unmodified.
			m.logger.Warn("rep.inject.skip",
				"path", r.URL.Path,
				"reason", "unsupported Content-Encoding: "+encoding,
			)
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

	// Update Content-Length to reflect the injected content.
	w.Header().Set("Content-Length", strconv.Itoa(len(injected)))

	// Remove Content-Encoding since we've modified the body.
	w.Header().Del("Content-Encoding")

	w.WriteHeader(rec.statusCode)
	if _, err := w.Write(injected); err != nil {
		m.logger.Debug("rep.inject.write_error", "path", r.URL.Path, "error", err)
	}

	m.logger.Debug("rep.inject.html",
		"path", r.URL.Path,
		"original_size", len(body),
		"injected_size", len(injected),
	)
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
	// Try inserting before </head>.
	headClose := bytes.Index(html, []byte("</head>"))
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

// isHTML checks if a Content-Type header indicates an HTML response.
func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html")
}

// responseRecorder captures the upstream response for inspection.
type responseRecorder struct {
	http.ResponseWriter
	body        *bytes.Buffer
	statusCode  int
	wroteHeader bool
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
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ReadFrom implements io.ReaderFrom for efficient copies.
func (r *responseRecorder) ReadFrom(src io.Reader) (int64, error) {
	return r.body.ReadFrom(src)
}
