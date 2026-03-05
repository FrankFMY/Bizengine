# Authentication and Authorization

## 1. Data Model

```
phones ──┐
         ├── users ──── labors ──── organizations
phones ──┘
```

- **phones**: phone number records. Each has `unformat` (digits only), `format` (human-readable), `country`, linked to a `user_id`.
- **users**: account records. Fields: `email`, `password_hash` (argon2id), `full_name`, `name`, `secret`, `aat`, `is_active`. Password stored as argon2id hash.
- **organizations**: tenants (renamed from `workspaces`). Fields: `name`, `slug`, `owner_id`, `plan`, `settings` (JSONB), `inn`.
- **labors**: membership records (renamed from `workspace_members`). Links `user_id` to `organization_id` with `role`, `permissions` (JSONB array), `admin` (boolean shortcut).

All four tables have `ver` (integer, auto-incremented on UPDATE) and `upd` (timestamptz, auto-set on UPDATE) via the `auto_ver_upd` trigger.

## 2. Password Hashing

argon2id with parameters: `m=65536, t=1, p=4, keyLen=32, saltLen=16`.

Stored format:
```
$argon2id$v=19$m=65536,t=1,p=4${base64_salt}${base64_hash}
```

Verification uses `crypto/subtle.ConstantTimeCompare`.

## 3. Session Architecture

Two-layer cookie-based authentication, both stored in Redis:

| Cookie | Name | TTL | Purpose |
|--------|------|-----|---------|
| Session | `teco_session` | 72h | Identifies the user. Long-lived. |
| Seance | `teco_seance` | 10m | Proves recent activity. Auto-extended on each request (sliding window). |

Cookie parameters: `HttpOnly`, `SameSite=Lax`, `Secure` (production only), `Path=/`.

### Session data in Redis

```json
{
  "id": "uuid-string",
  "user_id": "uuid",
  "phone_id": "uuid",
  "organization_id": "uuid",
  "role": "owner",
  "email": "user@example.com",
  "full_name": "John Doe",
  "created_at": "2026-03-05T10:00:00Z"
}
```

### Flow

1. `POST /auth/login` with `{email, password}` -> verify argon2id -> create session + seance in Redis -> set `teco_session` + `teco_seance` cookies.
2. Client sends cookies automatically (`credentials: 'include'`).
3. On each authenticated request, `teco_seance` TTL is extended (sliding window).
4. When seance expires (10 min without requests) -> 401 with body `"SEANCE"` -> client calls `POST /auth/unlock` with `{password}` -> new seance cookie.
5. When session expires (72h) -> 401 with body `"SESSION"` -> full re-login required.
6. Switch organization -> `POST /auth/switch` with `{organization_id}` -> updates session in Redis.

### 401 Response Format

The auth middleware returns `Content-Type: text/plain` with body:
- `"SESSION"` — session cookie missing or expired. Full re-login required.
- `"SEANCE"` — seance cookie missing or expired. Call `/auth/unlock`.

## 4. Auth Endpoints

All under `/api/v1/auth`.

### POST /auth/register

No auth required.

```json
// Request
{"email": "user@example.com", "password": "secret123", "full_name": "John Doe", "phone_id": "uuid"}

// Response 201
{"ok": true, "data": {"user": {"id": "uuid", "email": "...", "full_name": "...", "is_active": true}}}
```

Sets `teco_session` + `teco_seance` cookies.

### POST /auth/login

No auth required.

```json
// Request
{"email": "user@example.com", "password": "secret123", "phone_id": "uuid"}

// Response 200
{
  "ok": true,
  "data": {
    "user": {"id": "uuid", "email": "...", "full_name": "..."},
    "organizations": [{"id": "uuid", "name": "...", "slug": "...", "plan": "free"}]
  }
}
```

Sets `teco_session` + `teco_seance` cookies.

### POST /auth/logout

Requires session (seance may be expired). Uses `SessionOnlyMiddleware`.

```json
// Response 200
{"ok": true}
```

Clears cookies. Deletes session + seances from Redis. Disconnects Centrifugo connections. Unsubscribes all views.

### POST /auth/unlock

Requires session (seance may be expired). Uses `SessionOnlyMiddleware`.

```json
// Request
{"password": "secret123"}

// Response 200
{"ok": true, "data": {"status": "unlocked"}}
```

Sets new `teco_seance` cookie.

### GET /auth/check

Requires full auth (session + seance).

```json
// Response 200
{
  "ok": true,
  "data": {
    "seance_id": "uuid-string",
    "session_id": "uuid-string",
    "user_id": "uuid",
    "organization_id": "uuid",
    "phone_id": "uuid",
    "admin": true
  }
}
```

The `admin` field is `true` when role is `owner` or `admin`.

### POST /auth/switch

Requires full auth (session + seance).

```json
// Request
{"organization_id": "uuid"}

// Response 200
{"ok": true, "data": {"organization_id": "uuid", "role": "owner"}}
```

## 5. /pass/temp (Dev-Only Convenience Endpoint)

`POST /api/v1/pass/temp` — no auth required. Creates or finds a user+phone+organization+labor in one transaction. Designed for rapid dev/testing setup.

```json
// Request
{
  "data": {
    "country": "RU",
    "phone": "+7 (999) 123-45-67",
    "secret": "my_password",
    "name": "Test Company",
    "inn": "1234567890"
  }
}

// Response 200
{"ok": true}
```

Sets `teco_session` + `teco_seance` cookies. The created user gets role `owner` with `admin=true` in the organization matched by INN.

Logic:
1. Normalize phone (strip non-digits, replace leading 8 with 7).
2. Find existing user by phone `unformat`, or create new user + phone.
3. Hash secret with argon2id and store in `users.secret`.
4. Find existing organization by INN, or create new one.
5. Find or create labor (membership) with role=owner, admin=true.
6. Create session + seance, set cookies.

## 6. RBAC

### 5 Roles

| Role | Description |
|------|-------------|
| `owner` | Organization creator. Full access. Only role that can delete the organization. |
| `admin` | User/settings management. Full data access. |
| `manager` | Create/edit orders, products, warehouse operations. No finance or settings. |
| `operator` | Work with assigned tasks: order assembly, stock receiving. Own operations only. |
| `viewer` | Read-only access. |

### admin Boolean Shortcut

The `labors.admin` field is a shortcut: when `admin=true`, the effective role is treated as `owner` regardless of the `role` column value. This is checked during login and organization switch:

```go
if member.Admin {
    role = "owner"
}
```

### Default Permissions by Role

| Permission | owner | admin | manager | operator | viewer |
|---|---|---|---|---|---|
| entity.create | yes | yes | yes | - | - |
| entity.read | yes | yes | yes | yes | yes |
| entity.update | yes | yes | yes | - | - |
| entity.delete | yes | yes | - | - | - |
| catalog.manage | yes | yes | yes | - | - |
| warehouse.receive | yes | yes | yes | yes | - |
| warehouse.ship | yes | yes | yes | yes | - |
| warehouse.transfer | yes | yes | yes | - | - |
| warehouse.adjust | yes | yes | yes | - | - |
| order.create | yes | yes | yes | - | - |
| order.update | yes | yes | yes | - | - |
| order.cancel | yes | yes | yes | - | - |
| order.view | yes | yes | yes | yes | yes |
| hr.manage | yes | yes | - | - | - |
| hr.view | yes | yes | - | - | - |
| finance.manage | yes | yes | - | - | - |
| finance.view | yes | yes | yes | - | - |
| logistics.manage | yes | yes | yes | - | - |
| logistics.view | yes | yes | yes | - | - |
| process.manage | yes | yes | - | - | - |
| settings.manage | yes | yes | - | - | - |
| members.manage | yes | yes | - | - | - |

### Custom Permissions

Each labor can have additional permissions in the `permissions` JSONB array. Custom permissions are checked first, before role defaults.

### Permission Check

```go
auth.HasPermission(role, customPerms, "warehouse.receive")
```

Supports wildcard: a role with `entity.*` granted would pass check for `entity.create`.

### Middleware

Two middleware functions:
- `auth.Middleware(svc)` — validates both session + seance cookies. Populates context with `user_id`, `organization_id`, `role`, `session_id`, `seance_id`.
- `auth.SessionOnlyMiddleware(svc)` — validates only session cookie. Used for `/logout` and `/unlock` where seance may be expired.

### Context Keys

```go
auth.UserIDFromCtx(ctx)         // uuid.UUID
auth.OrganizationIDFromCtx(ctx) // uuid.UUID
auth.RoleFromCtx(ctx)           // string
auth.SessionIDFromCtx(ctx)      // string
auth.SeanceIDFromCtx(ctx)       // string
```

## 7. Configuration

```
SESSION_TTL=72h
SESSION_SEANCE_TTL=10m
SESSION_COOKIE_SECURE=false   # true in production (HTTPS only)
```
