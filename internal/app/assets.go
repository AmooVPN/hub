package app

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func staticAssetDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "./web/static"
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "web", "static"))
}

func staticAssetURL(rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "/static"
	}
	rel = strings.TrimLeft(path.Clean("/"+rel), "/")
	version := staticAssetVersion(rel)
	if version == "" {
		return "/static/" + rel
	}
	return "/static/" + rel + "?v=" + version
}

func staticAssetVersion(rel string) string {
	assetPath := filepath.Join(staticAssetDir(), filepath.FromSlash(rel))
	info, err := os.Stat(assetPath)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(info.ModTime().UTC().Unix(), 10)
}

func isStaticAssetRequest(c *fiber.Ctx) bool {
	return strings.HasPrefix(c.Path(), "/static/")
}

func staticAssetCacheControl() string {
	return "public, max-age=31536000, immutable"
}

func themeScript() string {
	return `<script>(function(){var mq=window.matchMedia('(prefers-color-scheme: dark)');function apply(){document.documentElement.setAttribute('data-bs-theme',mq.matches?'dark':'light');}apply();if(mq.addEventListener){mq.addEventListener('change',apply);}else if(mq.addListener){mq.addListener(apply);}})();</script>`
}
