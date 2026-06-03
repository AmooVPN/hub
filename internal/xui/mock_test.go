package xui

import (
	"errors"
	"testing"
)

func TestNormalizeStatusError(t *testing.T) {
	if !errors.Is(normalizeStatusError(1, "op", 401), ErrXUIUnauthorized) {
		t.Fatal("expected unauthorized")
	}
	if !errors.Is(normalizeStatusError(1, "op", 404), ErrXUINotFound) {
		t.Fatal("expected not found")
	}
}
