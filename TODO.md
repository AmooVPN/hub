# TODO.md — hub

Version: 1.0

Status: Planning Phase

Repository:

```txt
github.com/AmooVPM/hub
```

Project Name:

```txt
hub
```

Tagline:

```txt
Central Multi-Panel Management Hub for 3x-ui / Xray Deployments
```

---

# 1. Vision

hub is a centralized management platform that connects to multiple independent 3x-ui panels and allows administrators to manage the entire infrastructure from a single dashboard.

The goal is to eliminate the need to manually log into multiple 3x-ui instances and provide:

* Centralized client management
* Centralized inbound management
* Centralized subscription management
* Unified client portal
* Unified REST API
* Monitoring
* Auditing
* Backup and restore
* Multi-admin management

hub must never directly manipulate a 3x-ui database.

All communication with remote panels must happen through officially exposed APIs.

---

# 2. Primary Objectives

Build a platform where:

* One admin manages many panels.
* One client may exist on multiple panels.
* One subscription link represents multiple configs.
* Clients have their own dashboard.
* Clients have API access.
* All actions are logged.
* All data is backed up.
* Infrastructure remains portable through SQLite backups.

---

# 3. High-Level Architecture

```txt
                       ┌──────────────────┐
                       │     Client App   │
                       └────────┬─────────┘
                                │
                                │
                                ▼

                    ┌──────────────────────┐
                    │        hub          │
                    │                      │
                    │ Admin Dashboard      │
                    │ Client Portal        │
                    │ REST API             │
                    │ Subscription Engine  │
                    │ Monitoring           │
                    │ Audit System         │
                    └──────────┬───────────┘
                               │
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
          ▼                    ▼                    ▼

    ┌───────────┐      ┌───────────┐      ┌───────────┐
    │ 3x-ui #1  │      │ 3x-ui #2  │      │ 3x-ui #3  │
    └───────────┘      └───────────┘      └───────────┘
```

---

# 4. Technology Stack

Backend:

```txt
Go
Fiber
```

Frontend:

```txt
HTMX
Bootstrap 5
Bootstrap Icons
```

Database:

```txt
SQLite
```

Cache:

```txt
Redis
```

Authentication:

```txt
Sessions
JWT
Refresh Tokens
```

Documentation:

```txt
Swagger/OpenAPI
```

Containerization:

```txt
Docker
Docker Compose
```

Testing:

```txt
Go Test
httptest
```

---

# 5. Non-Negotiable Requirements

The application MUST:

* Use SQLite as primary database.
* Use Redis for cache and distributed locks.
* Support dark mode.
* Be mobile responsive.
* Work without JavaScript frameworks.
* Use HTMX for dynamic interactions.
* Use server-side rendering.
* Support JWT authentication.
* Support refresh tokens.
* Support SQLite import/export.
* Support API-first design.
* Support multiple 3x-ui panels.
* Support multiple admins.
* Support auditing.
* Support backup/restore.

The application MUST NOT:

* Directly manipulate 3x-ui databases.
* Store plain text passwords.
* Store plain text panel credentials.
* Require PostgreSQL.
* Require Kubernetes.

---

# 6. Project Structure

```txt
hub/
│
├── cmd/
│   └── hub/
│       └── main.go
│
├── internal/
│   │
│   ├── app/
│   ├── config/
│   ├── database/
│   ├── migrations/
│   ├── middleware/
│   ├── models/
│   ├── repositories/
│   ├── services/
│   ├── handlers/
│   ├── validators/
│   ├── jobs/
│   ├── audit/
│   ├── metrics/
│   ├── security/
│   ├── notifications/
│   ├── subscriptions/
│   ├── webhooks/
│   ├── xui/
│   └── utils/
│
├── web/
│   │
│   ├── templates/
│   │   ├── layouts/
│   │   ├── partials/
│   │   ├── admin/
│   │   ├── client/
│   │   └── public/
│   │
│   └── static/
│       ├── css/
│       ├── js/
│       ├── img/
│       └── fonts/
│
├── docs/
│
├── tests/
│
├── backups/
│
├── data/
│
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── README.md
├── TODO.md
└── .env.example
```

---

# 7. Environment Configuration

Create:

```txt
.env.example
```

Contents:

```env
APP_NAME=hub
APP_ENV=development

APP_ADDR=0.0.0.0:8080

APP_BASE_URL=http://localhost:8080

DATABASE_PATH=./data/hub.db

REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

SESSION_COOKIE_NAME=hub_session

SESSION_TTL_HOURS=168

JWT_ACCESS_TTL_MINUTES=15

JWT_REFRESH_TTL_DAYS=30

HUB_SECRET_KEY=replace_me

BACKUP_DIR=./backups

MAX_UPLOAD_SIZE_MB=100
```

---

# 8. Startup Requirements

On startup:

* Load config.
* Validate config.
* Create data directory.
* Create backups directory.
* Connect SQLite.
* Connect Redis.
* Apply migrations.
* Register routes.
* Start HTTP server.

Startup must fail if:

* SQLite unavailable.
* Secret key missing.
* Migrations fail.

---

# 9. SQLite Configuration

After opening DB execute:

```sql
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=5000;
```

---

# 10. Database Migration System

Requirements:

* Versioned migrations.
* Automatic migration execution.
* Up migrations only initially.
* Migration history table.

Create:

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL
);
```

Tasks:

* [ ] Migration runner
* [ ] Migration registry
* [ ] Schema version validation

---

# 11. Database Schema

## admin_users

```sql
CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

---

## clients

```sql
CREATE TABLE clients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT,
    email TEXT,
    status TEXT NOT NULL,
    traffic_limit_bytes INTEGER DEFAULT 0,
    expiry_time DATETIME,
    subscription_token TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

---

## panels

```sql
CREATE TABLE panels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    encrypted_password TEXT NOT NULL,
    version TEXT,
    status TEXT NOT NULL,
    last_sync_at DATETIME,
    last_error TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

---

## inbounds

```sql
CREATE TABLE inbounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    panel_id INTEGER NOT NULL,
    remote_inbound_id INTEGER NOT NULL,
    remark TEXT,
    protocol TEXT,
    port INTEGER,
    network TEXT,
    security TEXT,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    raw_json TEXT,
    last_synced_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

---

## client_attachments

```sql
CREATE TABLE client_attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id INTEGER NOT NULL,
    panel_id INTEGER NOT NULL,
    inbound_id INTEGER NOT NULL,

    remote_client_id TEXT,
    remote_email TEXT,

    enabled BOOLEAN NOT NULL DEFAULT 1,

    upload_bytes INTEGER DEFAULT 0,
    download_bytes INTEGER DEFAULT 0,

    traffic_limit_bytes INTEGER DEFAULT 0,

    expiry_time DATETIME,

    raw_config TEXT,

    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

---

## client_refresh_tokens

```sql
CREATE TABLE client_refresh_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id INTEGER NOT NULL,

    token_hash TEXT NOT NULL UNIQUE,

    user_agent TEXT,

    ip_address TEXT,

    revoked_at DATETIME,

    expires_at DATETIME NOT NULL,

    created_at DATETIME NOT NULL
);
```

---

## traffic_snapshots

```sql
CREATE TABLE traffic_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    client_id INTEGER NOT NULL,

    attachment_id INTEGER,

    upload_bytes INTEGER NOT NULL,

    download_bytes INTEGER NOT NULL,

    total_bytes INTEGER NOT NULL,

    captured_at DATETIME NOT NULL
);
```

---

## audit_logs

```sql
CREATE TABLE audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    actor_type TEXT NOT NULL,

    actor_id INTEGER,

    action TEXT NOT NULL,

    target_type TEXT,

    target_id INTEGER,

    metadata_json TEXT,

    created_at DATETIME NOT NULL
);
```

---

## sync_jobs

```sql
CREATE TABLE sync_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    panel_id INTEGER,

    job_type TEXT NOT NULL,

    status TEXT NOT NULL,

    message TEXT,

    started_at DATETIME,

    finished_at DATETIME,

    created_at DATETIME NOT NULL
);
```

---

# 12. Repository Layer

Every table must have:

```txt
Repository Interface
Repository Implementation
Unit Tests
```

Examples:

```txt
AdminRepository
ClientRepository
PanelRepository
InboundRepository
AttachmentRepository
AuditRepository
```

---

# 13. Models

Create models for:

```txt
AdminUser
Client
Panel
Inbound
ClientAttachment
TrafficSnapshot
AuditLog
SyncJob
RefreshToken
```

Each model should contain:

```txt
Validation
JSON serialization
DTO conversion
```

---

# 14. Encryption

Panel passwords must be encrypted using:

```txt
AES-256-GCM
```

Key source:

```txt
HUB_SECRET_KEY
```

Tasks:

* [ ] Encrypt panel credentials
* [ ] Decrypt panel credentials
* [ ] Rotation strategy design

---

# 15. Password Hashing

Supported:

```txt
Argon2id (preferred)
bcrypt (fallback)
```

Requirements:

* Never store plain passwords.
* Never log passwords.
* Never expose hashes.

---

# 16. Initial Milestone

Phase 1 completion criteria:

* [x] SQLite starts correctly.
* [x] Redis connects.
* [x] Migrations run.
* [x] Config loads.
* [x] Encryption works.
* [x] Password hashing works.
* [x] Project structure exists.
* [x] Repositories implemented.
* [x] Models implemented.



# PART 2 — Authentication, RBAC, Admin Portal, Client Portal, REST API, JWT

---

# 17. Authentication Overview

hub must support two separate authentication systems:

```txt
1. Admin authentication
2. Client authentication
```

Admin authentication is used for the main management dashboard.

Client authentication is used for:

```txt
- Client web portal
- Client REST API
- JWT access
```

---

# 18. Admin Authentication

Admin login must use server-side sessions.

Routes:

```txt
GET  /admin/login
POST /admin/login
POST /admin/logout
GET  /admin
```

Tasks:

* [x] Create admin login page.
* [x] Create admin logout action.
* [x] Create secure session storage.
* [x] Add session middleware.
* [x] Protect all `/admin/*` routes.
* [x] Redirect unauthenticated admins to `/admin/login`.
* [x] Add login rate limiting.
* [x] Add audit log for successful login.
* [x] Add audit log for failed login.
* [x] Add audit log for logout.
* [ ] Add remember-me option if needed later.

Session requirements:

```txt
- HTTPOnly cookie
- SameSite=Lax
- Secure=true in production
- Configurable TTL
```

Acceptance criteria:

* Admin can log in.
* Admin can log out.
* Admin cannot access `/admin/*` without session.
* Failed login attempts are rate limited.
* Login events are audited.

---

# 19. First Admin Bootstrap

On first startup, if no admin exists:

Option A:

```txt
Create default admin from env variables.
```

Env variables:

```env
INITIAL_ADMIN_USERNAME=admin
INITIAL_ADMIN_PASSWORD=change-me-now
```

Option B:

```txt
Show one-time setup page.
```

Recommended MVP:

```txt
Use env variables first.
Add setup page later.
```

Tasks:

* [x] Check if `admin_users` table is empty.
* [x] If empty, create initial admin from env.
* [x] Refuse production startup if initial password is weak/default.
* [ ] Force password change flag can be added later.

---

# 20. RBAC — Role-Based Access Control

Support admin roles:

```txt
owner
admin
support
readonly
```

Role definitions:

## owner

Can do everything.

```txt
- Manage admins
- Manage panels
- Manage clients
- Manage backups
- Import database
- Export database
- View audit logs
- Manage settings
```

## admin

Can manage operational resources.

```txt
- Manage panels
- Manage inbounds
- Manage clients
- Manage subscriptions
- View dashboard
- View audit logs
```

Cannot:

```txt
- Delete owner
- Import database
- Change global security settings
```

## support

Can support clients.

```txt
- View clients
- View configs
- Reset client password
- Enable/disable client
- View usage
```

Cannot:

```txt
- Add panels
- Delete panels
- Import backups
- Export backups
- Manage admins
```

## readonly

Can only view.

```txt
- View dashboard
- View panels
- View inbounds
- View clients
- View usage
```

Cannot change anything.

Tasks:

* [x] Create role constants.
* [x] Create permission constants.
* [x] Add permission middleware.
* [x] Add helper `RequirePermission`.
* [x] Add helper `RequireRole`.
* [x] Hide UI actions if admin lacks permission.
* [x] Return 403 for forbidden actions.
* [x] Add tests for permissions.

Acceptance criteria:

* Different admin roles have different access levels.
* UI does not show forbidden actions.
* Backend still blocks forbidden actions.

---

# 21. Admin User Management

Routes:

```txt
GET  /admin/users
GET  /admin/users/new
POST /admin/users
GET  /admin/users/:id/edit
POST /admin/users/:id
POST /admin/users/:id/disable
POST /admin/users/:id/enable
POST /admin/users/:id/delete
POST /admin/users/:id/reset-password
```

Tasks:

* [x] List admin users.
* [x] Create admin user.
* [x] Edit admin user.
* [x] Change role.
* [x] Disable admin user.
* [x] Enable admin user.
* [x] Delete admin user.
* [x] Prevent deleting last owner.
* [x] Prevent disabling last owner.
* [x] Add audit logs.

Acceptance criteria:

* Owner can manage admins.
* Last owner cannot be removed.
* Disabled admin cannot log in.

---

# 22. Optional Admin 2FA / TOTP

Not required for MVP, but design now.

Tables:

```sql
ALTER TABLE admin_users ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE admin_users ADD COLUMN totp_secret_encrypted TEXT;
```

Routes:

```txt
GET  /admin/security/2fa
POST /admin/security/2fa/setup
POST /admin/security/2fa/confirm
POST /admin/security/2fa/disable
```

Tasks:

* [ ] Add TOTP setup page later.
* [ ] Add QR code generation.
* [ ] Add recovery codes later.
* [ ] Require password confirmation before disabling 2FA.

Roadmap only. Do not implement in MVP unless foundation is complete.

---

# 23. Client Authentication — Web Portal

Client web portal uses sessions.

Routes:

```txt
GET  /client/login
POST /client/login
POST /client/logout
GET  /client
```

Tasks:

* [x] Create client login page.
* [x] Create client logout action.
* [x] Add client session middleware.
* [x] Protect `/client/*`.
* [x] Redirect unauthenticated clients to `/client/login`.
* [x] Rate-limit client login.
* [x] Block disabled clients.
* [x] Allow expired clients to log in but show expired status.

Acceptance criteria:

* Client can log in.
* Client can log out.
* Client cannot access another client’s data.
* Disabled client cannot log in.
* Expired client can log in but active configs are hidden.

---

# 24. Client Portal Pages

Routes:

```txt
GET  /client
GET  /client/profile
POST /client/profile/password
GET  /client/configs
GET  /client/subscription
GET  /client/usage
```

## Client Dashboard

Show:

```txt
- Account status
- Expiry time
- Remaining time
- Total traffic used
- Remaining traffic
- Active configs count
- Subscription link
```

## Client Profile

Show:

```txt
- Username
- Display name
- Email
- Password change form
```

## Client Configs

Show:

```txt
- Config list
- Panel name
- Inbound remark
- Protocol
- Status
- Copy button
- QR code
```

## Client Subscription

Show:

```txt
- Main subscription URL
- Raw subscription URL
- Base64 subscription URL
- Clash URL placeholder
- Sing-box URL placeholder
```

## Client Usage

Show:

```txt
- Upload bytes
- Download bytes
- Total bytes
- Traffic limit
- Remaining traffic
```

Tasks:

* [x] Create client layout.
* [x] Create client dashboard.
* [x] Create client profile page.
* [x] Create password change flow.
* [x] Create configs page.
* [x] Create subscription page.
* [x] Create usage page.
* [x] Add copy buttons.
* [x] Add QR codes.
* [x] Add mobile responsive cards.

Acceptance criteria:

* Client can manage own portal password.
* Client can copy configs.
* Client can copy subscription links.
* Client can see usage and expiry.

---

# 25. Client REST API Overview

Base path:

```txt
/api/v1/client
```

The client API must allow clients to authenticate programmatically and access:

```txt
- Own profile
- Subscription links
- Configs
- Usage
- Status
- Remaining time
```

Authentication:

```txt
JWT Bearer token
```

Header:

```http
Authorization: Bearer <access_token>
```

---

# 26. JWT Authentication Flow

Routes:

```txt
POST /api/v1/client/auth/login
POST /api/v1/client/auth/refresh
POST /api/v1/client/auth/logout
GET  /api/v1/client/me
```

Access token:

```txt
JWT
TTL: 15 minutes
```

Refresh token:

```txt
Opaque random token
TTL: 30 days
Stored hashed
```

Tasks:

* [x] Add JWT signing service.
* [x] Add JWT validation service.
* [x] Add refresh token generator.
* [x] Store hashed refresh tokens.
* [x] Add token revocation.
* [x] Add refresh endpoint.
* [x] Add logout endpoint.
* [x] Add API rate limiting.
* [x] Add JWT middleware.
* [x] Add tests.

---

# 27. API Login

Route:

```txt
POST /api/v1/client/auth/login
```

Request:

```json
{
  "username": "client_username",
  "password": "client_password"
}
```

Success response:

```json
{
  "access_token": "jwt_access_token",
  "refresh_token": "refresh_token",
  "token_type": "Bearer",
  "expires_in": 900
}
```

Failure response:

```json
{
  "error": {
    "code": "invalid_credentials",
    "message": "Invalid username or password"
  }
}
```

Rules:

* Disabled clients cannot receive tokens.
* Expired clients may receive tokens.
* Expired clients should receive status information but no active configs.
* Login attempts must be rate-limited.
* Refresh token must be stored hashed.

Acceptance criteria:

* Client can authenticate.
* Client receives access token and refresh token.
* Invalid credentials return safe generic error.
* Disabled client cannot authenticate.

---

# 28. API Refresh

Route:

```txt
POST /api/v1/client/auth/refresh
```

Request:

```json
{
  "refresh_token": "opaque_refresh_token"
}
```

Response:

```json
{
  "access_token": "new_jwt_access_token",
  "refresh_token": "new_refresh_token",
  "token_type": "Bearer",
  "expires_in": 900
}
```

Rules:

* Rotate refresh token on every refresh.
* Revoke old refresh token.
* Reject expired refresh token.
* Reject revoked refresh token.
* Store only hash of refresh token.

Acceptance criteria:

* Refresh works.
* Refresh token rotation works.
* Reused old refresh token fails.

---

# 29. API Logout

Route:

```txt
POST /api/v1/client/auth/logout
```

Request:

```json
{
  "refresh_token": "opaque_refresh_token"
}
```

Response:

```json
{
  "success": true
}
```

Tasks:

* [x] Revoke provided refresh token.
* [x] Add audit log.
* [ ] Access token remains valid until expiry unless blacklist is added later.

---

# 30. API Error Format

Every API error must use:

```json
{
  "error": {
    "code": "error_code",
    "message": "Human readable message"
  }
}
```

Common codes:

```txt
invalid_credentials
unauthorized
forbidden
not_found
validation_error
rate_limited
client_disabled
client_expired
internal_error
```

Tasks:

* [x] Create API response helper.
* [x] Create API error helper.
* [x] Add consistent HTTP status codes.

---

# 31. Client API Endpoints

Routes:

```txt
GET /api/v1/client/me
GET /api/v1/client/subscription
GET /api/v1/client/configs
GET /api/v1/client/usage
GET /api/v1/client/status
```

---

# 32. GET /api/v1/client/me

Response:

```json
{
  "id": 12,
  "username": "ali",
  "display_name": "Ali",
  "email": "ali@example.com",
  "status": "active",
  "expiry_time": "2026-07-01T00:00:00Z"
}
```

Tasks:

* [x] Load client from JWT subject.
* [x] Return safe client profile.
* [x] Do not expose password hash.
* [x] Do not expose internal subscription token unless needed through subscription endpoint.

---

# 33. GET /api/v1/client/subscription

Response:

```json
{
  "subscription_url": "https://hub.example.com/sub/CLIENT_TOKEN",
  "formats": {
    "raw": "https://hub.example.com/sub/CLIENT_TOKEN/raw",
    "base64": "https://hub.example.com/sub/CLIENT_TOKEN/base64",
    "clash": "https://hub.example.com/sub/CLIENT_TOKEN/clash",
    "singbox": "https://hub.example.com/sub/CLIENT_TOKEN/singbox"
  }
}
```

Rules:

* Disabled clients: return 403.
* Expired clients: return URLs but mark inactive, or return no active configs depending on config.
* Use `APP_BASE_URL`.

Tasks:

* [x] Add subscription URL builder.
* [x] Add format URLs.
* [x] Add tests.

---

# 34. GET /api/v1/client/configs

Response:

```json
{
  "configs": [
    {
      "id": 1,
      "panel_name": "Germany Panel",
      "inbound_remark": "VLESS Reality 443",
      "protocol": "vless",
      "enabled": true,
      "config": "vless://..."
    }
  ]
}
```

Rules:

* Only return active attachments.
* Do not return configs for disabled clients.
* Do not return configs for expired clients unless global setting allows expired visibility.
* Never return configs belonging to another client.

Tasks:

* [x] Create configs service.
* [x] Add active config filtering.
* [ ] Add tests.

---

# 35. GET /api/v1/client/usage

Response:

```json
{
  "upload_bytes": 123456,
  "download_bytes": 987654,
  "total_bytes": 1111110,
  "traffic_limit_bytes": 107374182400,
  "remaining_traffic_bytes": 106263071290
}
```

Tasks:

* [x] Aggregate usage from client attachments.
* [x] Calculate total upload.
* [x] Calculate total download.
* [x] Calculate total traffic.
* [x] Calculate remaining traffic.
* [x] If traffic limit is zero, treat as unlimited.
* [ ] Add tests.

---

# 36. GET /api/v1/client/status

Response:

```json
{
  "status": "active",
  "is_active": true,
  "is_expired": false,
  "expiry_time": "2026-07-01T00:00:00Z",
  "remaining_seconds": 2419200,
  "remaining_days": 28,
  "traffic_limit_bytes": 107374182400,
  "used_traffic_bytes": 1111110,
  "remaining_traffic_bytes": 106263071290
}
```

Tasks:

* [x] Calculate remaining seconds.
* [x] Calculate remaining days.
* [x] Calculate expired status.
* [x] Calculate traffic status.
* [ ] Add tests.

---

# 37. OpenAPI / Swagger

Add OpenAPI documentation for all API routes.

Route:

```txt
GET /api/docs
```

Requirements:

* Document auth login.
* Document refresh.
* Document logout.
* Document client profile.
* Document subscription endpoint.
* Document configs endpoint.
* Document usage endpoint.
* Document status endpoint.
* Document error format.
* Document JWT bearer auth.

Tasks:

* [x] Add OpenAPI spec.
* [x] Add Swagger UI.
* [x] Add docs generation command.
* [x] Keep docs versioned.

---

# 38. Admin Dashboard

Route:

```txt
GET /admin
```

Show:

```txt
- Total panels
- Online panels
- Offline panels
- Total inbounds
- Total clients
- Active clients
- Disabled clients
- Expired clients
- Total traffic
- Recent sync jobs
- Recent audit logs
- Recent errors
```

Tasks:

* [x] Create dashboard service.
* [x] Create dashboard handler.
* [x] Create dashboard template.
* [x] Add HTMX refresh for health widgets.
* [x] Add cards.
* [x] Add responsive layout.

Acceptance criteria:

* Admin sees useful overview after login.
* Dashboard works on mobile and desktop.

---

# 39. Admin Navigation

Sidebar items:

```txt
Dashboard
Panels
Inbounds
Clients
Subscriptions
Sync Jobs
Backups
Audit Logs
Settings
Admin Users
```

Tasks:

* [x] Build responsive sidebar.
* [x] Collapse sidebar on mobile.
* [x] Highlight active page.
* [x] Hide menu entries based on RBAC permissions.

---

# 40. Admin Settings

Routes:

```txt
GET  /admin/settings
POST /admin/settings
```

Settings to support later:

```txt
- App base URL
- Subscription behavior
- Expired client visibility
- Backup retention count
- Backup retention days
- Redis required mode
- Webhook settings
- Notification settings
```

For MVP, settings may be env-based only.

Tasks:

* [x] Create settings page placeholder.
* [x] Show current effective config.
* [x] Hide secrets.
* [ ] Add future database-backed settings.

---

# 41. Client API Rate Limiting

Apply rate limits to:

```txt
POST /api/v1/client/auth/login
POST /api/v1/client/auth/refresh
GET  /api/v1/client/*
```

Suggested defaults:

```txt
Login: 5 attempts per minute per IP
Refresh: 20 per minute per client/IP
Read endpoints: 120 per minute per client
```

Tasks:

* [ ] Redis-backed limiter.
* [ ] IP-based limiter.
* [ ] Client-ID-based limiter after auth.
* [ ] Return 429 with API error format.

---

# 42. Session Security

Admin and client portal sessions:

```txt
HTTPOnly
Secure in production
SameSite=Lax
Path scoped
Configurable TTL
```

Tasks:

* [ ] Add secure cookie config.
* [ ] Add session regeneration after login.
* [ ] Add logout session invalidation.
* [ ] Add tests.

---

# 43. Acceptance Criteria for Part 2

Part 2 is complete when:

* Admin can log in and log out.
* Client can log in and log out.
* Admin RBAC works.
* Admin dashboard loads.
* Client portal dashboard loads.
* Client can change password.
* Client API login returns JWT.
* Client API refresh rotates refresh tokens.
* Client API logout revokes refresh tokens.
* Client API returns profile.
* Client API returns subscription URLs.
* Client API returns configs.
* Client API returns usage.
* Client API returns status and remaining time.
* Swagger/OpenAPI docs exist.
* API errors use consistent format.
* Rate limits exist for login and API access.



# PART 3 — XUI Connector, Panels, Inbounds, Clients, Attachments, Subscriptions

---

# 44. 3x-ui Connector Overview

hub must communicate with each remote `3x-ui` panel through HTTP/API only.

The connector layer must be isolated in:

```txt
internal/xui/
```

or:

```txt
internal/services/xui/
```

Recommended package:

```txt
internal/xui/
```

The connector must hide all 3x-ui-specific API details from the rest of the application.

The rest of hub should call high-level methods like:

```go
Login(ctx context.Context) error

ListInbounds(ctx context.Context) ([]XUIInbound, error)

AddClient(ctx context.Context, inboundID int, client XUIClientCreateRequest) (*XUIClientResult, error)

UpdateClient(ctx context.Context, inboundID int, clientID string, req XUIClientUpdateRequest) error

DeleteClient(ctx context.Context, inboundID int, clientID string) error

ResetClientTraffic(ctx context.Context, inboundID int, clientID string) error

GetClientTraffic(ctx context.Context, clientID string) (*XUITraffic, error)
```

---

# 45. XUI Connector Goals

The connector must support:

* Login to remote panel.
* Cookie/session persistence.
* Auto re-login when session expires.
* Panel connection test.
* Panel version detection if available.
* Inbound list sync.
* Remote client creation.
* Remote client update.
* Remote client deletion.
* Remote client enable/disable when available.
* Traffic sync.
* Config generation.
* Safe error handling.
* API compatibility across different 3x-ui versions.

---

# 46. XUI Connector Structure

Create:

```txt
internal/xui/
├── client.go
├── auth.go
├── inbound.go
├── client_user.go
├── traffic.go
├── config.go
├── errors.go
├── types.go
├── compat.go
└── mock_test.go
```

Responsibilities:

```txt
client.go       Core HTTP client
auth.go         Login and session handling
inbound.go      Inbound API methods
client_user.go  Client/user management methods
traffic.go      Traffic-related methods
config.go       Config/subscription generation helpers
errors.go       Error normalization
types.go        DTOs
compat.go       Version compatibility helpers
```

---

# 47. XUI HTTP Client Requirements

Tasks:

* [x] Create `XUIClient` struct.
* [x] Store panel ID.
* [x] Store base URL.
* [x] Store username.
* [x] Store decrypted password only in memory.
* [x] Store HTTP client with timeout.
* [x] Support custom User-Agent.
* [x] Normalize trailing slash in base URL.
* [x] Support context cancellation.
* [ ] Add structured logging without secrets.
* [x] Add safe request helper.
* [x] Add JSON encode/decode helpers.
* [ ] Add form request helper if 3x-ui endpoint requires form login.

Timeouts:

```txt
Login timeout: 10 seconds
Normal request timeout: 20 seconds
Sync timeout: 60 seconds
```

---

# 48. XUI Session Handling

Remote 3x-ui panels may use session cookies.

Tasks:

* [ ] Store cookies in Redis.
* [ ] Key cookies by panel ID.
* [ ] Reuse cookies between requests.
* [ ] Detect expired session.
* [ ] Auto re-login once when session is expired.
* [ ] Never retry unsafe mutation blindly more than once.
* [ ] Clear cookies on authentication failure.
* [ ] Update panel last_login_at after successful login.

Redis key:

```txt
hub:panel:{panel_id}:cookies
```

Acceptance criteria:

* hub does not login on every request.
* Expired sessions recover automatically.
* Wrong credentials mark panel as authentication error.

---

# 49. XUI Error Model

Create normalized errors:

```go
var (
    ErrXUIUnauthorized = errors.New("xui unauthorized")
    ErrXUIForbidden    = errors.New("xui forbidden")
    ErrXUINotFound     = errors.New("xui not found")
    ErrXUITimeout      = errors.New("xui timeout")
    ErrXUIUnavailable  = errors.New("xui unavailable")
    ErrXUIBadResponse  = errors.New("xui bad response")
)
```

Also create typed error:

```go
type XUIError struct {
    PanelID    int64
    Operation  string
    StatusCode int
    Message    string
    Cause      error
}
```

Tasks:

* [x] Normalize HTTP status errors.
* [x] Normalize JSON parsing errors.
* [x] Normalize network errors.
* [x] Normalize 3x-ui API error messages.
* [x] Expose user-friendly message to handlers.
* [ ] Keep technical error in logs.

---

# 50. XUI Compatibility Layer

Different 3x-ui versions may expose slightly different endpoints or response formats.

Tasks:

* [ ] Add connector version detection.
* [ ] Add fallback endpoint support.
* [ ] Add compatibility flags.
* [ ] Store panel version when detected.
* [ ] Do not crash if version endpoint does not exist.
* [ ] Add warning to panel detail page if compatibility is partial.

Suggested struct:

```go
type XUICompatibility struct {
    Version string
    SupportsClientEnableDisable bool
    SupportsTrafficReset        bool
    SupportsOnlineUsers         bool
    SupportsSubscriptionAPI     bool
}
```

---

# 51. Panel Service

Create:

```txt
internal/services/panel/
```

Responsibilities:

* Manage panel CRUD.
* Encrypt/decrypt panel password.
* Test connection.
* Sync panel metadata.
* Update panel status.
* Coordinate XUI connector.

Tasks:

* [ ] Create panel service.
* [ ] Add create panel.
* [ ] Add update panel.
* [ ] Add delete panel.
* [ ] Add test connection.
* [ ] Add sync metadata.
* [ ] Add status update.
* [ ] Add audit logs.
* [ ] Add repository integration.

---

# 52. Panel Management Routes

Routes:

```txt
GET  /admin/panels
GET  /admin/panels/new
POST /admin/panels
GET  /admin/panels/:id
GET  /admin/panels/:id/edit
POST /admin/panels/:id
POST /admin/panels/:id/delete
POST /admin/panels/:id/test
POST /admin/panels/:id/sync
POST /admin/panels/:id/clear-session
```

Tasks:

* [x] List panels.
* [x] Show panel create form.
* [x] Save new panel.
* [x] Validate base URL.
* [x] Validate username/password.
* [x] Encrypt password before storing.
* [x] Show panel detail.
* [x] Edit panel.
* [x] Delete panel.
* [x] Test connection.
* [x] Sync panel.
* [x] Clear saved session cookie.
* [x] Show last error.
* [x] Show last sync time.
* [x] Show detected version.

Acceptance criteria:

* Admin can add a panel.
* Admin can test a panel.
* Admin can sync a panel.
* Admin can see panel status.
* Admin can delete a panel.
* Panel password is never shown after save.

---

# 53. Panel Status Values

Use:

```txt
unknown
online
offline
auth_error
degraded
syncing
error
```

Rules:

* `online`: login and basic API call succeeded.
* `offline`: network unavailable.
* `auth_error`: invalid credentials or expired auth not recoverable.
* `degraded`: panel reachable but some APIs fail.
* `syncing`: sync job currently running.
* `error`: unexpected error.

Tasks:

* [x] Add constants.
* [x] Add UI badges.
* [x] Add status update logic.
* [ ] Add status filters.

---

# 54. Inbound Sync

Inbound sync reads remote inbound data from 3x-ui and stores a local mirror in SQLite.

Tasks:

* [x] Call XUI list inbounds.
* [x] Upsert inbound records.
* [x] Store raw JSON.
* [x] Mark removed remote inbounds as disabled or stale.
* [x] Update `last_synced_at`.
* [x] Update panel `last_sync_at`.
* [x] Create sync job record.
* [x] Add audit log.

Important:

Do not delete local inbound records immediately if missing remotely. Mark them as stale first.

Add optional column if needed:

```sql
ALTER TABLE inbounds ADD COLUMN stale BOOLEAN NOT NULL DEFAULT 0;
```

Acceptance criteria:

* Admin can sync inbounds from a panel.
* Local inbounds reflect remote panel.
* Missing remote inbounds do not instantly destroy relationships.

---

# 55. Inbound Routes

Routes:

```txt
GET /admin/inbounds
GET /admin/panels/:id/inbounds
GET /admin/inbounds/:id
POST /admin/inbounds/:id/refresh
```

Tasks:

* [x] Show all inbounds.
* [x] Filter by panel.
* [x] Filter by protocol.
* [x] Filter by network.
* [x] Filter by security.
* [x] Show remote inbound ID.
* [x] Show port.
* [x] Show status.
* [x] Show stale badge.
* [x] Show raw JSON in collapsible view.
* [x] Add refresh button.

Acceptance criteria:

* Admin can view all synced inbounds.
* Admin can inspect raw inbound data.
* Admin can filter large inbound lists.

---

# 56. Inbound UI Requirements

Inbound table columns:

```txt
Panel
Remote ID
Remark
Protocol
Port
Network
Security
Enabled
Stale
Last Synced
Actions
```

HTMX features:

* Search without full reload.
* Filter by panel.
* Filter by protocol.
* Refresh one inbound or panel.

---

# 57. Client Service

Create:

```txt
internal/services/client/
```

Responsibilities:

* Create central hub clients.
* Update clients.
* Disable clients.
* Enable clients.
* Delete clients.
* Manage expiry.
* Manage traffic limits.
* Manage subscription token.
* Manage client password.
* Aggregate usage.
* Calculate status.

Tasks:

* [ ] Create client service.
* [ ] Create client repository.
* [ ] Add create client.
* [ ] Add update client.
* [ ] Add delete client.
* [x] Add enable/disable.
* [ ] Add password change.
* [ ] Add password reset.
* [ ] Add expiry calculation.
* [ ] Add traffic calculation.
* [ ] Add subscription token generation.
* [ ] Add audit logs.

---

# 58. Client Routes — Admin

Routes:

```txt
GET  /admin/clients
GET  /admin/clients/new
POST /admin/clients
GET  /admin/clients/:id
GET  /admin/clients/:id/edit
POST /admin/clients/:id
POST /admin/clients/:id/delete
POST /admin/clients/:id/disable
POST /admin/clients/:id/enable
POST /admin/clients/:id/reset-password
POST /admin/clients/:id/regenerate-sub-token
```

Tasks:

* [ ] List clients.
* [ ] Search clients.
* [ ] Filter by status.
* [ ] Filter by expired.
* [ ] Create client.
* [ ] Edit client.
* [ ] Disable client.
* [ ] Enable client.
* [ ] Delete client.
* [ ] Reset client password.
* [ ] Regenerate subscription token.
* [ ] Show client detail.

Acceptance criteria:

* Admin can fully manage central clients.
* Client username is unique.
* Subscription token is unique.
* Disabled clients lose access to active configs.

---

# 59. Client Status Values

Use:

```txt
active
disabled
expired
limited
deleted
```

Rules:

* `active`: enabled and not expired.
* `disabled`: manually disabled.
* `expired`: expiry time passed.
* `limited`: traffic limit reached.
* `deleted`: soft-deleted if soft delete is implemented.

Tasks:

* [ ] Add status service.
* [ ] Add computed status.
* [ ] Add UI badge.
* [ ] Add API status mapping.

---

# 60. Soft Delete Strategy

Recommended:

Do not physically delete clients by default.

Add later if needed:

```sql
ALTER TABLE clients ADD COLUMN deleted_at DATETIME;
```

Tasks:

* [ ] Design soft delete.
* [ ] For MVP, hard delete can be disabled or owner-only.
* [ ] Prevent accidental removal of remote clients unless explicitly confirmed.

---

# 61. Client Attachment Overview

A central hub client can be attached to many remote inbounds across many panels.

Example:

```txt
Client: ali

Attached to:
- Germany Panel / VLESS REALITY 443
- Finland Panel / Trojan TLS 443
- Iran Panel / Shadowsocks 8443
```

Each attachment represents:

```txt
client_id + panel_id + inbound_id + remote_client_id
```

This is the core orchestration model.

---

# 62. Attachment Service

Create:

```txt
internal/services/attachment/
```

Responsibilities:

* Attach client to inbound.
* Create remote client in 3x-ui.
* Store mapping.
* Generate raw config.
* Detach client.
* Disable attachment.
* Enable attachment.
* Sync traffic.
* Handle partial failures.

Tasks:

* [x] Create attachment service.
* [x] Add attach client to one inbound.
* [x] Add attach client to multiple inbounds.
* [x] Add detach.
* [ ] Add enable/disable.
* [ ] Add traffic sync.
* [x] Add config refresh.
* [x] Add audit logs.
* [x] Add partial failure result type.

---

# 63. Attachment Routes

Routes:

```txt
GET  /admin/clients/:id/attachments
POST /admin/clients/:id/attachments
POST /admin/clients/:id/attachments/:attachment_id/delete
POST /admin/clients/:id/attachments/:attachment_id/sync
POST /admin/clients/:id/attachments/:attachment_id/disable
POST /admin/clients/:id/attachments/:attachment_id/enable
POST /admin/clients/:id/attachments/:attachment_id/reset-traffic
```

Tasks:

* [x] Show attachments on client detail page.
* [x] Show available panels.
* [x] Show available inbounds grouped by panel.
* [x] Allow multi-select attach.
* [x] Show partial success/failure results.
* [x] Add detach action.
* [x] Add sync action.
* [x] Add enable/disable action.
* [ ] Add reset traffic action.

Acceptance criteria:

* Admin can attach one client to several inbounds.
* Failure in one panel does not roll back successful panels unless configured.
* UI clearly shows which attachments succeeded and failed.

---

# 64. Remote Client Identity Strategy

When creating remote clients in 3x-ui, generate a stable remote identifier.

Recommended:

```txt
hub_<client_username>_<client_id>_<inbound_id>
```

Example:

```txt
hub_ali_12_3
```

Rules:

* Must be deterministic enough for debugging.
* Must be unique per inbound.
* Must not expose sensitive data.
* Must be stored in `client_attachments.remote_email`.

Tasks:

* [ ] Add remote identity generator.
* [ ] Add tests.
* [ ] Prevent duplicates.

---

# 65. Remote Client Creation Strategy

When admin attaches a client:

1. Load client.
2. Load inbound.
3. Load panel.
4. Decrypt panel password.
5. Create XUI client.
6. Call remote panel API.
7. Store remote client ID/email.
8. Generate raw config.
9. Store attachment.
10. Invalidate subscription cache.

Tasks:

* [ ] Implement attach flow.
* [ ] Add transaction around local DB writes.
* [ ] Handle remote success + local DB failure.
* [ ] Handle local success + remote failure.
* [ ] Add reconciliation job for inconsistent states.

---

# 66. Partial Failure Result

Create struct:

```go
type AttachmentBatchResult struct {
    ClientID int64
    Results  []AttachmentResult
}

type AttachmentResult struct {
    PanelID   int64
    InboundID int64
    Success   bool
    Error     string
}
```

Tasks:

* [ ] Return partial result to UI.
* [ ] Show success/failure per inbound.
* [ ] Log failures.
* [ ] Add retry action.

---

# 67. Config Generation

hub should generate final config links from inbound data and remote client data.

Supported initially:

```txt
vless
vmess
trojan
shadowsocks
```

Tasks:

* [ ] Parse inbound raw JSON.
* [ ] Extract protocol-specific settings.
* [ ] Generate VLESS config.
* [ ] Generate VMess config.
* [ ] Generate Trojan config.
* [ ] Generate Shadowsocks config.
* [ ] Add tests with sample inbound JSON.
* [x] Store generated config in `client_attachments.raw_config`.

If config generation is too complex for MVP:

* Store raw config if returned by 3x-ui.
* Otherwise add placeholder and mark attachment as needing config refresh.

---

# 68. Subscription Service

Create:

```txt
internal/services/subscription/
```

Responsibilities:

* Build subscription output.
* Validate token.
* Check client status.
* Collect active attachments.
* Format output.
* Cache output in Redis.
* Invalidate output after changes.

Routes:

```txt
GET /sub/:token
GET /sub/:token/raw
GET /sub/:token/base64
GET /sub/:token/clash
GET /sub/:token/singbox
```

---

# 69. Subscription Rules

Rules:

* Disabled clients get HTTP 403.
* Deleted clients get HTTP 404.
* Expired clients return no active configs unless configured otherwise.
* Traffic-limited clients return no active configs unless configured otherwise.
* Disabled attachments are excluded.
* Stale inbounds are excluded unless configured otherwise.
* Output must be deterministic.
* Empty subscription should return valid empty body.

---

# 70. Raw Subscription Format

Route:

```txt
GET /sub/:token/raw
```

Output:

```txt
vless://...
trojan://...
ss://...
```

One config per line.

Tasks:

* [x] Build raw output.
* [x] Add content type `text/plain`.
* [x] Add tests.

---

# 71. Base64 Subscription Format

Route:

```txt
GET /sub/:token/base64
```

Output:

```txt
base64(raw_subscription)
```

Tasks:

* [x] Build base64 output.
* [x] Add content type `text/plain`.
* [x] Add tests.

---

# 72. Default Subscription Route

Route:

```txt
GET /sub/:token
```

Default behavior:

```txt
Return base64 output
```

Tasks:

* [x] Make default format configurable later.
* [x] For MVP use base64.
* [x] Add tests.

---

# 73. Clash Subscription Placeholder

Route:

```txt
GET /sub/:token/clash
```

MVP:

```txt
Return 501 Not Implemented
```

Later:

```txt
Return Clash YAML.
```

Tasks:

* [ ] Add route.
* [ ] Add placeholder.
* [ ] Add roadmap note.

---

# 74. Sing-box Subscription Placeholder

Route:

```txt
GET /sub/:token/singbox
```

MVP:

```txt
Return 501 Not Implemented
```

Later:

```txt
Return Sing-box JSON.
```

Tasks:

* [ ] Add route.
* [ ] Add placeholder.
* [ ] Add roadmap note.

---

# 75. Subscription Cache

Redis keys:

```txt
hub:sub:{token}:raw
hub:sub:{token}:base64
```

Tasks:

* [x] Cache raw output.
* [x] Cache base64 output.
* [ ] Cache TTL configurable.
* [ ] Invalidate on client update.
* [x] Invalidate on attachment update.
* [ ] Invalidate on inbound sync.
* [ ] Invalidate on traffic limit reached.

---

# 76. Traffic Sync

Traffic can be synced from remote panels.

Tasks:

* [x] Add traffic sync per panel.
* [x] Add traffic sync per attachment.
* [x] Update `client_attachments.upload_bytes`.
* [x] Update `client_attachments.download_bytes`.
* [x] Create traffic snapshot.
* [x] Update client usage aggregation.
* [x] Add sync job record.
* [x] Add audit log for manual sync.

Routes:

```txt
POST /admin/clients/:id/sync-traffic
POST /admin/panels/:id/sync-traffic
POST /admin/sync/traffic
```

---

# 77. Sync Jobs

Create background job system.

Initial simple implementation:

```txt
In-process goroutines + Redis locks
```

Later:

```txt
Persistent queue
```

Job types:

```txt
panel_test
panel_sync
inbound_sync
client_attach
traffic_sync
subscription_cache_refresh
```

Tasks:

* [x] Add job service.
* [x] Add sync job repository.
* [x] Add Redis lock.
* [x] Prevent concurrent sync for same panel.
* [x] Store job status.
* [x] Show job history in UI.

Redis lock key:

```txt
hub:lock:sync:panel:{panel_id}
```

---

# 78. Sync Job Status Values

Use:

```txt
queued
running
success
failed
cancelled
```

Tasks:

* [x] Add constants.
* [x] Add UI badges.
* [x] Add dashboard widget.
* [x] Add cleanup strategy.

---

# 79. Sync Jobs Routes

Routes:

```txt
GET /admin/sync-jobs
GET /admin/sync-jobs/:id
POST /admin/sync-jobs/:id/retry
```

Tasks:

* [x] List jobs.
* [x] Filter by status.
* [x] Filter by panel.
* [x] Show job detail.
* [x] Retry failed job when safe.

---

# 80. Panel Health Monitoring

Health check each panel periodically.

For MVP:

```txt
Manual health check button
```

Later:

```txt
Scheduled health checks
```

Tasks:

* [x] Add health check service.
* [x] Ping panel.
* [x] Test login/session.
* [x] Update panel status.
* [x] Store last error.
* [x] Show in dashboard.

---

# 81. Admin Subscriptions Page

Routes:

```txt
GET /admin/subscriptions
GET /admin/subscriptions/:client_id
POST /admin/subscriptions/:client_id/regenerate
POST /admin/subscriptions/:client_id/invalidate-cache
```

Tasks:

* [x] List clients with subscription status.
* [x] Show subscription URL.
* [x] Show active config count.
* [x] Show last cache time.
* [x] Regenerate token.
* [x] Invalidate cache.
* [x] Copy buttons.

---

# 82. Client Detail Page Requirements

Client detail page should show:

```txt
- Client identity
- Status
- Expiry
- Usage
- Traffic limit
- Subscription link
- Attached inbounds
- Configs
- Audit history
- Actions
```

Actions:

```txt
Edit
Disable
Enable
Reset password
Regenerate subscription token
Attach inbound
Detach inbound
Sync traffic
```

---

# 83. API Support for Admin Later

Not MVP, but design route namespace:

```txt
/api/v1/admin
```

Future:

```txt
POST /api/v1/admin/auth/login
GET  /api/v1/admin/panels
GET  /api/v1/admin/clients
POST /api/v1/admin/clients
```

Do not implement until client API is stable.

---

# 84. Acceptance Criteria for Part 3

Part 3 is complete when:

* [x] hub can connect to a 3x-ui panel.
* [x] hub can persist and reuse panel sessions.
* [x] hub can detect expired sessions.
* [x] hub can sync inbounds.
* [x] hub can display all inbounds.
* [x] Admin can create central clients.
* [x] Admin can attach clients to remote inbounds.
* [x] hub can create remote clients through 3x-ui API.
* [x] hub stores remote mappings.
* [x] hub can generate raw subscription output.
* [x] hub can generate base64 subscription output.
* [x] Client subscription link works.
* [x] Traffic usage can be synced manually.
* [x] Sync jobs are stored and visible.
* [x] Partial failures are handled clearly.

# TODO.md — hub

# PART 4 — Backup/Restore, Monitoring, Webhooks, Notifications, Background Jobs

---

# 85. Backup / Restore Overview

hub uses SQLite as the main database, so backup and restore must be first-class features.

Admins must be able to:

```txt
- Export the current SQLite database
- Download backup files
- Import a previous backup
- Validate backup metadata
- Automatically create a pre-import safety backup
- Delete old backup files
- View backup history
```

Backup and restore must be available from:

```txt
Admin Dashboard → Backups
```

---

# 86. Backup Routes

Routes:

```txt
GET  /admin/backups
GET  /admin/backups/export
POST /admin/backups/import
GET  /admin/backups/download/:filename
POST /admin/backups/delete/:filename
```

Permissions:

```txt
owner: full access
admin: export only if allowed
support: no access
readonly: no access
```

Tasks:

* [x] Add backup page.
* [x] Add export action.
* [x] Add import form.
* [x] Add backup list.
* [x] Add backup download.
* [x] Add backup delete.
* [x] Add permission checks.
* [x] Add audit logs.

---

# 87. Backup File Format

Backup filename format:

```txt
hub-backup-YYYY-MM-DD-HH-MM-SS.zip
```

Zip contents:

```txt
hub.db
metadata.json
```

Example:

```txt
hub-backup-2026-06-03-19-30-00.zip
```

---

# 88. Backup Metadata

`metadata.json`:

```json
{
  "app": "hub",
  "version": "0.1.0",
  "database": "sqlite",
  "schema_version": 1,
  "created_at": "2026-06-03T19:30:00Z"
}
```

Tasks:

* [x] Generate metadata on export.
* [x] Validate metadata on import.
* [x] Reject backups where `app != "hub"`.
* [x] Reject unsupported schema versions.
* [x] Reject malformed metadata.

---

# 89. SQLite Export Requirements

Export must be safe.

Do not simply copy the database file while it may be in WAL mode unless checkpointing or online backup is used.

Preferred strategy:

```txt
Use SQLite online backup API.
```

Acceptable strategy:

```txt
Pause writes
Run WAL checkpoint
Copy database safely
Resume writes
```

Tasks:

* [ ] Add export lock.
* [ ] Pause sync jobs during export if needed.
* [ ] Run WAL checkpoint.
* [ ] Create temporary backup DB.
* [ ] Zip backup DB with metadata.
* [ ] Save zip into backup directory.
* [ ] Return download response.
* [ ] Resume jobs.
* [ ] Add audit log.

Acceptance criteria:

* Export works while app is running.
* Exported DB can be imported later.
* Backup zip contains valid DB and metadata.

---

# 90. SQLite Import Requirements

Import is dangerous and must be defensive.

Import flow:

```txt
1. Receive uploaded .zip file.
2. Validate upload size.
3. Extract only into temporary directory.
4. Prevent path traversal.
5. Validate metadata.json.
6. Validate hub.db exists.
7. Validate SQLite file.
8. Validate schema version.
9. Create automatic pre-import backup.
10. Pause sync jobs.
11. Close current DB connection safely if needed.
12. Replace DB file.
13. Reopen DB.
14. Run migrations.
15. Resume jobs.
16. Show success.
```

Tasks:

* [x] Add import service.
* [x] Add zip validation.
* [x] Add path traversal protection.
* [x] Add SQLite validation.
* [x] Add schema validation.
* [x] Add automatic pre-import backup.
* [ ] Add restore rollback on failure.
* [ ] Add audit log.
* [x] Add tests.

Acceptance criteria:

* Bad zip files are rejected.
* Wrong app metadata is rejected.
* Unsupported schema is rejected.
* Failed import does not destroy current DB.
* Current DB is automatically backed up before import.

---

# 91. Backup Retention

Settings:

```txt
BACKUP_RETENTION_COUNT=20
BACKUP_RETENTION_DAYS=30
```

Tasks:

* [ ] Add backup retention config.
* [ ] Add cleanup old backups job.
* [ ] Never delete pre-import backup automatically in MVP.
* [ ] Show backup size.
* [ ] Show backup creation time.

---

# 92. Automatic Backups

Roadmap feature.

Scheduled backups:

```txt
daily
weekly
monthly
```

Tasks:

* [ ] Add automatic backup setting.
* [ ] Add scheduled backup job.
* [ ] Add retention policy.
* [ ] Add notification on failure.

Not required for MVP.

---

# 93. Backup UI

Backup page must show:

```txt
- Export button
- Import upload form
- Existing backups table
- Filename
- Size
- Created time
- Download action
- Delete action
```

Import form warnings:

```txt
Importing a backup will replace the current database.
A safety backup will be created automatically before import.
```

Tasks:

* [ ] Add confirmation modal for import.
* [ ] Add confirmation modal for delete.
* [ ] Add success/error alerts.
* [ ] Add HTMX upload progress if possible.

---

# 94. Monitoring Overview

hub must expose enough monitoring to understand:

```txt
- App health
- SQLite status
- Redis status
- Panel health
- Sync jobs
- API usage
- Subscription requests
- Backup status
```

---

# 95. Health Check Routes

Routes:

```txt
GET /health
GET /health/live
GET /health/ready
```

Behavior:

```txt
/health/live:
  App process is alive.

/health/ready:
  SQLite is reachable.
  Redis is reachable if required.
  Migrations are applied.
```

Tasks:

* [ ] Add health handlers.
* [ ] Add SQLite ping.
* [ ] Add Redis ping.
* [ ] Add migration status check.
* [ ] Return JSON.

Example response:

```json
{
  "status": "ok",
  "sqlite": "ok",
  "redis": "ok",
  "version": "0.1.0"
}
```

---

# 96. Metrics

Expose optional metrics endpoint:

```txt
GET /metrics
```

For Prometheus-compatible metrics.

Metrics:

```txt
hub_http_requests_total
hub_http_request_duration_seconds
hub_panels_total
hub_panels_online
hub_panels_offline
hub_clients_total
hub_clients_active
hub_clients_disabled
hub_subscriptions_requests_total
hub_sync_jobs_total
hub_sync_jobs_failed_total
hub_backup_exports_total
hub_backup_imports_total
```

Tasks:

* [ ] Add metrics middleware.
* [ ] Add Prometheus endpoint.
* [ ] Add config flag to enable/disable metrics.
* [ ] Protect metrics endpoint optionally.

---

# 97. Panel Health Monitoring

Panel health states:

```txt
unknown
online
offline
auth_error
degraded
error
```

Tasks:

* [ ] Add manual health check.
* [ ] Add scheduled health check later.
* [ ] Update panel status.
* [ ] Store last error.
* [ ] Store last checked time.
* [ ] Show on dashboard.
* [ ] Show on panel list.

Health check should verify:

```txt
- Network reachability
- Login/session validity
- Basic API availability
```

---

# 98. Background Jobs Overview

hub needs background jobs for:

```txt
- Panel sync
- Inbound sync
- Traffic sync
- Subscription cache refresh
- Backup cleanup
- Health checks
- Webhook delivery retries
- Notification delivery
```

Initial implementation:

```txt
In-process worker pool + Redis locks
```

Later implementation:

```txt
Persistent queue backed by SQLite or Redis Streams
```

---

# 99. Job Types

Supported job types:

```txt
panel_test
panel_sync
inbound_sync
traffic_sync
client_attach
client_detach
subscription_cache_refresh
backup_export
backup_import
backup_cleanup
panel_health_check
webhook_delivery
notification_delivery
```

---

# 100. Job Statuses

Use:

```txt
queued
running
success
failed
cancelled
retrying
```

Tasks:

* [ ] Create job constants.
* [ ] Create job runner.
* [ ] Create job repository.
* [ ] Create worker pool.
* [ ] Store started_at.
* [ ] Store finished_at.
* [ ] Store error message.
* [ ] Store retry count.
* [ ] Show jobs in admin UI.

---

# 101. Job Locking

Use Redis locks to prevent dangerous concurrent work.

Examples:

```txt
hub:lock:sync:panel:{panel_id}
hub:lock:backup:export
hub:lock:backup:import
hub:lock:client:{client_id}:attach
```

Tasks:

* [x] Implement Redis lock helper.
* [x] Add lock TTL.
* [x] Add lock owner token.
* [x] Only lock owner can release lock.
* [x] Fallback behavior if Redis unavailable.
* [ ] Add tests.

---

# 102. Sync Job UI

Routes:

```txt
GET /admin/sync-jobs
GET /admin/sync-jobs/:id
POST /admin/sync-jobs/:id/retry
POST /admin/sync-jobs/:id/cancel
```

UI must show:

```txt
- Job ID
- Type
- Status
- Panel
- Started at
- Finished at
- Duration
- Message
- Retry count
```

Tasks:

* [ ] Add job list page.
* [ ] Add job detail page.
* [ ] Add retry action.
* [ ] Add cancel action placeholder.
* [ ] Add filters.

---

# 103. Webhooks Overview

hub should support outgoing webhooks for automation.

Events:

```txt
client.created
client.updated
client.disabled
client.enabled
client.deleted
client.expired
client.traffic_limited

panel.created
panel.updated
panel.deleted
panel.online
panel.offline
panel.auth_error

inbound.synced

subscription.token_regenerated

backup.exported
backup.imported

sync.failed
```

Not required for MVP, but design the system now.

---

# 104. Webhook Tables

Add later migration:

```sql
CREATE TABLE webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT 1,
    events TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

```sql
CREATE TABLE webhook_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    webhook_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL,
    response_status INTEGER,
    response_body TEXT,
    error_message TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_retry_at DATETIME,
    created_at DATETIME NOT NULL,
    delivered_at DATETIME,
    FOREIGN KEY(webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);
```

---

# 105. Webhook Security

Requirements:

* Sign webhook payloads with HMAC-SHA256.
* Include timestamp.
* Include event ID.
* Retry failed deliveries.
* Do not include secrets.
* Allow webhook disable.

Headers:

```txt
X-Ahub-Event
X-Ahub-Delivery
X-Ahub-Timestamp
X-Ahub-Signature
```

Signature format:

```txt
sha256=<hex_signature>
```

---

# 106. Webhook Routes

Routes:

```txt
GET  /admin/webhooks
GET  /admin/webhooks/new
POST /admin/webhooks
GET  /admin/webhooks/:id
GET  /admin/webhooks/:id/edit
POST /admin/webhooks/:id
POST /admin/webhooks/:id/delete
POST /admin/webhooks/:id/test
GET  /admin/webhooks/:id/deliveries
```

Tasks:

* [ ] Add webhook management later.
* [ ] Add delivery retry job later.
* [ ] Add webhook test action later.

---

# 107. Notification System Overview

Notifications are for admins.

Channels:

```txt
- Admin dashboard alerts
- Email later
- Telegram later
```

Events worth notifying:

```txt
- Panel offline
- Panel auth error
- Sync failed
- Backup import completed
- Backup import failed
- Backup export failed
- Client traffic limit reached
- Client expired
```

---

# 108. Notification Tables

Add later:

```sql
CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    severity TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    read_at DATETIME,
    created_at DATETIME NOT NULL
);
```

Severity:

```txt
info
success
warning
danger
```

---

# 109. Notification UI

Routes:

```txt
GET  /admin/notifications
POST /admin/notifications/:id/read
POST /admin/notifications/read-all
```

Tasks:

* [ ] Add notification bell.
* [ ] Add unread count.
* [ ] Add notification page.
* [ ] Add mark as read.
* [ ] Add mark all as read.

Roadmap only unless MVP is complete.

---

# 110. Telegram Notifications

Roadmap feature.

Env:

```env
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
```

Tasks:

* [ ] Add Telegram notifier.
* [ ] Add test message action.
* [ ] Add event mapping.
* [ ] Add retry on failure.

---

# 111. Email Notifications

Roadmap feature.

Env:

```env
SMTP_HOST=
SMTP_PORT=
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=
```

Tasks:

* [ ] Add SMTP notifier.
* [ ] Add templates.
* [ ] Add test email action.

---

# 112. Audit Log Viewer

Audit logs already exist in schema.

Routes:

```txt
GET /admin/audit-logs
GET /admin/audit-logs/:id
```

Filters:

```txt
actor_type
actor_id
action
target_type
target_id
date_from
date_to
```

Tasks:

* [ ] Add audit logs list page.
* [ ] Add filters.
* [ ] Add metadata JSON viewer.
* [ ] Add export audit logs later.
* [ ] Add permission check.

Audit log examples:

```txt
admin.login
admin.logout
panel.created
panel.updated
panel.deleted
panel.tested
panel.synced
client.created
client.updated
client.disabled
client.enabled
client.deleted
client.password_reset
client.subscription_regenerated
attachment.created
attachment.deleted
backup.exported
backup.imported
api.client_login
api.client_refresh
```

---

# 113. System Events

Create internal event bus interface.

```go
type Event struct {
    Type string
    ActorType string
    ActorID int64
    TargetType string
    TargetID int64
    Payload map[string]any
    CreatedAt time.Time
}
```

Consumers:

```txt
- Audit logger
- Notification system
- Webhook dispatcher
- Metrics collector
```

Tasks:

* [ ] Add event dispatcher.
* [ ] Emit events from services.
* [ ] Subscribe audit logger.
* [ ] Subscribe notification service later.
* [ ] Subscribe webhook service later.

---

# 114. Real-Time Dashboard Updates

Use HTMX polling first.

Examples:

```html
<div hx-get="/admin/partials/panel-health"
     hx-trigger="every 30s"
     hx-swap="outerHTML">
</div>
```

Later:

```txt
Server-Sent Events
```

Tasks:

* [ ] Add HTMX polling for panel status.
* [ ] Add HTMX polling for sync jobs.
* [ ] Add SSE roadmap.

---

# 115. Reverse Proxy Awareness

hub may run behind:

```txt
- Nginx
- Caddy
- Traefik
- Cloudflare Tunnel
```

Tasks:

* [ ] Respect `X-Forwarded-Proto`.
* [ ] Respect `X-Forwarded-Host`.
* [ ] Respect `X-Real-IP`.
* [ ] Add trusted proxy config.
* [ ] Generate subscription URLs using `APP_BASE_URL`.
* [ ] Never blindly trust forwarded headers unless configured.

Env:

```env
TRUST_PROXY=false
TRUSTED_PROXIES=
```

---

# 116. API Usage Logging

For client API:

Log:

```txt
- Login success/failure
- Refresh
- Logout
- Subscription URL fetch
- Config fetch
- Usage fetch
```

Do not log:

```txt
- JWT token
- Refresh token
- Full config links if they contain secrets
- Subscription token
```

Tasks:

* [ ] Add safe API logging middleware.
* [ ] Add request ID.
* [ ] Add correlation ID.

---

# 117. Request ID Middleware

Every request gets:

```txt
X-Request-ID
```

Tasks:

* [ ] Generate request ID if missing.
* [ ] Include in logs.
* [ ] Include in API error responses optionally.
* [ ] Forward to webhook delivery logs.

---

# 118. Error Pages

Create error pages:

```txt
400
401
403
404
429
500
```

Tasks:

* [ ] Add public error layout.
* [ ] Add admin-safe error rendering.
* [ ] Add client-safe error rendering.
* [ ] For API, always return JSON.

---

# 119. Admin Activity Timeline

On client detail page, show recent audit events related to that client.

Tasks:

* [ ] Query audit logs by target.
* [ ] Show recent 20 events.
* [ ] Add “View all” link.

---

# 120. Data Cleanup Jobs

Cleanup:

```txt
- Expired refresh tokens
- Old sync jobs
- Old traffic snapshots
- Old backup files according to retention
- Old webhook deliveries
```

Tasks:

* [ ] Add cleanup service.
* [ ] Add manual cleanup action.
* [ ] Add scheduled cleanup later.

---

# 121. Manual Maintenance Page

Routes:

```txt
GET  /admin/maintenance
POST /admin/maintenance/cleanup
POST /admin/maintenance/check-db
POST /admin/maintenance/vacuum-db
```

Tasks:

* [ ] Add DB integrity check.
* [ ] Add SQLite VACUUM action.
* [ ] Add cleanup action.
* [ ] Add permission checks.
* [ ] Add warnings.

SQLite check:

```sql
PRAGMA integrity_check;
```

---

# 122. Backup and Import Tests

Required tests:

* [ ] Export creates zip.
* [ ] Zip contains hub.db.
* [ ] Zip contains metadata.json.
* [ ] Metadata is valid.
* [ ] Import rejects missing metadata.
* [ ] Import rejects wrong app.
* [ ] Import rejects path traversal.
* [ ] Import rejects invalid SQLite.
* [ ] Import creates pre-import backup.
* [ ] Failed import keeps old DB.

---

# 123. Job System Tests

Required tests:

* [x] Job creation.
* [x] Job status update.
* [x] Job failure state.
* [x] Retry count.
* [x] Redis lock acquire.
* [x] Redis lock release.
* [x] Lock cannot be released by non-owner.
* [x] Concurrent panel sync prevented.

---

# 124. Monitoring Tests

Required tests:

* [ ] `/health/live` returns ok.
* [ ] `/health/ready` checks SQLite.
* [ ] `/health/ready` reports Redis state.
* [ ] Metrics endpoint returns expected format if enabled.

---

# 125. Acceptance Criteria for Part 4

Part 4 is complete when:

* Admin can export SQLite database.
* Exported backup has valid metadata.
* Admin can import a valid backup.
* Import creates a safety backup first.
* Invalid imports are rejected safely.
* Health endpoints exist.
* Metrics endpoint exists or is clearly feature-flagged.
* Background jobs are tracked.
* Sync jobs are visible.
* Audit log viewer exists.
* Request ID middleware exists.
* Error pages exist.
* Panel health is visible.
* Redis locks protect sync and backup operations.


# PART 5 — Docker, CI/CD, Testing, Security, Documentation, Roadmap, Final Implementation Order

---

# 126. Docker Requirements

hub must be easy to run with Docker Compose.

Required services:

```txt
hub
redis
```

SQLite database must be persisted through a bind mount or named volume.

Backups must be persisted through a bind mount or named volume.

---

# 127. Dockerfile

Create:

```txt
Dockerfile
```

Requirements:

* Multi-stage build.
* Small final image.
* Non-root user.
* Static binary if possible.
* Include templates and static assets.
* Expose port `8080`.

Example structure:

```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /src

RUN apk add --no-cache build-base sqlite-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /out/hub ./cmd/hub

FROM alpine:latest

RUN apk add --no-cache ca-certificates sqlite-libs tzdata

WORKDIR /app

RUN adduser -D -H hub

COPY --from=builder /out/hub /app/hub
COPY web /app/web

RUN mkdir -p /app/data /app/backups && chown -R hub:hub /app

USER hub

EXPOSE 8080

CMD ["/app/hub"]
```

Tasks:

* [ ] Add Dockerfile.
* [ ] Ensure templates are available inside container.
* [ ] Ensure static files are available inside container.
* [ ] Ensure app can write to `/app/data`.
* [ ] Ensure app can write to `/app/backups`.
* [ ] Run as non-root.
* [ ] Add container healthcheck.

---

# 128. docker-compose.yml

Create:

```txt
docker-compose.yml
```

Example:

```yaml
services:
  hub:
    build: .
    container_name: hub
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - APP_NAME=hub
      - APP_ENV=production
      - APP_ADDR=0.0.0.0:8080
      - APP_BASE_URL=http://localhost:8080
      - DATABASE_PATH=/app/data/hub.db
      - REDIS_ADDR=redis:6379
      - HUB_SECRET_KEY=change-this-secret
      - BACKUP_DIR=/app/backups
      - TRUST_PROXY=false
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
    depends_on:
      redis:
        condition: service_started
    healthcheck:
      test: ["CMD", "/app/hub", "healthcheck"]
      interval: 30s
      timeout: 5s
      retries: 3

  redis:
    image: redis:7-alpine
    container_name: hub-redis
    restart: unless-stopped
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

Tasks:

* [ ] Add Compose file.
* [ ] Add persistent data volume.
* [ ] Add persistent backups volume.
* [ ] Add Redis volume.
* [ ] Add production env example.
* [ ] Document first-run admin setup.

---

# 129. Reverse Proxy Examples

Add docs for:

```txt
Nginx
Caddy
Traefik
Cloudflare Tunnel
```

Important:

* `APP_BASE_URL` must match public URL.
* Secure cookies require HTTPS.
* Trust forwarded headers only if `TRUST_PROXY=true`.

Tasks:

* [ ] Add `docs/reverse-proxy.md`.
* [ ] Add Nginx example.
* [ ] Add Caddy example.
* [ ] Add Cloudflare Tunnel note.
* [ ] Add HTTPS warning.

---

# 130. Makefile

Create:

```txt
Makefile
```

Targets:

```makefile
run:
	go run ./cmd/hub

test:
	go test ./...

build:
	go build -o bin/hub ./cmd/hub

fmt:
	go fmt ./...

lint:
	golangci-lint run

docker-build:
	docker build -t hub .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f hub

migrate:
	go run ./cmd/hub migrate

dev:
	go run ./cmd/hub
```

Tasks:

* [ ] Add Makefile.
* [ ] Ensure all targets work.
* [ ] Add help target later.

---

# 131. CLI Commands

The binary should support optional commands:

```txt
hub serve
hub migrate
hub create-admin
hub healthcheck
hub backup export
hub backup import <file>
```

Tasks:

* [ ] Add command parser.
* [ ] Default command should be `serve`.
* [ ] Add `healthcheck` for Docker.
* [ ] Add `create-admin`.
* [ ] Add migration command.
* [ ] Add backup commands later.

---

# 132. Testing Strategy

Testing layers:

```txt
Unit Tests
Repository Tests
Service Tests
Handler Tests
Connector Tests
Integration Tests
```

Use:

```txt
go test ./...
```

Test database:

```txt
SQLite temporary file or in-memory DB
```

Redis tests:

```txt
Use fake Redis where possible.
Use real Redis integration tests optionally.
```

---

# 133. Required Unit Tests

Must test:

```txt
Config loader
Password hashing
Password verification
Encryption
Decryption
JWT signing
JWT validation
Refresh token hashing
Subscription URL generation
Remaining time calculation
Traffic calculation
API error helpers
```

Tasks:

* [ ] Add config tests.
* [ ] Add password tests.
* [ ] Add crypto tests.
* [ ] Add JWT tests.
* [ ] Add subscription tests.
* [ ] Add usage tests.

---

# 134. Required Repository Tests

Test repositories:

```txt
AdminRepository
ClientRepository
PanelRepository
InboundRepository
AttachmentRepository
RefreshTokenRepository
AuditRepository
SyncJobRepository
BackupRepository if implemented
```

Tasks:

* [ ] Create test DB helper.
* [ ] Run migrations in tests.
* [ ] Test create/read/update/delete.
* [ ] Test unique constraints.
* [ ] Test foreign key behavior.
* [ ] Test transactions.

---

# 135. Required Service Tests

Test services:

```txt
AuthService
ClientService
PanelService
AttachmentService
SubscriptionService
BackupService
SyncJobService
UsageService
```

Tasks:

* [ ] Test admin login.
* [ ] Test client login.
* [ ] Test disabled client login rejection.
* [ ] Test expired client status.
* [ ] Test create client.
* [ ] Test attach client partial failure.
* [ ] Test subscription excludes disabled attachments.
* [ ] Test traffic aggregation.
* [ ] Test backup export.
* [ ] Test backup import validation.

---

# 136. XUI Connector Tests

Use `httptest.Server`.

Simulate:

```txt
Successful login
Failed login
Expired session
List inbounds
Add client success
Add client failure
Traffic response
Malformed response
Timeout
```

Tasks:

* [ ] Mock login endpoint.
* [ ] Mock inbound endpoint.
* [ ] Mock add client endpoint.
* [ ] Mock session expiration.
* [ ] Test auto re-login.
* [ ] Test normalized errors.
* [ ] Test no secret leakage in logs.

---

# 137. Handler Tests

Test HTTP handlers:

```txt
Admin login
Client login
Client API login
Client API refresh
Client API me
Client API usage
Subscription endpoint
Backup export
Backup import validation
Health endpoints
```

Tasks:

* [ ] Add Fiber app test helper.
* [ ] Add authenticated admin request helper.
* [ ] Add authenticated client request helper.
* [ ] Add JWT helper.

---

# 138. Security Checklist

Before release:

* [ ] No plaintext passwords in DB.
* [ ] No panel passwords in logs.
* [ ] No JWTs in logs.
* [ ] No refresh tokens in logs.
* [ ] No subscription tokens in logs.
* [ ] CSRF protection on HTML forms.
* [ ] Rate limiting on login.
* [ ] Rate limiting on API auth.
* [ ] Secure cookies in production.
* [ ] HTTPOnly cookies.
* [ ] SameSite cookies.
* [ ] Input validation.
* [ ] Output escaping.
* [ ] Zip-slip protection.
* [ ] SQLite import validation.
* [ ] RBAC checks on backend.
* [ ] UI permission hiding.
* [ ] Security headers.
* [ ] Request size limits.
* [ ] Upload size limits.
* [ ] Timeout on remote panel calls.
* [ ] SSRF protection for panel base URLs where possible.

---

# 139. SSRF Protection for Panel URLs

Because admins can add remote panel URLs, basic SSRF protection should exist.

MVP:

* Require owner/admin permission to add panels.
* Validate URL scheme.
* Allow only `http` and `https`.
* Reject malformed URLs.
* Show warning for private IPs if app is public-facing.

Optional strict mode:

```env
PANEL_URL_STRICT_MODE=true
PANEL_URL_ALLOW_PRIVATE=false
```

Tasks:

* [ ] Validate panel URL.
* [ ] Reject unsupported schemes.
* [ ] Normalize base URL.
* [ ] Add timeout.
* [ ] Add strict mode later.

---

# 140. Security Headers

Apply globally:

```txt
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

CSP basic:

```txt
Content-Security-Policy: default-src 'self'
```

Because Bootstrap/HTMX may be loaded locally, avoid external CDNs in production.

Tasks:

* [ ] Vendor Bootstrap assets locally.
* [ ] Vendor HTMX locally.
* [ ] Add security headers middleware.
* [ ] Add CSP compatible with local assets.

---

# 141. Frontend Asset Policy

For production:

```txt
No CDN dependency.
```

Assets:

```txt
Bootstrap CSS
Bootstrap JS
Bootstrap Icons
HTMX
Custom CSS
Custom JS
```

Tasks:

* [ ] Place vendor assets in `web/static/vendor`.
* [ ] Reference local assets in templates.
* [ ] Add cache headers.
* [ ] Add asset versioning later.

---

# 142. UI Polish Checklist

All pages must have:

* [ ] Dark mode.
* [ ] Responsive layout.
* [ ] Mobile navigation.
* [ ] Loading states.
* [ ] Empty states.
* [ ] Error states.
* [ ] Confirmation modals.
* [ ] Copy buttons.
* [ ] Form validation messages.
* [ ] Consistent buttons.
* [ ] Consistent cards.
* [ ] Consistent badges.
* [ ] Accessible labels.
* [ ] Keyboard-friendly forms.

---

# 143. Documentation

Create:

```txt
README.md
docs/
```

Docs:

```txt
docs/installation.md
docs/docker.md
docs/reverse-proxy.md
docs/backup-restore.md
docs/3x-ui-connector.md
docs/client-api.md
docs/security.md
docs/development.md
docs/roadmap.md
```

---

# 144. README.md Requirements

README must include:

```txt
Project description
Features
Stack
Screenshots placeholder
Quick start
Docker Compose setup
Environment variables
Initial admin setup
How to add a panel
How to sync inbounds
How to create a client
How to attach client to inbounds
How subscription links work
Client API overview
Backup/restore overview
Security notes
Roadmap
License
```

---

# 145. Client API Documentation

Create:

```txt
docs/client-api.md
```

Must document:

```txt
POST /api/v1/client/auth/login
POST /api/v1/client/auth/refresh
POST /api/v1/client/auth/logout
GET  /api/v1/client/me
GET  /api/v1/client/subscription
GET  /api/v1/client/configs
GET  /api/v1/client/usage
GET  /api/v1/client/status
```

Include:

```txt
Request examples
Response examples
Error examples
JWT usage
Rate limits
```

---

# 146. Backup Documentation

Create:

```txt
docs/backup-restore.md
```

Include:

```txt
Export process
Import process
Backup file format
Metadata format
Safety backup behavior
Restore warnings
CLI restore
Docker volume notes
```

---

# 147. Development Documentation

Create:

```txt
docs/development.md
```

Include:

```txt
Local setup
Running Redis
Running tests
Project structure
Adding migrations
Adding handlers
Adding services
Adding templates
Mocking 3x-ui
```

---

# 148. GitHub Actions CI

Create:

```txt
.github/workflows/ci.yml
```

CI steps:

```txt
Checkout
Setup Go
Cache modules
go mod download
go fmt check
go test ./...
go build ./cmd/hub
Docker build
```

Example:

```yaml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - run: go mod download

      - run: go test ./...

      - run: go build -o hub ./cmd/hub

      - run: docker build -t hub .
```

Tasks:

* [ ] Add CI workflow.
* [ ] Add Redis service.
* [ ] Add test step.
* [ ] Add build step.
* [ ] Add Docker build step.

---

# 149. Release Workflow

Roadmap.

Create:

```txt
.github/workflows/release.yml
```

Release artifacts:

```txt
Linux amd64
Linux arm64
Docker image
Checksums
```

Tasks:

* [ ] Add GoReleaser later.
* [ ] Add GitHub Container Registry publish later.
* [ ] Add changelog later.

---

# 150. Versioning

Use semantic versioning:

```txt
v0.1.0
v0.2.0
v1.0.0
```

Expose version in:

```txt
Dashboard footer
/health
CLI
Docker labels
```

Tasks:

* [ ] Add version package.
* [ ] Set version at build time.
* [ ] Show version in UI.
* [ ] Show version in health endpoint.

---

# 151. MVP Definition

The MVP is complete when:

* [ ] App starts with Go + Fiber.
* [ ] SQLite works.
* [ ] Redis works.
* [ ] Migrations run.
* [ ] Admin can log in.
* [ ] Admin can add a 3x-ui panel.
* [ ] Admin can test panel connection.
* [ ] Admin can sync inbounds.
* [ ] Admin can create a client.
* [ ] Admin can attach client to at least one inbound.
* [ ] hub creates remote client in 3x-ui.
* [ ] Subscription link returns at least raw/base64 configs.
* [ ] Client can log in to portal.
* [ ] Client can see configs.
* [ ] Client can change password.
* [ ] Client API login returns JWT.
* [ ] Client API can return subscription URL.
* [ ] Client API can return usage.
* [ ] Client API can return remaining time.
* [ ] Admin can export SQLite backup.
* [ ] Admin can import SQLite backup.
* [ ] App runs with Docker Compose.

---

# 152. v0.1.0 Scope

Version `v0.1.0` should include:

```txt
Foundation
Admin auth
Client auth
Panel CRUD
XUI login
Inbound sync
Client CRUD
Client attachment
Raw subscription
Base64 subscription
Client portal
Client JWT API
Backup export/import
Docker Compose
Basic tests
```

Do not include:

```txt
Billing
Telegram notifications
Email notifications
Webhook UI
Admin API
Clash output
Sing-box output
Multi-tenant organizations
```

---

# 153. v0.2.0 Scope

Add:

```txt
Traffic charts
Scheduled traffic sync
Panel health checks
Better dashboard
Audit log viewer
Backup retention
OpenAPI docs
More tests
Improved config generation
```

---

# 154. v0.3.0 Scope

Add:

```txt
Clash subscription output
Sing-box subscription output
Webhook system
Telegram notifications
Admin 2FA
API tokens
Admin API preview
```

---

# 155. v1.0.0 Scope

Production-ready:

```txt
Stable connector
Stable API
Stable backup/restore
Stable RBAC
Stable audit system
Stable Docker deployment
Full documentation
Release artifacts
Security hardening
```

---

# 156. Final Implementation Order

Follow this order exactly.

---

## Phase 1 — Foundation

* [ ] Create repo structure.
* [ ] Initialize Go module.
* [ ] Add Fiber.
* [ ] Add config loader.
* [ ] Add logger.
* [ ] Add SQLite.
* [ ] Add Redis.
* [ ] Add migration runner.
* [ ] Add base layout.
* [ ] Add Bootstrap 5 local assets.
* [ ] Add HTMX local assets.
* [ ] Add health endpoints.

---

## Phase 2 — Database and Models

* [ ] Add migrations.
* [ ] Add models.
* [ ] Add repositories.
* [ ] Add transaction helper.
* [ ] Add tests for repositories.

---

## Phase 3 — Security Foundation

* [ ] Add password hashing.
* [ ] Add encryption service.
* [ ] Add session service.
* [ ] Add JWT service.
* [ ] Add refresh token service.
* [ ] Add CSRF protection.
* [ ] Add security headers.
* [ ] Add rate limiter.

---

## Phase 4 — Admin Auth and Dashboard

* [ ] Add initial admin bootstrap.
* [ ] Add admin login.
* [ ] Add admin logout.
* [ ] Add admin dashboard.
* [ ] Add RBAC middleware.
* [ ] Add admin user management.

---

## Phase 5 — Client Auth and Portal

* [ ] Add client login.
* [ ] Add client logout.
* [ ] Add client dashboard.
* [ ] Add client profile.
* [ ] Add client password change.

---

## Phase 6 — Client API

* [ ] Add API route group.
* [ ] Add API error format.
* [ ] Add client API login.
* [ ] Add client API refresh.
* [ ] Add client API logout.
* [ ] Add client API JWT middleware.
* [ ] Add `/me`.
* [ ] Add `/subscription`.
* [ ] Add `/configs`.
* [ ] Add `/usage`.
* [ ] Add `/status`.

---

## Phase 7 — XUI Connector

* [ ] Add connector struct.
* [ ] Add login.
* [ ] Add cookie storage.
* [ ] Add auto re-login.
* [ ] Add list inbounds.
* [ ] Add add client.
* [ ] Add update client.
* [ ] Add delete client.
* [ ] Add traffic read.
* [ ] Add tests with mock server.

---

## Phase 8 — Panels

* [ ] Add panel CRUD.
* [ ] Add test connection.
* [ ] Add clear session.
* [ ] Add sync panel.
* [ ] Add panel status UI.

---

## Phase 9 — Inbounds

* [ ] Add inbound sync.
* [ ] Add inbound list.
* [ ] Add inbound filters.
* [ ] Add raw JSON viewer.

---

## Phase 10 — Clients

* [ ] Add client CRUD.
* [ ] Add enable/disable.
* [ ] Add reset password.
* [ ] Add regenerate subscription token.
* [ ] Add usage/status calculations.

---

## Phase 11 — Attachments

* [ ] Add attachment service.
* [ ] Add attach UI.
* [ ] Add multi-inbound attach.
* [ ] Add remote client creation.
* [ ] Add detach.
* [ ] Add partial failure handling.
* [ ] Add traffic sync per attachment.

---

## Phase 12 — Subscriptions

* [ ] Add raw subscription.
* [ ] Add base64 subscription.
* [ ] Add default subscription route.
* [ ] Add cache.
* [ ] Add cache invalidation.
* [ ] Add QR code in portal.

---

## Phase 13 — Backup and Restore

* [ ] Add backup export.
* [ ] Add backup import.
* [ ] Add backup metadata.
* [ ] Add pre-import safety backup.
* [ ] Add backup UI.
* [ ] Add tests.

---

## Phase 14 — Jobs, Monitoring, Audit

* [ ] Add sync jobs.
* [ ] Add job UI.
* [ ] Add audit log writer.
* [ ] Add audit log viewer.
* [ ] Add panel health checks.
* [ ] Add metrics endpoint.

---

## Phase 15 — Docker and Documentation

* [ ] Add Dockerfile.
* [ ] Add docker-compose.yml.
* [ ] Add Makefile.
* [ ] Add README.
* [ ] Add docs.
* [ ] Add CI.

---

## Phase 16 — Polish

* [ ] Improve mobile UI.
* [ ] Add loading states.
* [ ] Add empty states.
* [ ] Add confirmation modals.
* [ ] Add better error pages.
* [ ] Add final tests.
* [ ] Run full manual test.

---

# 157. Manual Test Checklist

Before tagging `v0.1.0`:

* [ ] Start app locally.
* [ ] Start app with Docker Compose.
* [ ] Create initial admin.
* [ ] Log in as admin.
* [ ] Add 3x-ui panel.
* [ ] Test panel.
* [ ] Sync inbounds.
* [ ] Create client.
* [ ] Attach client to inbound.
* [ ] Confirm remote client exists in 3x-ui.
* [ ] Open subscription link.
* [ ] Open client portal.
* [ ] Change client password.
* [ ] Login through client API.
* [ ] Fetch subscription URL through API.
* [ ] Fetch usage through API.
* [ ] Fetch status through API.
* [ ] Export backup.
* [ ] Import backup.
* [ ] Restart app.
* [ ] Confirm data persisted.
* [ ] Confirm logs have no secrets.

---

# 158. Final Notes for Codex

When implementing this project:

* Build incrementally.
* Keep commits small.
* Prefer simple working code over over-engineering.
* Do not implement future roadmap features before MVP.
* Keep UI server-rendered.
* Use HTMX only where it improves UX.
* Keep API responses consistent.
* Keep security checks in backend, not only UI.
* Never log secrets.
* Always add tests for critical code.
* Prefer explicit code over magic.
