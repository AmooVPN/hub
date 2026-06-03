package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AmooVPM/hub/internal/openapi"
)

func main() {
	if err := os.MkdirAll("docs", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join("docs", "openapi.yaml"), []byte(openapi.YAML), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join("docs", "openapi.json"), []byte(openapi.JSON), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
