// Package health provides the /rep/health endpoint.
//
// Per REP-RFC-0001 §4.5, this endpoint returns gateway health status
// including variable counts per tier and guardrail status.
package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/ruach-tech/rep/gateway/internal/config"
	"github.com/ruach-tech/rep/gateway/internal/guardrails"
)

// Response is the JSON body returned by /rep/health.
type Response struct {
	Status        string          `json:"status"`
	Version       string          `json:"version"`
	Variables     VariableCounts  `json:"variables"`
	Guardrails    GuardrailStatus `json:"guardrails"`
	UptimeSeconds int64           `json:"uptime_seconds"`
}

// VariableCounts holds per-tier variable counts.
type VariableCounts struct {
	Public    int `json:"public"`
	Sensitive int `json:"sensitive"`
	Server    int `json:"server"`
}

// GuardrailStatus holds guardrail scan results.
type GuardrailStatus struct {
	Warnings int `json:"warnings"`
	Blocked  int `json:"blocked"`
}

// Handler serves the /rep/health endpoint.
type Handler struct {
	version         string
	vars            *config.ClassifiedVars
	guardrailResult *guardrails.Result
	startTime       time.Time
}

// NewHandler creates a new health check handler.
func NewHandler(version string, vars *config.ClassifiedVars, gr *guardrails.Result, startTime time.Time) *Handler {
	return &Handler{
		version:         version,
		vars:            vars,
		guardrailResult: gr,
		startTime:       startTime,
	}
}

// ServeHTTP handles GET /rep/health requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	blocked := 0
	warnings := 0
	if h.guardrailResult != nil {
		warnings = len(h.guardrailResult.Warnings)
	}

	resp := Response{
		Status:  "healthy",
		Version: h.version,
		Variables: VariableCounts{
			Public:    len(h.vars.Public),
			Sensitive: len(h.vars.Sensitive),
			Server:    len(h.vars.Server),
		},
		Guardrails: GuardrailStatus{
			Warnings: warnings,
			Blocked:  blocked,
		},
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Default().Error("rep.health.encode_error", "error", err)
	}
}
