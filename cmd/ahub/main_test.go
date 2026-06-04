package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateEnvFileUpdatesValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("APP_PORT=8080\nAPP_BASE_URL=http://localhost:8080\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := updateEnvFile(path, map[string]string{"APP_PORT": "9090", "APP_BASE_URL": "http://localhost:9090"}); err != nil {
		t.Fatalf("update env: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	text := string(data)
	for _, expected := range []string{"APP_PORT=9090", "APP_BASE_URL=http://localhost:9090"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in %s", expected, text)
		}
	}
}

func TestPrintHelp(t *testing.T) {
	var buf strings.Builder
	printHelp(&buf)
	if got := buf.String(); !strings.Contains(got, "ahub create env") || !strings.Contains(got, "ahub start") || !strings.Contains(got, "--no-follow") {
		t.Fatalf("unexpected help text: %s", got)
	}
}

func TestBuildLogsComposeArgs(t *testing.T) {
	args, err := buildLogsComposeArgs([]string{"--tail", "25", "--since", "1h", "--no-follow"})
	if err != nil {
		t.Fatalf("build logs args: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, expected := range []string{"logs", "--tail 25", "--since 1h", "hub"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q in %q", expected, joined)
		}
	}
	if strings.Contains(joined, "-f") {
		t.Fatalf("did not expect follow flag in %q", joined)
	}
}
