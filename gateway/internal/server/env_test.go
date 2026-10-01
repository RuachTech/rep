package server

import (
	"os"
	"strings"
	"testing"
)

// clearREPEnv removes every REP_* variable from the process environment for
// the duration of the test, so server.New sees only what the test sets.
func clearREPEnv(t *testing.T) {
	t.Helper()
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(key, "REP_") {
			t.Setenv(key, "") // registers restoration on cleanup
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("unsetenv %q: %v", key, err)
			}
		}
	}
}
