# hub

Central multi-panel management hub for 3x-ui / Xray deployments.

## Features
- Multi-panel management
- Admin dashboard and client portal
- Client REST API and subscription links
- Backup and restore workflow
- Audit logs and notifications
- Redis-backed locks and caches

## Stack
- Go
- Fiber
- SQLite
- Redis
- HTMX
- Bootstrap 5

## Screenshots
- Placeholder: add screenshots of the dashboard, client portal, and admin forms.

## Quick Start
1. Copy `.env.example` to `.env` and set `HUB_SECRET_KEY`.
2. Start Redis.
3. Run the app:

```bash
go run ./cmd/hub serve
```

4. Open `http://localhost:8080`.

## Docker Compose
```bash
docker compose up -d --build
```

The compose file starts `hub` and `redis` with persistent volumes for `./data` and `./backups`.

## Environment Variables
- `APP_NAME`
- `APP_ENV`
- `APP_ADDR`
- `APP_BASE_URL`
- `DATABASE_PATH`
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `REDIS_DB`
- `SESSION_COOKIE_NAME`
- `SESSION_TTL_HOURS`
- `JWT_ACCESS_TTL_MINUTES`
- `JWT_REFRESH_TTL_DAYS`
- `HUB_SECRET_KEY`
- `BACKUP_DIR`
- `BACKUP_RETENTION_COUNT`
- `BACKUP_RETENTION_DAYS`
- `AUTOMATIC_BACKUP_ENABLED`
- `AUTOMATIC_BACKUP_SCHEDULE`
- `METRICS_ENABLED`
- `TRUST_PROXY`
- `TRUSTED_PROXIES`
- `MAX_UPLOAD_SIZE_MB`
- `INITIAL_ADMIN_USERNAME`
- `INITIAL_ADMIN_PASSWORD`
- `INITIAL_ADMIN_EMAIL`
- `INITIAL_ADMIN_ROLE`

## Initial Admin Setup
- On first start, the initial admin is bootstrapped from environment variables when the database is empty.
- Default values are shown in `.env.example`.

## Panel Workflow
### Add a Panel
- Open `/admin/panels/new`.
- Provide the panel base URL, username, and password.
- Only `http` and `https` panel URLs are accepted.

### Sync Inbounds
- Open a panel detail page.
- Use `Sync` to pull remote inbounds.

### Create a Client
- Open `/admin/clients/new`.
- Set username, password, and optional limits/expiry.

### Attach a Client to Inbounds
- Open a client detail page.
- Use `Manage attachments` to select inbounds.

## Subscription Links
- Client subscriptions are exposed under `/sub/:token`.
- Raw, Base64, Clash, and Sing-box variants are available from the client and admin views.

## Client API Overview
- `POST /api/v1/client/auth/login`
- `POST /api/v1/client/auth/refresh`
- `POST /api/v1/client/auth/logout`
- `GET /api/v1/client/me`
- `GET /api/v1/client/subscription`
- `GET /api/v1/client/configs`
- `GET /api/v1/client/usage`
- `GET /api/v1/client/status`

## Backup / Restore
- Backup exports create a zip archive containing `metadata.json` and `hub.db`.
- Import validation creates a safety backup before staging the uploaded archive.
- Full restore wiring is still evolving; see `docs/backup-restore.md` for current behavior.

## Security Notes
- SQLite is the primary database.
- Redis is required for locks and cache.
- Panel credentials are encrypted.
- JWTs and refresh tokens are short-lived.
- CSRF protection is enabled for HTML forms.

## Roadmap
- See `docs/roadmap.md` and `TODO.md`.

## License
- No license file has been added yet.
