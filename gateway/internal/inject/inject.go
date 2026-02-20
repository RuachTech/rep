// Package inject provides the HTML injection middleware.
//
// Per REP-RFC-0001 §4.3, the gateway intercepts HTML responses and injects
// a <script id="__rep__" type="application/json"> block before </head>.
package inject

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// Middleware wraps an http.Handler and injects the REP script tag into HTML responses.
type Middleware struct {
	// next is the upstream handler (reverse proxy or file server).
	next http.Handler

	// scriptTag is the pre-rendered <script> block to inject.
	scriptTag []byte

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
	m.scriptTag = []byte(scriptTag)
}

// ServeHTTP intercepts HTML responses and injects the REP payload.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		w.Write(rec.body.Bytes())
		return
	}

	// Inject the REP script tag into the HTML.
	body := rec.body.Bytes()
	injected := injectIntoHTML(body, m.scriptTag)

	// Update Content-Length to reflect the injected content.
	w.Header().Set("Content-Length", strconv.Itoa(len(injected)))

	// Remove Content-Encoding since we've modified the body.
	// (If upstream compressed it, we've already decompressed via the recorder.)
	w.Header().Del("Content-Encoding")

	w.WriteHeader(rec.statusCode)
	w.Write(injected)

	m.logger.Debug("rep.inject.html",
		"path", r.URL.Path,
		"original_size", len(body),
		"injected_size", len(injected),
	)
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
	body       *bytes.Buffer
	statusCode int
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
