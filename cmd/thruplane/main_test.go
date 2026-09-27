package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIVersionDoesNotLoadConfiguration(t *testing.T) {
	var output bytes.Buffer
	err := runCLI([]string{"-version", "-config", filepath.Join(t.TempDir(), "missing.yaml")}, &output, testLogger())
	if err != nil {
		t.Fatalf("runCLI() error = %v", err)
	}
	for _, value := range []string{"thruplane", "version=dev", "revision=unknown", "build_date=unknown"} {
		if !strings.Contains(output.String(), value) {
			t.Errorf("version output = %q, want %q", output.String(), value)
		}
	}
}

func TestRunCLICheckConfig(t *testing.T) {
	path := writeCLIConfig(t, `
providers:
  local:
    type: ollama
models:
  local:
    targets:
      - provider: local
        model: llama3.2:latest
`)
	var output bytes.Buffer
	if err := runCLI([]string{"-check-config", "-config", path}, &output, testLogger()); err != nil {
		t.Fatalf("runCLI() error = %v", err)
	}
	if got := strings.TrimSpace(output.String()); got != "configuration valid: "+path {
		t.Fatalf("output = %q, want validation confirmation", got)
	}
}

func TestRunCLICheckConfigRejectsInvalidFileWithoutEchoingSecrets(t *testing.T) {
	path := writeCLIConfig(t, `
server:
  api_key: top-secret-value
  unexpected: true
providers:
  local:
    type: ollama
models:
  local:
    targets:
      - provider: local
        model: llama3.2:latest
`)
	var output bytes.Buffer
	err := runCLI([]string{"-check-config", "-config", path}, &output, testLogger())
	if err == nil || !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("runCLI() error = %v, want strict configuration error", err)
	}
	if strings.Contains(err.Error(), "top-secret-value") || strings.Contains(output.String(), "top-secret-value") {
		t.Fatal("configuration validation exposed the API key")
	}
}

func writeCLIConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
