package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ruach-tech/rep/gateway/internal/config"
	"github.com/ruach-tech/rep/gateway/internal/guardrails"
)

func TestHealth_Success(t *testing.T) {
	vars := &config.ClassifiedVars{
		Public:    []config.Variable{{Name: "A"}},
		Sensitive: []config.Variable{{Name: "B"}},
		Server:    []config.Variable{{Name: "C"}},
	}
	gr := &guardrails.Result{}

	h := NewHandler("0.1.0", vars, gr, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/rep/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status=healthy, got %s", resp.Status)
	}
	if resp.Version != "0.1.0" {
		t.Errorf("expected version=0.1.0, got %s", resp.Version)
	}
	if resp.Variables.Public != 1 {
		t.Errorf("expected 1 public, got %d", resp.Variables.Public)
	}
	if resp.Variables.Sensitive != 1 {
		t.Errorf("expected 1 sensitive, got %d", resp.Variables.Sensitive)
	}
	if resp.Variables.Server != 1 {
		t.Errorf("expected 1 server, got %d", resp.Variables.Server)
	}
}

func TestHealth_MethodNotAllowed(t *testing.T) {
	h := NewHandler("0.1.0", &config.ClassifiedVars{}, &guardrails.Result{}, time.Now())

	req := httptest.NewRequest(http.MethodPost, "/rep/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHealth_VariableCounts(t *testing.T) {
	vars := &config.ClassifiedVars{
		Public:    make([]config.Variable, 3),
		Sensitive: make([]config.Variable, 2),
		Server:    make([]config.Variable, 1),
	}
	h := NewHandler("0.1.0", vars, &guardrails.Result{}, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/rep/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Variables.Public != 3 || resp.Variables.Sensitive != 2 || resp.Variables.Server != 1 {
		t.Errorf("counts mismatch: %+v", resp.Variables)
	}
}

func TestHealth_Uptime(t *testing.T) {
	startTime := time.Now().Add(-5 * time.Second)
	h := NewHandler("0.1.0", &config.ClassifiedVars{}, &guardrails.Result{}, startTime)

	req := httptest.NewRequest(http.MethodGet, "/rep/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.UptimeSeconds < 5 {
		t.Errorf("expected uptime >= 5s, got %d", resp.UptimeSeconds)
	}
}

func TestHealth_GuardrailWarnings(t *testing.T) {
	gr := &guardrails.Result{
		Warnings: []guardrails.Warning{
			{VariableName: "A", DetectionType: "high_entropy"},
			{VariableName: "B", DetectionType: "known_format"},
		},
	}
	h := NewHandler("0.1.0", &config.ClassifiedVars{}, gr, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/rep/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Guardrails.Warnings != 2 {
		t.Errorf("expected 2 warnings, got %d", resp.Guardrails.Warnings)
	}
}

func TestHealth_NilGuardrails(t *testing.T) {
	h := NewHandler("0.1.0", &config.ClassifiedVars{}, nil, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/rep/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Guardrails.Warnings != 0 {
		t.Errorf("expected 0 warnings with nil guardrails, got %d", resp.Guardrails.Warnings)
	}
}

func TestHealth_ContentType(t *testing.T) {
	h := NewHandler("0.1.0", &config.ClassifiedVars{}, &guardrails.Result{}, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/rep/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
}
