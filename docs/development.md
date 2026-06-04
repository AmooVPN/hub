# Development

## Local Setup
```bash
cp .env.example .env
go run ./cmd/hub migrate
go run ./cmd/hub create-admin
go run ./cmd/hub serve
```

## Running Redis
- Use a local Redis instance or `docker compose up redis`.

## Running Tests
```bash
go test ./...
```

## Project Structure
- `cmd/hub` entrypoint
- `internal/app` handlers and wiring
- `internal/config` configuration loading
- `internal/database` SQLite and Redis setup
- `internal/repositories` persistence layer
- `internal/services` business logic

## Adding Migrations
- Add migration files under `internal/migrations`.
- Keep schema changes backward compatible with the existing SQLite data.

## Adding Handlers
- Add route wiring in `internal/app/app.go`.
- Keep form handlers server-rendered and HTMX-friendly.

## Adding Services
- Keep remote panel logic in `internal/xui`.
- Keep business logic in `internal/services`.

## Adding Templates
- Preserve server-side rendering.
- Keep templates escaped and accessible.

## Mocking 3x-ui
- Use `httptest.Server` in tests.
- Cover login, list, add, and timeout behaviors.
