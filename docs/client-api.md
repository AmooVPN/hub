# Client API

## Authentication
- Client API access uses JWT access tokens plus refresh tokens.
- Login issues both tokens.
- Protected endpoints require `Authorization: Bearer <access_token>`.

## Endpoints
- `POST /api/v1/client/auth/login`
- `POST /api/v1/client/auth/refresh`
- `POST /api/v1/client/auth/logout`
- `GET /api/v1/client/me`
- `GET /api/v1/client/subscription`
- `GET /api/v1/client/configs`
- `GET /api/v1/client/usage`
- `GET /api/v1/client/status`

## Login
Request:
```json
{ "username": "client", "password": "secret" }
```

Response:
```json
{ "access_token": "...", "refresh_token": "...", "token_type": "Bearer", "expires_in": 900 }
```

## Refresh
Request:
```json
{ "refresh_token": "..." }
```

Response:
```json
{ "access_token": "...", "refresh_token": "...", "token_type": "Bearer", "expires_in": 900 }
```

## Logout
Request:
```json
{ "refresh_token": "..." }
```

Response:
```json
{ "success": true }
```

## Me
Response:
```json
{
  "client": { "id": 1, "username": "client", "display_name": "Client", "email": "client@example.com", "status": "active" },
  "status": "active",
  "traffic_limit_bytes": 1073741824,
  "upload_bytes": 12345,
  "download_bytes": 67890,
  "total_bytes": 80235,
  "remaining_bytes": 1073661589,
  "active_configs": 2,
  "subscription_url": "https://hub.example.com/sub/token",
  "raw_subscription_url": "https://hub.example.com/sub/token/raw",
  "base64_subscription_url": "https://hub.example.com/sub/token/base64",
  "clash_url": "https://hub.example.com/sub/token/clash",
  "singbox_url": "https://hub.example.com/sub/token/singbox"
}
```

## Subscription
Response:
```json
{
  "subscription_url": "https://hub.example.com/sub/token",
  "formats": {
    "raw": "https://hub.example.com/sub/token/raw",
    "base64": "https://hub.example.com/sub/token/base64",
    "clash": "https://hub.example.com/sub/token/clash",
    "singbox": "https://hub.example.com/sub/token/singbox"
  },
  "active": true
}
```

## Configs
Response:
```json
{ "configs": [{ "id": 1, "panel_name": "panel-1", "inbound_remark": "main", "protocol": "vless", "enabled": true, "config": "vless://..." }] }
```

## Usage
Response:
```json
{ "upload_bytes": 12345, "download_bytes": 67890, "total_bytes": 80235, "traffic_limit_bytes": 1073741824, "remaining_traffic_bytes": 1073661589 }
```

## Status
Response:
```json
{ "status": "active", "is_active": true, "is_expired": false, "expiry_time": "2026-01-01T00:00:00Z", "remaining_seconds": 86400, "remaining_days": 1, "traffic_limit_bytes": 1073741824, "used_traffic_bytes": 80235, "remaining_traffic_bytes": 1073661589 }
```

## Error Example
```json
{ "error": { "code": "invalid_credentials", "message": "Invalid username or password" } }
```

## JWT Usage
- Include `Authorization: Bearer <access_token>` on protected requests.
- Access tokens are short-lived.
- Refresh tokens are rotated on refresh and revoked on logout.

## Rate Limits
- Login attempts are rate-limited per client username and IP.
- Failed login attempts increment a tracker and return `429` when the limit is exceeded.
- Refresh and logout require valid refresh tokens.
