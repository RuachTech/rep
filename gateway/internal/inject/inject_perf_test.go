package inject

import (
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// largeHTMLBody returns an HTML body just over compressMinBytes so the
// middleware will compress it for clients that accept gzip.
func largeHTMLBody() string {
	var b strings.Builder
	b.WriteString("<html><head><title>t</title></head><body>")
	b.WriteString(strings.Repeat("Lorem ipsum dolor sit amet. ", 100))
	b.WriteString("</body></html>")
	return b.String()
}

func TestMiddleware_GzipEncodingWhenAccepted(t *testing.T) {
	html := largeHTMLBody()
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We expect the middleware to strip Accept-Encoding before the
		// upstream sees it; if a value sneaks through we want to know.
		if got := r.Header.Get("Accept-Encoding"); got != "" {
			t.Errorf("upstream should not see Accept-Encoding, got %q", got)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})

	m := New(upstream, testScriptTag, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected Content-Encoding=gzip, got %q", got)
	}

	if got := rec.Header().Get("Vary"); !containsToken(got, "Accept-Encoding") {
		t.Errorf("expected Vary to include Accept-Encoding, got %q", got)
	}

	// The body should be valid gzip and decompress to the original-with-tag.
	gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("response body is not valid gzip: %v", err)
	}
	defer func() { _ = gz.Close() }()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("decompressing body: %v", err)
	}
	if !strings.Contains(string(decompressed), `id="__rep__"`) {
		t.Error("decompressed body should contain the injected script tag")
	}

	// Compressed wire size should be smaller than the original — that is
	// the whole point.
	if len(rec.Body.Bytes()) >= len(html) {
		t.Errorf("compressed body (%d) should be smaller than original (%d)",
			len(rec.Body.Bytes()), len(html))
	}
}

func TestMiddleware_NoGzipWhenClientDoesNotAccept(t *testing.T) {
	html := largeHTMLBody()
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})

	m := New(upstream, testScriptTag, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Accept-Encoding header.
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("expected no Content-Encoding, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), `id="__rep__"`) {
		t.Error("identity body should still contain the injected tag")
	}
}

func TestMiddleware_NoGzipForSmallBodies(t *testing.T) {
	// Shorter than compressMinBytes — gzip overhead would exceed savings.
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body>Hi</body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("small response should not be gzipped, got encoding %q", got)
	}
}

func TestMiddleware_GzipQZeroRejection(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(largeHTMLBody()))
	})

	m := New(upstream, testScriptTag, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Client says "I'd accept anything except gzip."
	req.Header.Set("Accept-Encoding", "gzip;q=0, identity")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("gzip;q=0 should not be gzipped, got encoding %q", got)
	}
}

func TestMiddleware_CacheDisabledByDefault(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body>x</body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, req)
	}

	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("cache disabled by default — expected 3 upstream calls, got %d", got)
	}
}

func TestMiddleware_CacheHitSkipsUpstream(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body>cached</body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	// First request populates the cache.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec1 := httptest.NewRecorder()
	m.ServeHTTP(rec1, req1)
	body1 := rec1.Body.String()

	// Second request should hit the cache.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	m.ServeHTTP(rec2, req2)
	body2 := rec2.Body.String()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 upstream call (cache hit on second request), got %d", got)
	}
	if body1 != body2 {
		t.Errorf("cached response should be byte-identical: %q vs %q", body1, body2)
	}
	if !strings.Contains(body2, `id="__rep__"`) {
		t.Error("cached response should still contain the injected tag")
	}
}

func TestMiddleware_CacheHitRespectsAcceptEncoding(t *testing.T) {
	html := largeHTMLBody()
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	// Populate cache with a no-gzip request.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec1 := httptest.NewRecorder()
	m.ServeHTTP(rec1, req1)
	if got := rec1.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("first (no-AE) request should be identity, got %q", got)
	}

	// Same path, this time with gzip — should hit cache and serve gzipped variant.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Accept-Encoding", "gzip")
	rec2 := httptest.NewRecorder()
	m.ServeHTTP(rec2, req2)
	if got := rec2.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("cache hit should serve gzip when client asks for it, got %q", got)
	}
}

func TestMiddleware_UpdateScriptTagInvalidatesCache(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	// Populate.
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	// Update tag — cache should be invalidated.
	newTag := `<script id="__rep__" type="application/json">{"public":{"X":"1"}}</script>`
	m.UpdateScriptTag(newTag)

	rec = httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("UpdateScriptTag should invalidate the cache (expected 2 upstream calls, got %d)", got)
	}
	if !strings.Contains(rec.Body.String(), `"X":"1"`) {
		t.Error("response after UpdateScriptTag should reflect the new tag")
	}
}

func TestMiddleware_CacheSkipsSetCookieResponses(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		w.Header().Add("Set-Cookie", "session=abc; Path=/")
		_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("Set-Cookie responses must not be cached — expected 3 upstream calls, got %d", got)
	}
}

func TestMiddleware_CacheSkipsNon200(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<html><head></head><body>fail</body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("non-200 responses must not be cached — expected 2 upstream calls, got %d", got)
	}
}

func TestMiddleware_CacheKeyIncludesQueryString(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body>` + r.URL.RawQuery + `</body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	for _, q := range []string{"a=1", "a=2", "a=1"} {
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/page?"+q, nil))
	}

	// Two distinct queries → two upstream calls. The third repeats `a=1`
	// so it should hit the cache.
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("cache key should distinguish query strings: expected 2 upstream calls, got %d", got)
	}
}

func TestMiddleware_CacheSkipsCookieRequests(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Cookie", "session=abc")
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, req)
	}

	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("requests with Cookie must not be cached: expected 3 upstream calls, got %d", got)
	}
}

func TestMiddleware_CacheSkipsAuthorizationRequests(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer xyz")
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, req)
	}

	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("requests with Authorization must not be cached: expected 3 upstream calls, got %d", got)
	}
}

func TestMiddleware_CacheSkipsCacheControlPrivate(t *testing.T) {
	cases := []string{"private", "no-store", "no-cache", "private, max-age=60"}
	for _, cc := range cases {
		t.Run(cc, func(t *testing.T) {
			var calls int32
			upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.Header().Set("Content-Type", "text/html")
				w.Header().Set("Cache-Control", cc)
				_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
			})

			m := New(upstream, testScriptTag, slog.Default())
			m.EnableCache()

			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			}

			if got := atomic.LoadInt32(&calls); got != 2 {
				t.Errorf("Cache-Control %q should disable caching, got %d upstream calls", cc, got)
			}
		})
	}
}

func TestMiddleware_CacheSkipsVaryByCookie(t *testing.T) {
	var calls int32
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Vary", "Origin, Cookie")
		_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	m.EnableCache()

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("Vary by Cookie should disable caching: expected 2 upstream calls, got %d", got)
	}
}

func TestMiddleware_StripsETagAndLastModified(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("ETag", `"upstream-etag-abc"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2026 07:28:00 GMT")
		_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("ETag"); got != "" {
		t.Errorf("ETag should be stripped (the body has been modified), got %q", got)
	}
	if got := rec.Header().Get("Last-Modified"); got != "" {
		t.Errorf("Last-Modified should be stripped (the body has been modified), got %q", got)
	}
}

func TestMiddleware_BodylessStatusPassThrough(t *testing.T) {
	cases := []int{
		http.StatusContinue,            // 100
		http.StatusNoContent,           // 204
		http.StatusNotModified,         // 304
		http.StatusSwitchingProtocols,  // 101 (1xx range)
	}
	for _, status := range cases {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html") // tempting to inject!
				w.Header().Set("ETag", `"keep-me"`)
				w.WriteHeader(status)
				// No body — bodyless statuses must not write one.
			})

			m := New(upstream, testScriptTag, slog.Default())
			rec := httptest.NewRecorder()
			m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			if rec.Code != status {
				t.Errorf("status passed through wrong: got %d, want %d", rec.Code, status)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("bodyless status %d gained a body of %d bytes", status, rec.Body.Len())
			}
			// Validators on bodyless responses should pass through untouched
			// (they describe the upstream's representation, which we didn't
			// modify because we didn't write a body).
			if got := rec.Header().Get("ETag"); got != `"keep-me"` {
				t.Errorf("bodyless response should preserve upstream ETag, got %q", got)
			}
		})
	}
}

func TestMiddleware_VaryHeaderPresent(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); !containsToken(got, "Accept-Encoding") {
		t.Errorf("Vary header should include Accept-Encoding, got %q", got)
	}
}

func TestAcceptsGzip(t *testing.T) {
	cases := map[string]bool{
		"":                       false,
		"identity":               false,
		"gzip":                   true,
		"gzip, deflate":          true,
		"deflate, gzip;q=0.8":    true,
		"gzip;q=0":               false,
		"identity, gzip ; q = 0": false,
		"*":                      true,
		"*;q=0":                  false,
		"deflate, *;q=0.5":       true,
		// RFC 9110 §12.5.3: an explicit coding parameter takes precedence
		// over `*`. These two cases caught a regression early on.
		"gzip;q=0, *;q=0.5": false, // explicit gzip rejection wins
		"*;q=0, gzip":       true,  // explicit gzip allowance wins
	}
	for header, want := range cases {
		got := acceptsGzip(header)
		if got != want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestAddVary_DoesNotDuplicate(t *testing.T) {
	cases := []struct {
		name    string
		initial []string // existing Vary header values (multiple = repeated header)
		add     string
		want    []string
	}{
		{
			name:    "empty",
			initial: nil,
			add:     "Accept-Encoding",
			want:    []string{"Accept-Encoding"},
		},
		{
			name:    "single matching value",
			initial: []string{"Accept-Encoding"},
			add:     "Accept-Encoding",
			want:    []string{"Accept-Encoding"},
		},
		{
			name:    "comma-separated existing",
			initial: []string{"Origin, Accept-Encoding"},
			add:     "Accept-Encoding",
			want:    []string{"Origin, Accept-Encoding"},
		},
		{
			name:    "case-insensitive match",
			initial: []string{"origin, accept-encoding"},
			add:     "Accept-Encoding",
			want:    []string{"origin, accept-encoding"},
		},
		{
			name:    "different value adds",
			initial: []string{"Origin"},
			add:     "Accept-Encoding",
			want:    []string{"Origin", "Accept-Encoding"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for _, v := range tc.initial {
				h.Add("Vary", v)
			}
			addVary(h, tc.add)

			got := h.Values("Vary")
			if len(got) != len(tc.want) {
				t.Fatalf("Vary header count = %d, want %d (got %v)", len(got), len(tc.want), got)
			}
			for i, want := range tc.want {
				if got[i] != want {
					t.Errorf("Vary[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// containsToken reports whether a comma-separated header value contains the
// given token (case-insensitive). Used to assert Vary contents.
func containsToken(header, token string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
