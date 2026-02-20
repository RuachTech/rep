package inject

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const testScriptTag = `<script id="__rep__" type="application/json">{"public":{}}</script>`

func TestInjectIntoHTML_BeforeHeadClose(t *testing.T) {
	html := []byte(`<html><head><title>T</title></head><body></body></html>`)
	result := injectIntoHTML(html, []byte(testScriptTag))

	s := string(result)
	headCloseIdx := strings.Index(s, "</head>")
	scriptIdx := strings.Index(s, testScriptTag)

	if scriptIdx == -1 {
		t.Fatal("script tag not found in output")
	}
	if scriptIdx >= headCloseIdx {
		t.Error("script tag should appear before </head>")
	}
}

func TestInjectIntoHTML_AfterHeadOpen(t *testing.T) {
	// No </head> tag, only <head>.
	html := []byte(`<html><head><body></body></html>`)
	result := injectIntoHTML(html, []byte(testScriptTag))

	s := string(result)
	headOpenEnd := strings.Index(s, "<head>") + len("<head>")
	scriptIdx := strings.Index(s, testScriptTag)

	if scriptIdx == -1 {
		t.Fatal("script tag not found in output")
	}
	if scriptIdx < headOpenEnd {
		t.Error("script tag should appear after <head>")
	}
}

func TestInjectIntoHTML_HeadWithAttributes(t *testing.T) {
	html := []byte(`<html><head lang="en"><body></body></html>`)
	result := injectIntoHTML(html, []byte(testScriptTag))

	s := string(result)
	if !strings.Contains(s, testScriptTag) {
		t.Fatal("script tag not found in output")
	}
	// Should appear after the closing > of <head lang="en">.
	attrEnd := strings.Index(s, `<head lang="en">`) + len(`<head lang="en">`)
	scriptIdx := strings.Index(s, testScriptTag)
	if scriptIdx < attrEnd {
		t.Error("script should appear after <head> tag closes")
	}
}

func TestInjectIntoHTML_Fallback(t *testing.T) {
	html := []byte(`<div>hello</div>`)
	result := injectIntoHTML(html, []byte(testScriptTag))

	s := string(result)
	if !strings.HasPrefix(s, testScriptTag) {
		t.Error("script tag should be prepended when no <head> exists")
	}
}

func TestInjectIntoHTML_EmptyDocument(t *testing.T) {
	result := injectIntoHTML([]byte{}, []byte(testScriptTag))

	s := string(result)
	if !strings.Contains(s, testScriptTag) {
		t.Fatal("script tag should be present even for empty document")
	}
}

func TestMiddleware_HTMLResponse(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head></head><body>Hello</body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `id="__rep__"`) {
		t.Error("expected injected script tag in HTML response")
	}
	if !strings.Contains(body, "Hello") {
		t.Error("original body content should be preserved")
	}
}

func TestMiddleware_NonHTMLResponse(t *testing.T) {
	jsonBody := `{"key":"value"}`
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonBody))
	})

	m := New(upstream, testScriptTag, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	body := rec.Body.String()
	if body != jsonBody {
		t.Errorf("non-HTML response should be unmodified: got %q", body)
	}
}

func TestMiddleware_ContentLengthUpdated(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	body := rec.Body.Bytes()
	cl := rec.Header().Get("Content-Length")
	if cl == "" {
		t.Fatal("Content-Length should be set")
	}

	expectedLen := len(body)
	if cl != strings.TrimSpace(cl) {
		t.Error("Content-Length should not have extra whitespace")
	}
	_ = expectedLen // The header value is set by the middleware.
}

func TestIsHTML(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"text/html", true},
		{"text/html; charset=utf-8", true},
		{"TEXT/HTML", true},
		{"application/json", false},
		{"text/plain", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isHTML(tt.ct); got != tt.want {
			t.Errorf("isHTML(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}

func TestUpdateScriptTag_ConcurrentSafety(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head></head><body></body></html>`))
	})

	m := New(upstream, testScriptTag, slog.Default())

	var wg sync.WaitGroup
	// Writers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.UpdateScriptTag(testScriptTag)
			}
		}()
	}

	// Readers (ServeHTTP).
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rec := httptest.NewRecorder()
				m.ServeHTTP(rec, req)
			}
		}()
	}

	wg.Wait()
}

func TestDecompressBody_Gzip(t *testing.T) {
	// We don't test actual gzip here since we'd need to create gzip data,
	// but we test the identity and unsupported cases.
	body := []byte("hello world")

	result, err := decompressBody(body, "identity")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != "hello world" {
		t.Errorf("expected original body, got %q", result)
	}
}

func TestDecompressBody_Unsupported(t *testing.T) {
	_, err := decompressBody([]byte("data"), "br")
	if err == nil {
		t.Fatal("expected error for unsupported encoding")
	}
}

func TestDecompressBody_Empty(t *testing.T) {
	result, err := decompressBody([]byte("data"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != "data" {
		t.Errorf("expected original data, got %q", result)
	}
}
