package app

import (
	"path/filepath"
	"runtime"
)

func staticAssetDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "./web/static"
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "web", "static"))
}
