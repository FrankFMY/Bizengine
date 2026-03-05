# BizEngine

Server engine for creating digital twins of businesses. Turns any business — from a coffee shop to a logistics company — into a programmable system of entities, components, events, and automated processes.

## What it does

BizEngine provides a unified backend for managing business operations through an Entity-Component-System (ECS) architecture combined with Event Sourcing. Every business object (product, order, warehouse, employee, vehicle, point of sale) is an **Entity** with attached **Components** (typed JSONB data). Every mutation produces an **Event** in an append-only log. Business workflows are **state machines** defined in YAML. Real-time UI updates are delivered through **Arcana** (reactive data sync engine) powered by Centrifugo.

### Core capabilities

- **Universal entity model** — any business object is an Entity with flexible Components, no schema migrations needed for new object types
- **Event sourcing** — full audit trail, reactive views via Centrifugo, replay to any point in time
- **Process engine** — YAML-defined state machines for order fulfillment, stock replenishment, delivery tracking, employee onboarding
- **Arcana integration** — reactive data sync engine with 32 graph definitions, JSON Patch diffs, pagination support, and Centrifugo delivery
- **Multi-tenancy** — organization-based isolation, every query scoped by `organization_id`
- **Double-entry accounting** — financial module with proper debit/credit bookkeeping (int64 kopeks, no floats)
- **3D space validation** — AABB collision detection and parent containment checks for spatial layouts
- **Webhook dispatch** — HMAC-SHA256 signed, fire-and-forget delivery to external systems
- **File management** — presigned upload/download URLs via MinIO/S3
- **Notifications inbox** — per-user notifications with event-driven creation and Arcana reactive graph
- **Modular monolith** — strict module boundaries, modules communicate only via events, zero circular imports

### Business modules

| Module | Description |
|--------|-------------|
| **Catalog** | Products, categories, pricing rules, SKU/barcode, units of measurement |
| **Warehouse** | Stock levels, movements (receive/ship/transfer/adjust), inventory stocktaking, low-stock alerts |
| **Orders** | Order lifecycle, auto-numbering, refunds with stock return and finance reversal, payment tracking |
| **HR** | Employees, shifts, timesheets, payroll calculation (NDFL 13%), absence management (vacation/sick leave) |
| **Finance** | Chart of accounts (РСБУ), double-entry transactions, P&L reports, period management, cash operations |
| **Logistics** | Routes, stops, GPS tracking, driver assignment, geo-history |
| **CRM** | Customers and suppliers management, tags, categories, order/transaction history |
| **Settings** | Organization settings (currency, timezone, requisites, integrations, features) |
| **Documents** | HTML print forms — invoice, TORG-12, act, receipt, price tags |
| **Import** | Mass data import from CSV/XLSX (products, customers, suppliers) with Windows-1251 support |
| **Files** | Presigned S3/MinIO upload and download URLs, entity attachment |
| **Webhooks** | External endpoint subscriptions, HMAC signing, event-driven dispatch |
| **Notifications** | Per-user inbox, event-driven creation, read/unread tracking |
| **Space** | 3D AABB bounds validation, collision detection, parent containment |

### Integration stubs (Russian market)

- **FNS** (Federal Tax Service) — fiscal receipts, tax reporting
- **EDO** (Electronic Document Exchange) — UPD, invoices
- **Chestniy Znak** — product labeling and traceability
- **Bank** — payment processing, statement import

## Architecture

```
+--------------------------------------------------+
|                    API Layer                       |
|      REST (Chi) + Arcana (/arcana) + Webhooks     |
+--------------------------------------------------+
|                  Auth / RBAC                       |
|   Cookie Sessions (argon2id) + Permissions + Org   |
+--------------------------------------------------+
|               Module Layer (Systems)               |
|   Catalog | Warehouse | Order | HR | Finance | ... |
+--------------------------------------------------+
|                   Core Layer                       |
|       Entity | Event Bus | Process Engine          |
+-----------------+--------------------------------+
|     Arcana      |        Storage Layer             |
|     Engine      |   PostgreSQL | Redis | MinIO     |
+-----------------+--------------------------------+
|                Real-time Layer                     |
|    Centrifugo v6 (connect/subscribe proxy)         |
+--------------------------------------------------+
```

Dependency direction: API -> Module -> Core -> Storage. Never reversed.

## Tech stack

| Component | Technology | Why |
|-----------|-----------|-----|
| Language | Go 1.24 | Performance, concurrency, single binary |
| Database | PostgreSQL 16 | JSONB, partitioning, trigram search, RLS |
| Cache/Sessions | Redis 7 | Session store, rate limiting, idempotency |
| Real-time | Centrifugo v6 | Scalable WebSocket delivery, connect/subscribe proxy |
| File Storage | MinIO (S3-compatible) | Presigned URLs for direct browser uploads |
| HTTP Router | chi/v5 | Lightweight, net/http compatible |
| Auth | Cookie sessions + argon2id | HttpOnly cookies + Redis session store |
| Reactive Sync | [Arcana v0.1.3](https://github.com/FrankFMY/arcana) | Graph-based subscriptions, JSON Patch diffs, pagination |
| SQL Driver | pgx/v5 | Native PostgreSQL, no ORM |
| Logging | zerolog | Structured JSON logs |
| Config | go-envconfig | Env-based configuration |
| Testing | testify + testcontainers | Assertions + real PostgreSQL in tests |

## Project structure

```
bizengine/
+-- cmd/
|   +-- server/main.go           # Entry point, dependency injection
|   +-- gen-types/main.go        # TypeScript type generator
|   +-- seed/main.go             # Database seed utility
+-- internal/
|   +-- core/                    # ECS foundation
|   |   +-- entity/              # Entity + Component CRUD
|   |   +-- event/               # Event Bus + Event Store
|   |   +-- process/             # State machine engine
|   |   +-- auth/                # Sessions, RBAC, middleware
|   +-- module/                  # Business logic
|   |   +-- catalog/             # Product catalog, pricing rules
|   |   +-- warehouse/           # Inventory management, stocktaking
|   |   +-- order/               # Order processing, refunds
|   |   +-- hr/                  # Employees, payroll, absences
|   |   +-- finance/             # Double-entry accounting, P&L, periods, cash
|   |   +-- logistics/           # Routes and tracking
|   |   +-- crm/                 # Customer and supplier management
|   |   +-- settings/            # Organization settings
|   |   +-- file/                # File upload/download (S3/MinIO)
|   |   +-- space/               # 3D AABB spatial validation
|   +-- api/
|   |   +-- rest/                # Chi router, all HTTP handlers
|   |   +-- centrifugo/          # Connect/subscribe proxy, publisher
|   +-- graphs/                  # Arcana graph definitions (32 graphs)
|   +-- notification/            # Notification inbox service
|   +-- webhook/                 # Webhook dispatch service
|   +-- dataimport/              # Mass CSV/XLSX data import
|   +-- documents/               # HTML print form generation
|   +-- storage/
|   |   +-- postgres/            # All repository implementations
|   |   +-- redis/               # Session store
|   |   +-- s3/                  # MinIO/S3 client
|   +-- integration/             # External system stubs (FNS, EDO, etc.)
|   +-- export/                  # Excel/PDF export
+-- pkg/
|   +-- types/                   # Shared domain types
|   +-- config/                  # Environment config
|   +-- errs/                    # Typed errors (NotFound, Conflict, BadRequest)
|   +-- money/                   # Financial arithmetic (int64 kopeks)
|   +-- dsl/                     # YAML process definition parser + validator
+-- migrations/                  # 19 numbered up/down SQL migrations
+-- processes/                   # 4 YAML business process definitions
+-- deploy/                      # Dockerfile, docker-compose, centrifugo.json
+-- docs/                        # Technical documentation
+-- sdk/                         # TypeScript SDK
```

## API overview

Base URL: `/api/v1` | Auth: Cookie sessions (`credentials: 'include'`) | Format: `application/json`

Request bodies use the `{"data": {...}}` envelope. Responses use `{"ok": true, "data": {...}}` or `{"ok": false, "error": {...}}`.

### Auth
```
POST /api/v1/auth/register          # Create account (sets cookies)
POST /api/v1/auth/login             # Login (sets cookies)
POST /api/v1/auth/logout            # Clear session
POST /api/v1/auth/unlock            # Refresh expired seance (requires password)
POST /api/v1/auth/switch            # Switch active organization
GET  /api/v1/auth/check             # Current session info (user_id, org_id, role)
POST /api/v1/pass/temp              # Dev-only: create user+org in one call (no auth)
```

Auth model: phones -> users -> labors -> organizations. Password hashing: argon2id. Two cookies: `session` (72h) and `seance` (10min auto-refresh). See `docs/AUTH_RBAC.md`.

### Entities (universal CRUD)
```
POST   /api/v1/organizations/{orgID}/entities
GET    /api/v1/organizations/{orgID}/entities?kind=product&search=...&parent_id=...
GET    /api/v1/organizations/{orgID}/entities/{id}?include=components
PUT    /api/v1/organizations/{orgID}/entities/{id}
DELETE /api/v1/organizations/{orgID}/entities/{id}
```

### Components
```
PUT    /api/v1/organizations/{orgID}/entities/{id}/components/{type}
GET    /api/v1/organizations/{orgID}/entities/{id}/components/{type}
GET    /api/v1/organizations/{orgID}/entities/{id}/components
DELETE /api/v1/organizations/{orgID}/entities/{id}/components/{type}
```

Setting `type=bounds` triggers 3D AABB validation (collision detection + parent containment). Returns `422` with `{"bounds": "COLLISION"}` or `{"bounds": "OUTSIDE_PARENT"}` on validation failure.

### Catalog
```
POST   /api/v1/organizations/{orgID}/catalog/products
GET    /api/v1/organizations/{orgID}/catalog/products?category_id=...&in_stock=true&search=...
GET    /api/v1/organizations/{orgID}/catalog/products/{id}
PUT    /api/v1/organizations/{orgID}/catalog/products/{id}
POST   /api/v1/organizations/{orgID}/catalog/products/{id}/archive
POST   /api/v1/organizations/{orgID}/catalog/categories
GET    /api/v1/organizations/{orgID}/catalog/categories
PUT    /api/v1/organizations/{orgID}/catalog/categories/{id}
DELETE /api/v1/organizations/{orgID}/catalog/categories/{id}
```

### Warehouse
```
POST   /api/v1/organizations/{orgID}/warehouse/receive
POST   /api/v1/organizations/{orgID}/warehouse/ship
POST   /api/v1/organizations/{orgID}/warehouse/transfer
POST   /api/v1/organizations/{orgID}/warehouse/adjust
GET    /api/v1/organizations/{orgID}/warehouse/{id}/stock
GET    /api/v1/organizations/{orgID}/warehouse/{id}/stock/{productID}
GET    /api/v1/organizations/{orgID}/warehouse/low-stock
GET    /api/v1/organizations/{orgID}/warehouse/movements
```

### Orders
```
POST   /api/v1/organizations/{orgID}/orders
GET    /api/v1/organizations/{orgID}/orders?status=active&include=items
GET    /api/v1/organizations/{orgID}/orders/{id}
PUT    /api/v1/organizations/{orgID}/orders/{id}
POST   /api/v1/organizations/{orgID}/orders/{id}/submit
POST   /api/v1/organizations/{orgID}/orders/{id}/confirm
POST   /api/v1/organizations/{orgID}/orders/{id}/pay
POST   /api/v1/organizations/{orgID}/orders/{id}/ship
POST   /api/v1/organizations/{orgID}/orders/{id}/deliver
POST   /api/v1/organizations/{orgID}/orders/{id}/cancel
POST   /api/v1/organizations/{orgID}/orders/{id}/refund
GET    /api/v1/organizations/{orgID}/orders/refunds
GET    /api/v1/organizations/{orgID}/orders/refunds/{id}
```

### CRM
```
POST   /api/v1/organizations/{orgID}/crm/customers
GET    /api/v1/organizations/{orgID}/crm/customers
GET    /api/v1/organizations/{orgID}/crm/customers/{id}
PUT    /api/v1/organizations/{orgID}/crm/customers/{id}
PUT    /api/v1/organizations/{orgID}/crm/customers/{id}/tags
POST   /api/v1/organizations/{orgID}/crm/suppliers
GET    /api/v1/organizations/{orgID}/crm/suppliers
GET    /api/v1/organizations/{orgID}/crm/suppliers/{id}
PUT    /api/v1/organizations/{orgID}/crm/suppliers/{id}
```

### Settings
```
GET    /api/v1/organizations/{orgID}/settings
PUT    /api/v1/organizations/{orgID}/settings
PUT    /api/v1/organizations/{orgID}/settings/integrations
PUT    /api/v1/organizations/{orgID}/settings/logo
```

### Documents (print forms)
```
GET    /api/v1/organizations/{orgID}/documents/invoice/{orderID}
GET    /api/v1/organizations/{orgID}/documents/torg12/{orderID}
GET    /api/v1/organizations/{orgID}/documents/act/{orderID}
GET    /api/v1/organizations/{orgID}/documents/receipt/{orderID}
GET    /api/v1/organizations/{orgID}/documents/price-tags?product_ids=...
```

### Import (mass data)
```
POST   /api/v1/organizations/{orgID}/import/products      # CSV/XLSX multipart upload
POST   /api/v1/organizations/{orgID}/import/customers
POST   /api/v1/organizations/{orgID}/import/suppliers
```

### HR
```
POST   /api/v1/organizations/{orgID}/hr/employees
GET    /api/v1/organizations/{orgID}/hr/employees
GET    /api/v1/organizations/{orgID}/hr/employees/{id}
PUT    /api/v1/organizations/{orgID}/hr/employees/{id}
POST   /api/v1/organizations/{orgID}/hr/employees/{id}/terminate
POST   /api/v1/organizations/{orgID}/hr/shifts
GET    /api/v1/organizations/{orgID}/hr/shifts
PUT    /api/v1/organizations/{orgID}/hr/shifts/{id}
DELETE /api/v1/organizations/{orgID}/hr/shifts/{id}
POST   /api/v1/organizations/{orgID}/hr/timesheets/clock-in
POST   /api/v1/organizations/{orgID}/hr/timesheets/{id}/clock-out
GET    /api/v1/organizations/{orgID}/hr/timesheets
POST   /api/v1/organizations/{orgID}/hr/timesheets/{id}/approve
POST   /api/v1/organizations/{orgID}/hr/payrolls/calculate
POST   /api/v1/organizations/{orgID}/hr/payrolls/{id}/approve
GET    /api/v1/organizations/{orgID}/hr/payrolls
POST   /api/v1/organizations/{orgID}/hr/absences
POST   /api/v1/organizations/{orgID}/hr/absences/{id}/approve
POST   /api/v1/organizations/{orgID}/hr/absences/{id}/reject
GET    /api/v1/organizations/{orgID}/hr/absences
```

### Finance
```
POST   /api/v1/organizations/{orgID}/finance/accounts
GET    /api/v1/organizations/{orgID}/finance/accounts
GET    /api/v1/organizations/{orgID}/finance/accounts/{id}/balance
POST   /api/v1/organizations/{orgID}/finance/transactions
GET    /api/v1/organizations/{orgID}/finance/transactions
GET    /api/v1/organizations/{orgID}/finance/transactions/{id}
POST   /api/v1/organizations/{orgID}/finance/transactions/{id}/post
POST   /api/v1/organizations/{orgID}/finance/invoices
GET    /api/v1/organizations/{orgID}/finance/invoices
POST   /api/v1/organizations/{orgID}/finance/invoices/{id}/pay
GET    /api/v1/organizations/{orgID}/finance/reports/trial-balance
GET    /api/v1/organizations/{orgID}/finance/reports/pnl?from=...&to=...
POST   /api/v1/organizations/{orgID}/finance/periods/{year}/{month}/close
GET    /api/v1/organizations/{orgID}/finance/periods
POST   /api/v1/organizations/{orgID}/finance/cash-operations
GET    /api/v1/organizations/{orgID}/finance/cash-operations
```

### Logistics
```
POST   /api/v1/organizations/{orgID}/logistics/routes
GET    /api/v1/organizations/{orgID}/logistics/routes
GET    /api/v1/organizations/{orgID}/logistics/routes/{id}
POST   /api/v1/organizations/{orgID}/logistics/routes/{id}/start
POST   /api/v1/organizations/{orgID}/logistics/routes/{id}/complete
POST   /api/v1/organizations/{orgID}/logistics/routes/{id}/stops/{stopID}/arrive
POST   /api/v1/organizations/{orgID}/logistics/routes/{id}/stops/{stopID}/complete
POST   /api/v1/organizations/{orgID}/logistics/geo
GET    /api/v1/organizations/{orgID}/logistics/geo/{entityID}/track
```

### Files
```
POST   /api/v1/organizations/{orgID}/files/upload-url    # Request presigned upload URL
POST   /api/v1/organizations/{orgID}/files/{id}/confirm   # Confirm upload completed
GET    /api/v1/organizations/{orgID}/files/{id}           # Get file metadata + presigned download URL
GET    /api/v1/organizations/{orgID}/files?entity_id=...  # List files for entity
DELETE /api/v1/organizations/{orgID}/files/{id}           # Delete file
```

Upload flow: `POST /upload-url` -> client uploads directly to MinIO via presigned URL -> `POST /confirm` -> file status changes to `confirmed`.

### Webhooks
```
POST   /api/v1/organizations/{orgID}/webhooks              # Create webhook subscription
GET    /api/v1/organizations/{orgID}/webhooks              # List webhooks
DELETE /api/v1/organizations/{orgID}/webhooks/{id}         # Delete webhook
POST   /api/v1/organizations/{orgID}/webhooks/{id}/toggle  # Enable/disable
POST   /api/v1/organizations/{orgID}/webhooks/{id}/test    # Send test delivery
```

Webhooks are dispatched fire-and-forget on every event. Payloads are signed with HMAC-SHA256 (`X-Webhook-Signature` header).

### Notifications
```
GET    /api/v1/organizations/{orgID}/notifications           # List notifications (filter: ?read=true/false)
GET    /api/v1/organizations/{orgID}/notifications/count     # Count unread
POST   /api/v1/organizations/{orgID}/notifications/{id}/read # Mark single as read
POST   /api/v1/organizations/{orgID}/notifications/read-all  # Mark all as read
```

Notifications are created automatically by event subscriptions (e.g., `order.paid`, `warehouse.stock.low`, `hr.shift.created`).

### Events & Processes
```
GET    /api/v1/organizations/{orgID}/events                          # List events (paginated)
GET    /api/v1/organizations/{orgID}/events/entity/{entityID}        # Events for entity
POST   /api/v1/organizations/{orgID}/processes/definitions           # Create process definition
GET    /api/v1/organizations/{orgID}/processes/definitions           # List definitions
GET    /api/v1/organizations/{orgID}/processes/definitions/{defID}   # Get definition
PUT    /api/v1/organizations/{orgID}/processes/definitions/{defID}   # Update definition
DELETE /api/v1/organizations/{orgID}/processes/definitions/{defID}   # Delete definition
GET    /api/v1/organizations/{orgID}/processes/instances             # List process instances
GET    /api/v1/organizations/{orgID}/processes/instances/{id}        # Get instance
GET    /api/v1/organizations/{orgID}/processes/entity/{entityID}     # Processes for entity
POST   /api/v1/organizations/{orgID}/processes/trigger               # Trigger process manually
```

### Arcana (reactive data sync)
```
POST   /arcana/subscribe             # Subscribe to a graph (20 available)
POST   /arcana/unsubscribe           # Unsubscribe by params_hash
POST   /arcana/sync                  # Reconnect sync
GET    /arcana/active                # List active subscriptions
GET    /arcana/schema                # Graph definitions registry
GET    /arcana/health                # Engine health
```

### Integrations (Russian market stubs)
```
POST   /api/v1/organizations/{orgID}/integrations/fiscal/receipt
GET    /api/v1/organizations/{orgID}/integrations/fiscal/receipt/{id}
POST   /api/v1/organizations/{orgID}/integrations/edo/documents
GET    /api/v1/organizations/{orgID}/integrations/edo/documents/incoming
POST   /api/v1/organizations/{orgID}/integrations/edo/documents/{id}/accept
POST   /api/v1/organizations/{orgID}/integrations/edo/documents/{id}/reject
POST   /api/v1/organizations/{orgID}/integrations/marking/verify
POST   /api/v1/organizations/{orgID}/integrations/marking/receipt
POST   /api/v1/organizations/{orgID}/integrations/marking/shipment
```

### Health & Metrics
```
GET    /health    # No auth required
GET    /metrics   # Request statistics (avg latency, by status, by path)
```

## Event system

All mutations produce events that flow through the event bus. Subscribers react asynchronously:

| Event | Description |
|-------|-------------|
| `catalog.product.created/updated/archived` | Product lifecycle |
| `order.created/submitted/confirmed/paid/shipped/delivered/cancelled` | Order lifecycle |
| `order.refunded` | Order refund with stock return and finance reversal |
| `warehouse.stock.received/shipped/transferred/adjusted/low` | Inventory changes |
| `warehouse.inventory.started/counted/completed` | Inventory stocktaking |
| `hr.employee.hired/terminated` | Employee lifecycle |
| `hr.shift.created/started/completed` | Shift management |
| `hr.timesheet.approved` | Timesheet approval |
| `hr.payroll.calculated/approved` | Payroll calculation and approval |
| `hr.absence.requested/approved/rejected` | Absence management (vacation, sick leave) |
| `finance.transaction.created/posted` | Financial transactions |
| `finance.period.opened/closed` | Accounting period management |
| `finance.cash.created` | Cash operations (deposits, withdrawals) |
| `logistics.route.created/started/completed` | Route lifecycle |
| `logistics.stop.arrived/completed` | Stop tracking |
| `logistics.geo.updated` | GPS position updates |
| `crm.customer.created/updated` | Customer management |
| `crm.supplier.created/updated` | Supplier management |
| `settings.updated` | Organization settings changes |
| `notification.created` | Notification inbox |

The event bus supports exact matching (`order.paid`) and wildcard subscriptions (`warehouse.stock.*`). Event subscribers run in detached goroutines to avoid blocking the request.

## Process engine

YAML-defined state machines that react to events and advance through states:

```yaml
# processes/order_fulfillment.yaml
id: order_fulfillment
name: Order Fulfillment
trigger_on: order.created
entity_kind: order
init_state: new

states:
  new:
    name: New Order
    transitions:
      - to: confirmed
        event: order.confirmed
  confirmed:
    name: Confirmed
    transitions:
      - to: paid
        event: order.paid
        actions:
          - type: emit_event
            event: notification.created
  paid:
    name: Paid
    transitions:
      - to: shipped
        event: order.shipped
  shipped:
    name: Shipped
    transitions:
      - to: delivered
        event: order.delivered
  delivered:
    name: Delivered
    terminal: true
  cancelled:
    name: Cancelled
    terminal: true
```

**Built-in process definitions:** order_fulfillment, stock_replenishment, delivery_tracking, employee_onboarding.

**Custom definitions** can be created via the REST API (`POST /processes/definitions`) with full validation: nonexistent transition targets, missing terminal states, and invalid action types all return `400 BAD_REQUEST`.

## RBAC permissions

| Role | Permissions |
|------|-------------|
| **owner** | All permissions |
| **admin** | All permissions |
| **manager** | catalog.*, warehouse.*, order.*, logistics.*, finance.view |
| **operator** | entity.read, catalog.view, warehouse.receive/ship, order.view |
| **viewer** | entity.read, catalog.view, order.view |

## Getting started

### Prerequisites

- Go 1.24+
- Docker and Docker Compose
- Make

### Setup

```bash
# Clone
git clone https://github.com/FrankFMY/Bizengine.git
cd Bizengine

# Start infrastructure (PostgreSQL, Redis, MinIO, Centrifugo)
docker compose up -d

# Copy environment config
cp .env.example .env

# Run migrations
make migrate-up

# Seed demo data (optional)
go run ./cmd/seed

# Start server (with hot-reload)
make dev
```

The server starts on `http://localhost:8080`. Health check: `GET /health`.

### Infrastructure ports

| Service | Port | Purpose |
|---------|------|---------|
| BizEngine | 8080 | REST API |
| PostgreSQL | 5436 | Database |
| Redis | 6381 | Sessions, cache |
| MinIO API | 9000 | Object storage |
| MinIO Console | 9001 | Web UI |
| Centrifugo | 8001 | WebSocket server |

### Build

```bash
make build        # Compile to ./bin/engine
make run          # Build and run
```

### Testing

```bash
make test                 # Unit tests (~300 tests, ~5s)
make test-integration     # Integration tests (requires Docker)
make test-coverage        # Coverage report -> coverage.html
```

### Docker

```bash
# Build image
docker build -f deploy/Dockerfile -t bizengine .

# Run with compose (includes PostgreSQL, Redis, MinIO, Centrifugo)
docker compose up -d
```

## Makefile commands

| Command | Description |
|---------|-------------|
| `make build` | Compile server binary |
| `make run` | Build and run |
| `make dev` | Hot-reload development (air) |
| `make test` | Unit tests with race detector |
| `make test-integration` | Integration tests with testcontainers |
| `make test-coverage` | Generate HTML coverage report |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Rollback last migration |
| `make migrate-create` | Create new migration pair |
| `make lint` | Run go vet |
| `make clean` | Remove build artifacts |

## Configuration

All configuration via environment variables (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `bizengine` | Database user |
| `DB_PASSWORD` | `bizengine` | Database password |
| `DB_NAME` | `bizengine` | Database name |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_DB` | `0` | Redis database |
| `SESSION_TTL` | `72h` | Session cookie lifetime |
| `SESSION_SEANCE_TTL` | `10m` | Seance cookie lifetime (auto-refresh) |
| `SESSION_COOKIE_SECURE` | `false` | Secure flag on cookies (true in production) |
| `CENTRIFUGO_API_URL` | `http://localhost:8001/api` | Centrifugo HTTP API endpoint |
| `CENTRIFUGO_API_KEY` | — | Centrifugo API key |
| `S3_ENDPOINT` | `http://localhost:9000` | MinIO/S3 endpoint |
| `S3_BUCKET` | `bizengine` | Storage bucket |
| `S3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `S3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `ALLOWED_ORIGINS` | `localhost:3000,5173` | CORS allowed origins |
| `LOG_LEVEL` | `debug` | Log level (debug/info/warn/error) |
| `ENV` | `development` | Environment name |

## Middleware stack

Request processing order:

1. **Recoverer** — panic recovery, returns 500
2. **SecurityHeaders** — X-Frame-Options: DENY, X-Content-Type-Options: nosniff, CSP, HSTS, Referrer-Policy, Permissions-Policy
3. **CORS** — configurable allowed origins
4. **Metrics** — request timing, status code tracking
5. **Logger** — structured JSON request logs
6. **RateLimit** — 300 req/min per IP (sliding window, uses X-Forwarded-For)
7. **APIVersion** — X-API-Version header
8. **TrimStrings** — whitespace cleanup on string fields
9. **UnwrapRequest** — strips `{"data": ...}` envelope from POST/PUT/PATCH bodies
10. **Idempotency** — Redis-backed deduplication for POST requests
11. **Auth** — session + seance cookie validation

## Codebase stats

- **~35,000 lines** of Go code
- **240+ unit tests** across 26 test suites
- **19 database migrations** (38 files with up/down)
- **4 YAML process definitions** + custom definitions via API
- **32 Arcana graph definitions** for reactive data sync
- **180+ REST API endpoints** + 6 Arcana endpoints
- **40+ event types** with async subscriber fan-out

## Author

**Pryanishnikov Artem Alekseevich**

- Email: Pryanishnikovartem@gmail.com
- Telegram: [@FrankFMY](https://t.me/FrankFMY)
- GitHub: [@FrankFMY](https://github.com/FrankFMY)

## License

Proprietary. All rights reserved. See [LICENSE](LICENSE) for details.

No part of this software may be used, copied, modified, or distributed without prior written consent of the author.
