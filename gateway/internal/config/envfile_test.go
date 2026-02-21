package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempEnvFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp env file: %v", err)
	}
	return path
}

func TestParseEnvFile_Basic(t *testing.T) {
	path := writeTempEnvFile(t, "FOO=bar\nBAZ=qux\n")

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%s", vars["FOO"])
	}
	if vars["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got BAZ=%s", vars["BAZ"])
	}
}

func TestParseEnvFile_Comments(t *testing.T) {
	path := writeTempEnvFile(t, "# This is a comment\nFOO=bar\n# Another comment\n")

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vars) != 1 {
		t.Errorf("expected 1 var, got %d", len(vars))
	}
	if vars["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%s", vars["FOO"])
	}
}

func TestParseEnvFile_EmptyLines(t *testing.T) {
	path := writeTempEnvFile(t, "\n\nFOO=bar\n\nBAZ=qux\n\n")

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vars) != 2 {
		t.Errorf("expected 2 vars, got %d", len(vars))
	}
}

func TestParseEnvFile_QuotedValues(t *testing.T) {
	path := writeTempEnvFile(t, `FOO="hello world"
BAZ='single quoted'
PLAIN=no quotes
`)

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars["FOO"] != "hello world" {
		t.Errorf("expected FOO=hello world, got FOO=%s", vars["FOO"])
	}
	if vars["BAZ"] != "single quoted" {
		t.Errorf("expected BAZ=single quoted, got BAZ=%s", vars["BAZ"])
	}
	if vars["PLAIN"] != "no quotes" {
		t.Errorf("expected PLAIN=no quotes, got PLAIN=%s", vars["PLAIN"])
	}
}

func TestParseEnvFile_ValueWithEquals(t *testing.T) {
	path := writeTempEnvFile(t, "URL=https://example.com?foo=bar&baz=qux\n")

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars["URL"] != "https://example.com?foo=bar&baz=qux" {
		t.Errorf("expected URL with embedded =, got %s", vars["URL"])
	}
}

func TestParseEnvFile_EmptyValue(t *testing.T) {
	path := writeTempEnvFile(t, "EMPTY=\n")

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := vars["EMPTY"]; !ok || v != "" {
		t.Errorf("expected EMPTY='', got %q (ok=%v)", v, ok)
	}
}

func TestParseEnvFile_NotFound(t *testing.T) {
	_, err := ParseEnvFile("/nonexistent/.env")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseEnvFile_Whitespace(t *testing.T) {
	path := writeTempEnvFile(t, "  FOO  =  bar  \n")

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars["FOO"] != "bar" {
		t.Errorf("expected FOO=bar after trimming, got FOO=%q", vars["FOO"])
	}
}

func TestReadAndClassify_WithEnvFile(t *testing.T) {
	clearREPEnv(t)
	path := writeTempEnvFile(t, "REP_PUBLIC_FROM_FILE=file_value\nREP_SENSITIVE_FILE_KEY=secret\n")

	vars, err := ReadAndClassify(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vars.Public) != 1 || vars.Public[0].Name != "FROM_FILE" || vars.Public[0].Value != "file_value" {
		t.Errorf("expected public var FROM_FILE=file_value from env file, got %+v", vars.Public)
	}
	if len(vars.Sensitive) != 1 || vars.Sensitive[0].Name != "FILE_KEY" {
		t.Errorf("expected sensitive var FILE_KEY from env file, got %+v", vars.Sensitive)
	}
}

func TestReadAndClassify_EnvOverridesFile(t *testing.T) {
	clearREPEnv(t)
	path := writeTempEnvFile(t, "REP_PUBLIC_API_URL=from_file\n")
	t.Setenv("REP_PUBLIC_API_URL", "from_env")

	vars, err := ReadAndClassify(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vars.Public) != 1 {
		t.Fatalf("expected 1 public var, got %d", len(vars.Public))
	}
	if vars.Public[0].Value != "from_env" {
		t.Errorf("process env should override file: expected from_env, got %s", vars.Public[0].Value)
	}
}
