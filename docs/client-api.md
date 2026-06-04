# Client API

## Authentication
JWT access tokens and refresh tokens are used for client API access.

## Endpoints
- `POST /api/v1/client/auth/login`
- `POST /api/v1/client/auth/refresh`
- `POST /api/v1/client/auth/logout`
- `GET /api/v1/client/me`
- `GET /api/v1/client/subscription`
- `GET /api/v1/client/configs`
- `GET /api/v1/client/usage`
- `GET /api/v1/client/status`

## Request Examples
```json
{ "username": "client", "password": "secret" }
```

```json
{ "refresh_token": "..." }
```

## Response Examples
```json
{ "access_token": "...", "refresh_token": "...", "token_type": "Bearer", "expires_in": 900 }
```

## Error Examples
```json
{ "error": { "code": "invalid_credentials", "message": "Invalid username or password" } }
```

## JWT Usage
- Send `Authorization: Bearer <access_token>` to protected endpoints.

## Rate Limits
- Login attempts are rate-limited per client username and IP.
- Refresh tokens are validated and revoked on use.
