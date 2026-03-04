# BizEngine

Server engine for creating digital twins of businesses. Turns any business — from a coffee shop to a logistics company — into a programmable system of entities, components, events, and automated processes.

## What it does

BizEngine provides a unified backend for managing business operations through an Entity-Component-System (ECS) architecture combined with Event Sourcing. Every business object (product, order, warehouse, employee, vehicle, point of sale) is an **Entity** with attached **Components** (typed JSONB data). Every mutation produces an **Event** in an append-only log. Business workflows are **state machines** defined in YAML. Real-time UI updates are delivered through a **Reactive Views Engine** powered by Centrifugo.

### Core capabilities

- **Universal entity model** — any business object is an Entity with flexible Components, no schema migrations needed for new object types
- **Event sourcing** — full audit trail, reactive views via Centrifugo, replay to any point in time
- **Process engine** — YAML-defined state machines for order fulfillment, stock replenishment, delivery tracking, employee onboarding
- **Reactive views** — normalized refs/tables architecture with automatic diff computation and real-time delivery via Centrifugo
- **Multi-tenancy** — workspace-based isolation, every query scoped by `workspace_id`
- **Double-entry accounting** — financial module with proper debit/credit bookkeeping (int64 kopeks, no floats)
- **Modular monolith** — strict module boundaries, modules communicate only via events, zero circular imports

### Business modules

| Module | Description |
|--------|-------------|
| **Catalog** | Products, categories, pricing, SKU/barcode, attributes |
| **Warehouse** | Stock levels, movements (receive/ship/transfer/adjust), low-stock alerts |
| **Orders** | Order lifecycle, auto-numbering, status transitions, payment tracking |
| **HR** | Employees, departments, shifts, timesheets, clock-in/clock-out |
| **Finance** | Chart of accounts, double-entry transactions, trial balance reports |
| **Logistics** | Routes, stops, GPS tracking, driver assignment, geo-history |

### Integration stubs (Russian market)

- **FNS** (Federal Tax Service) — fiscal receipts, tax reporting
- **EDO** (Electronic Document Exchange) — UPD, invoices
- **Chestniy Znak** — product labeling and traceability
- **Bank** — payment processing, statement import

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   API Layer                      │
│             REST (Chi) + Views API               │
├─────────────────────────────────────────────────┤
│                 Auth / RBAC                      │
│    Cookie Sessions + Permissions + Workspace     │
├─────────────────────────────────────────────────┤
│              Module Layer (Systems)              │
│  Catalog │ Warehouse │ Order │ HR │ Finance │ …  │
├─────────────────────────────────────────────────┤
│                  Core Layer                      │
│      Entity │ Event Bus │ Process Engine         │
├────────────────────┬────────────────────────────┤
│  Reactive Views    │       Storage Layer         │
│  Engine + Triggers │  PostgreSQL │ Redis │ NATS  │
├────────────────────┴────────────────────────────┤
│               Real-time Layer                    │
│   Centrifugo v6 (connect/subscribe proxy)        │
└─────────────────────────────────────────────────┘
```

Dependency direction: API → Module → Core → Storage. Never reversed.

## Tech stack

| Component | Technology | Why |
|-----------|-----------|-----|
| Language | Go 1.24 | Performance, concurrency, single binary |
| Database | PostgreSQL 16 | JSONB, partitioning, trigram search, RLS |
| Cache/Sessions | Redis 7 | Session store, rate limiting, caching |
| Real-time | Centrifugo v6 | Scalable WebSocket delivery, connect/subscribe proxy |
| Messaging | NATS 2.10 | JetStream for distributed events (future) |
| HTTP Router | chi/v5 | Lightweight, net/http compatible |
| Auth | Cookie sessions | HttpOnly cookies + Redis session store |
| SQL Driver | pgx/v5 | Native PostgreSQL, no ORM |
| Logging | zerolog | Structured JSON logs |
| Config | go-envconfig | Env-based configuration |
| Testing | testify + testcontainers | Assertions + real PostgreSQL in tests |

## Project structure

```
bizengine/
├── cmd/server/main.go           # Entry point, dependency injection
├── internal/
│   ├── core/                    # ECS foundation
│   │   ├── entity/              # Entity + Component CRUD
│   │   ├── event/               # Event Bus + Event Store
│   │   ├── process/             # State machine engine
│   │   └── auth/                # Sessions, RBAC, middleware
│   ├── module/                  # Business logic
│   │   ├── catalog/             # Product catalog
│   │   ├── warehouse/           # Inventory management
│   │   ├── order/               # Order processing
│   │   ├── hr/                  # Human resources
│   │   ├── finance/             # Double-entry accounting
│   │   └── logistics/           # Routes and tracking
│   ├── api/
│   │   ├── rest/                # Chi router, all HTTP handlers
│   │   └── centrifugo/          # Connect/subscribe proxy, publisher
│   ├── views/                   # Reactive views engine
│   │   ├── defs/                # 20 view definitions
│   │   ├── manager.go           # Subscribe, invalidate, sync
│   │   ├── diff.go              # Refs diff computation
│   │   ├── triggers.go          # Event → view mapping
│   │   └── publisher.go         # Centrifugo HTTP API client
│   ├── storage/postgres/        # All repository implementations
│   └── integration/             # External system stubs
├── sdk/
│   ├── views.d.ts               # Auto-generated TypeScript types (20 views)
│   └── client.ts                # ArcanaClient for Svelte 5
├── scripts/
│   └── gen_view_types.go        # TypeScript code generator
├── pkg/
│   ├── types/                   # Shared domain types
│   ├── config/                  # Environment config
│   ├── errs/                    # Typed errors (NotFound, Conflict, etc.)
│   ├── money/                   # Financial arithmetic (int64 kopeks)
│   └── dsl/                     # YAML process definition parser
├── migrations/                  # 8 numbered up/down SQL migrations
├── processes/                   # YAML business process definitions
├── deploy/                      # Dockerfile, docker-compose, centrifugo.json
└── docs/                        # Full technical specification
```

## API overview

Base URL: `/api/v1` | Auth: Cookie sessions (`credentials: 'include'`) | Format: `application/json`

### Auth
```
POST /api/v1/auth/register          # Create account (sets cookies)
POST /api/v1/auth/login             # Login (sets cookies)
POST /api/v1/auth/logout            # Clear session
POST /api/v1/auth/unlock            # Refresh expired seance
POST /api/v1/auth/switch            # Switch active workspace
GET  /api/v1/auth/check             # Current session info
```

### Entities (universal CRUD)
```
POST   /api/v1/workspaces/{wsID}/entities
GET    /api/v1/workspaces/{wsID}/entities?kind=product&search=...
GET    /api/v1/workspaces/{wsID}/entities/{id}?include=components
PUT    /api/v1/workspaces/{wsID}/entities/{id}
DELETE /api/v1/workspaces/{wsID}/entities/{id}
```

### Components
```
PUT    /api/v1/workspaces/{wsID}/entities/{id}/components/{type}
GET    /api/v1/workspaces/{wsID}/entities/{id}/components/{type}
GET    /api/v1/workspaces/{wsID}/entities/{id}/components
DELETE /api/v1/workspaces/{wsID}/entities/{id}/components/{type}
```

### Catalog
```
POST   /api/v1/workspaces/{wsID}/catalog/products
GET    /api/v1/workspaces/{wsID}/catalog/products?category_id=...&in_stock=true
GET    /api/v1/workspaces/{wsID}/catalog/products/{id}
PUT    /api/v1/workspaces/{wsID}/catalog/products/{id}
POST   /api/v1/workspaces/{wsID}/catalog/products/{id}/archive
POST   /api/v1/workspaces/{wsID}/catalog/categories
GET    /api/v1/workspaces/{wsID}/catalog/categories
PUT    /api/v1/workspaces/{wsID}/catalog/categories/{id}
DELETE /api/v1/workspaces/{wsID}/catalog/categories/{id}
```

### Warehouse
```
POST   /api/v1/workspaces/{wsID}/warehouse/receive
POST   /api/v1/workspaces/{wsID}/warehouse/ship
POST   /api/v1/workspaces/{wsID}/warehouse/transfer
POST   /api/v1/workspaces/{wsID}/warehouse/adjust
GET    /api/v1/workspaces/{wsID}/warehouse/{id}/stock
GET    /api/v1/workspaces/{wsID}/warehouse/low-stock
GET    /api/v1/workspaces/{wsID}/warehouse/movements
```

### Orders
```
POST   /api/v1/workspaces/{wsID}/orders
GET    /api/v1/workspaces/{wsID}/orders?status=active&include=items
GET    /api/v1/workspaces/{wsID}/orders/{id}
PUT    /api/v1/workspaces/{wsID}/orders/{id}
POST   /api/v1/workspaces/{wsID}/orders/{id}/confirm
POST   /api/v1/workspaces/{wsID}/orders/{id}/pay
POST   /api/v1/workspaces/{wsID}/orders/{id}/ship
POST   /api/v1/workspaces/{wsID}/orders/{id}/deliver
POST   /api/v1/workspaces/{wsID}/orders/{id}/cancel
```

### HR
```
POST   /api/v1/workspaces/{wsID}/hr/employees
GET    /api/v1/workspaces/{wsID}/hr/employees
GET    /api/v1/workspaces/{wsID}/hr/employees/{id}
PUT    /api/v1/workspaces/{wsID}/hr/employees/{id}
POST   /api/v1/workspaces/{wsID}/hr/employees/{id}/terminate
POST   /api/v1/workspaces/{wsID}/hr/shifts
GET    /api/v1/workspaces/{wsID}/hr/shifts
PUT    /api/v1/workspaces/{wsID}/hr/shifts/{id}
DELETE /api/v1/workspaces/{wsID}/hr/shifts/{id}
POST   /api/v1/workspaces/{wsID}/hr/timesheets/clock-in
POST   /api/v1/workspaces/{wsID}/hr/timesheets/{id}/clock-out
GET    /api/v1/workspaces/{wsID}/hr/timesheets
POST   /api/v1/workspaces/{wsID}/hr/timesheets/{id}/approve
```

### Finance
```
POST   /api/v1/workspaces/{wsID}/finance/accounts
GET    /api/v1/workspaces/{wsID}/finance/accounts
GET    /api/v1/workspaces/{wsID}/finance/accounts/{id}/balance
POST   /api/v1/workspaces/{wsID}/finance/transactions
GET    /api/v1/workspaces/{wsID}/finance/transactions
GET    /api/v1/workspaces/{wsID}/finance/transactions/{id}
POST   /api/v1/workspaces/{wsID}/finance/transactions/{id}/post
POST   /api/v1/workspaces/{wsID}/finance/invoices
GET    /api/v1/workspaces/{wsID}/finance/invoices
POST   /api/v1/workspaces/{wsID}/finance/invoices/{id}/pay
GET    /api/v1/workspaces/{wsID}/finance/reports/trial-balance
```

### Logistics
```
POST   /api/v1/workspaces/{wsID}/logistics/routes
GET    /api/v1/workspaces/{wsID}/logistics/routes
GET    /api/v1/workspaces/{wsID}/logistics/routes/{id}
POST   /api/v1/workspaces/{wsID}/logistics/routes/{id}/start
POST   /api/v1/workspaces/{wsID}/logistics/routes/{id}/complete
POST   /api/v1/workspaces/{wsID}/logistics/routes/{id}/stops/{stopID}/arrive
POST   /api/v1/workspaces/{wsID}/logistics/routes/{id}/stops/{stopID}/complete
POST   /api/v1/workspaces/{wsID}/logistics/geo
GET    /api/v1/workspaces/{wsID}/logistics/geo/{entityID}/track
```

### Events & Processes
```
GET    /api/v1/workspaces/{wsID}/events
GET    /api/v1/workspaces/{wsID}/events/entity/{entityID}
GET    /api/v1/workspaces/{wsID}/processes/definitions
GET    /api/v1/workspaces/{wsID}/processes/definitions/{defID}
GET    /api/v1/workspaces/{wsID}/processes/instances
GET    /api/v1/workspaces/{wsID}/processes/instances/{id}
GET    /api/v1/workspaces/{wsID}/processes/entity/{entityID}
POST   /api/v1/workspaces/{wsID}/processes/trigger
```

### Reactive Views
```
POST   /api/v1/workspaces/{wsID}/views/subscribe
POST   /api/v1/workspaces/{wsID}/views/unsubscribe
GET    /api/v1/workspaces/{wsID}/views/active
POST   /api/v1/workspaces/{wsID}/views/sync
```

### Health
```
GET    /health    # No auth required
```

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

# Start infrastructure
docker compose up -d

# Copy environment config
cp .env.example .env

# Run migrations
make migrate-up

# Start server (with hot-reload)
make dev
```

The server starts on `http://localhost:8080`. Health check: `GET /health`.
Centrifugo runs on `http://localhost:8001`.

### Build

```bash
make build        # Compile to ./bin/engine
make run          # Build and run
```

### Testing

```bash
make test                 # Unit tests (~300 tests, ~5s)
make test-integration     # Integration tests (requires Docker)
make test-coverage        # Coverage report → coverage.html
```

### Docker

```bash
# Build image
docker build -f deploy/Dockerfile -t bizengine .

# Run with compose (includes PostgreSQL, Redis, NATS, Centrifugo)
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
| `SESSION_TTL` | `72h` | Session cookie lifetime |
| `SESSION_SEANCE_TTL` | `10m` | Seance cookie lifetime (auto-refresh) |
| `SESSION_COOKIE_SECURE` | `false` | Secure flag on cookies (true in production) |
| `CENTRIFUGO_API_URL` | `http://localhost:8001/api` | Centrifugo HTTP API endpoint |
| `CENTRIFUGO_API_KEY` | — | Centrifugo API key |
| `LOG_LEVEL` | `debug` | Log level (debug/info/warn/error) |
| `ENV` | `development` | Environment name |

## Codebase stats

- **~22,500 lines** of Go code
- **294 unit tests** across 18 test suites
- **8 database migrations** (16 files with up/down)
- **4 YAML process definitions**
- **20 reactive view definitions**
- **95+ REST API endpoints**
- **0 TODO/FIXME/HACK markers** in Go code
- **TypeScript SDK**: 2 files (views.d.ts + client.ts)

## Author

**Pryanishnikov Artem Alekseevich** (Прянишников Артём Алексеевич)

- Email: Pryanishnikovartem@gmail.com
- Telegram: [@FrankFMY](https://t.me/FrankFMY)
- GitHub: [@FrankFMY](https://github.com/FrankFMY)

## License

Proprietary. All rights reserved. See [LICENSE](LICENSE) for details.

No part of this software may be used, copied, modified, or distributed without prior written consent of the author.
