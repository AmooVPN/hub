# Docker

## Compose
```bash
docker compose up -d --build
```

## Included Services
- `hub`
- `redis`

## Volumes
- `hub-data`
- `hub-backups`
- `redis-data`

## Notes
- The container healthcheck uses `hub healthcheck`.
- Set `APP_BASE_URL` to the public URL when running behind a proxy.
