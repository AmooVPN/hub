# Development

## Local Setup
1. Copy `.env.example` to `.env` and set `HUB_SECRET_KEY`.
2. Start Redis locally or with Docker.
3. Run migrations.
4. Create the initial admin.
5. Start the app.

```bash
cp .env.example .env
docker compose up -d redis
go run ./cmd/hub migrate
go run ./cmd/hub create-admin
go run ./cmd/hub serve
```

## Running Redis
- Use a local Redis instance or `docker compose up redis`.
- The app exits on startup if Redis is unavailable.

## Running Tests
```bash
go test ./...
```

- The full suite uses SQLite and miniredis-backed tests.
- Run `go test ./internal/app ./internal/services` while iterating on handlers and business logic.

## Project Structure
- `cmd/hub` entrypoint
- `internal/app` handlers and wiring
- `internal/config` configuration loading
- `internal/database` SQLite and Redis setup
- `internal/repositories` persistence layer
- `internal/services` business logic
- `internal/xui` 3x-ui client wrapper
- `web/templates` HTML templates and shells
- `web/static` local frontend assets

## Adding Migrations
- Add migration files under `internal/migrations`.
- Keep schema changes backward compatible with the existing SQLite data.
- Update repository tests when new columns or tables are added.

## Adding Handlers
- Add route wiring in `internal/app/app.go`.
- Keep form handlers server-rendered and HTMX-friendly.
- Use shared shell/layout helpers instead of duplicating page chrome.
- Add route tests with `httptest.Server` or Fiber helpers when a flow is security-sensitive.

## Adding Services
- Keep remote panel logic in `internal/xui`.
- Keep business logic in `internal/services`.
- Favor small, testable services with explicit inputs and outputs.

## Adding Templates
- Preserve server-side rendering.
- Keep templates escaped and accessible.
- Prefer Bootstrap utility classes already used elsewhere in the app.
- Keep copy buttons, modal actions, and form labels consistent.

## Mocking 3x-ui
- Use `httptest.Server` in tests.
- Cover login, list, add, and timeout behaviors.
- Return realistic JSON payloads and test error paths like 401, 403, malformed JSON, and slow responses.
