package app

import (
	"html"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/openapi"
)

func (r *Runner) getOpenAPIJSON(c *fiber.Ctx) error {
	return c.Type("json").SendString(openapi.JSON)
}

func (r *Runner) getOpenAPIYAML(c *fiber.Ctx) error {
	return c.Type("yaml").SendString(openapi.YAML)
}

func (r *Runner) getOpenAPIDocs(c *fiber.Ctx) error {
	return c.Type("html").SendString(renderOpenAPIDocsPage(r.cfg.AppName))
}

func renderOpenAPIDocsPage(appName string) string {
	return renderPage("API Docs", `<main class="container-fluid py-3 py-lg-4"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(appName) + ` API Docs</h1><p class="text-body-secondary mb-0">OpenAPI 3.0 specification</p></div><div class="d-flex gap-2"><a class="btn btn-outline-secondary btn-sm" href="/api/openapi.json">JSON</a><a class="btn btn-outline-secondary btn-sm" href="/api/openapi.yaml">YAML</a><a class="btn btn-outline-secondary btn-sm" href="/admin/login">Admin</a></div></div><div class="card shadow-sm"><div class="card-body"><p class="text-body-secondary">Swagger UI is not bundled. Use the raw specification downloads below.</p><pre class="mb-0 small text-break">` + html.EscapeString(openapi.JSON) + `</pre></div></div></main>`)
}
