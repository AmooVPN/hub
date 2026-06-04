# Installation

## Requirements
- Go 1.23+
- Redis 7+
- SQLite

## Local Setup
```bash
cp .env.example .env
go run ./cmd/hub migrate
go run ./cmd/hub create-admin
go run ./cmd/hub serve
```

## Notes
- `HUB_SECRET_KEY` is required.
- `DATABASE_PATH` and `BACKUP_DIR` must be writable.
- Use `TRUST_PROXY=true` only behind a trusted proxy.
