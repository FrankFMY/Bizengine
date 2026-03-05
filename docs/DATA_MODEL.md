# Data Model

Migrations: 001_core through 009_auth_redesign (18 files with up/down).

## Global Rules

- All IDs: UUID v4 (`uuid_generate_v4()`).
- All tables have `ver INTEGER NOT NULL DEFAULT 1` and `upd TIMESTAMPTZ NOT NULL DEFAULT now()`, auto-incremented by the `auto_ver_upd` trigger on every UPDATE.
- All timestamps: `TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
- Soft delete: `deleted_at TIMESTAMPTZ` (NULL = not deleted).
- Money: `BIGINT` (kopeks). No NUMERIC/DECIMAL for monetary amounts.
- Quantities: `NUMERIC(15,4)` (fractional units: kg, liters, meters).
- JSON data: `JSONB NOT NULL DEFAULT '{}'`.
- All FK: `ON DELETE CASCADE` or `ON DELETE RESTRICT` (explicit per table).
- All business tables with `organization_id` have RLS enabled.
- Extensions: `uuid-ossp`, `pg_trgm`.

## 009_auth_redesign — Key Changes

Migration 009 performed the following structural changes:

1. **workspaces -> organizations**: renamed table, added `inn VARCHAR(12)`, `ver`, `upd`.
2. **workspace_members -> labors**: renamed table, `workspace_id` -> `organization_id`, added `admin BOOLEAN`, `id UUID`, `ver`, `upd`, `iat`. Kept `role` and `permissions` columns intact.
3. **phones**: new table for phone numbers linked to users.
4. **users**: added `ver`, `upd`, `name`, `secret`, `aat`.
5. **workspace_id -> organization_id**: renamed in ALL business tables (entities, components, orders, order_items, stock_levels, stock_movements, shifts, timesheets, accounts, transactions, transaction_lines, invoices, routes, route_stops, geo_tracks, events, process_definitions, process_instances, order_number_sequences).
6. **ver/upd columns**: added to all business tables that didn't have them.
7. **auto_ver_upd trigger**: created on all tables — automatically increments `ver` and sets `upd = now()` on every UPDATE.

## Auth Tables

### phones

```sql
CREATE TABLE phones (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ver       INTEGER NOT NULL DEFAULT 1,
    upd       TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat       TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id   UUID NOT NULL REFERENCES users(id),
    unformat  VARCHAR(20) NOT NULL,  -- digits only, e.g. "79991234567"
    format    VARCHAR(30) NOT NULL,  -- human-readable, e.g. "7 (999) 123-45-67"
    country   VARCHAR(5) NOT NULL DEFAULT 'RU'
);
CREATE UNIQUE INDEX idx_phones_unformat ON phones(unformat);
```

### users

```sql
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,       -- argon2id hash
    full_name     TEXT NOT NULL DEFAULT '',
    phone         TEXT,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    name          VARCHAR(100),        -- display name
    secret        VARCHAR(255),        -- alternate argon2id hash (for /pass/temp)
    aat           TIMESTAMPTZ,         -- last auth activity timestamp
    ver           INTEGER NOT NULL DEFAULT 1,
    upd           TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_users_email ON users(lower(email));
```

### organizations (formerly workspaces)

```sql
CREATE TABLE organizations (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    plan       TEXT NOT NULL DEFAULT 'free',
    settings   JSONB NOT NULL DEFAULT '{}',
    inn        VARCHAR(12),
    ver        INTEGER NOT NULL DEFAULT 1,
    upd        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### labors (formerly workspace_members)

```sql
CREATE TABLE labors (
    id              UUID DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'viewer',        -- owner, admin, manager, operator, viewer
    permissions     JSONB NOT NULL DEFAULT '[]',           -- additional granular permissions
    admin           BOOLEAN NOT NULL DEFAULT true,         -- shortcut: admin=true maps to role=owner
    ver             INTEGER NOT NULL DEFAULT 1,
    upd             TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat             TIMESTAMPTZ NOT NULL DEFAULT now(),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, user_id)
);
```

## Core Tables

### entities

```sql
CREATE TABLE entities (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,         -- product, category, service, employee, warehouse, customer, vehicle, location
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active',
    parent_id       UUID REFERENCES entities(id) ON DELETE SET NULL,
    meta            JSONB NOT NULL DEFAULT '{}',
    sort_order      INT NOT NULL DEFAULT 0,
    ver             INTEGER NOT NULL DEFAULT 1,
    upd             TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);
```

Entity `kind` values used across modules: `product`, `category`, `service`, `employee`, `warehouse`, `customer`, `vehicle`, `location`.

### components

```sql
CREATE TABLE components (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entity_id       UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL,
    type            TEXT NOT NULL,          -- price, barcode, media, attributes, salary, employment
    data            JSONB NOT NULL DEFAULT '{}',
    version         BIGINT NOT NULL DEFAULT 1,
    ver             INTEGER NOT NULL DEFAULT 1,
    upd             TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, type)
);
```

### events

```sql
CREATE TABLE events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL,
    entity_id       UUID,
    type            TEXT NOT NULL,
    data            JSONB NOT NULL DEFAULT '{}',
    actor_id        UUID,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version         BIGINT NOT NULL DEFAULT 1
) PARTITION BY RANGE (timestamp);
```

### process_definitions / process_instances

State machine definitions (YAML-based) and their running instances. See PROCESS_ENGINE.md for details.

## Business Tables

All business tables follow the same pattern: `organization_id` for tenant isolation, `ver`/`upd` for versioning, RLS enabled.

### Warehouse: stock_levels, stock_movements

- `stock_levels`: composite PK `(organization_id, product_id, warehouse_id)`. Tracks `quantity`, `reserved`. Available = `quantity - reserved`.
- `stock_movements`: audit log of all stock operations (receive, ship, transfer, adjust, return, write_off, reserve, unreserve).

### Orders: orders, order_items, order_number_sequences

- `orders`: linked to entity via `entity_id`. Human-readable `number` (auto-generated). Money fields in BIGINT kopeks.
- `order_items`: line items with snapshot of product name/price at time of order.

### HR: shifts, timesheets

- Employee data stored as entities (kind=employee) with salary/employment components.
- `shifts`: scheduled work periods.
- `timesheets`: actual clock-in/clock-out records.

### Finance: accounts, transactions, transaction_lines, invoices

- Double-entry bookkeeping. Each transaction has balanced debit/credit lines.
- `CHECK (debit >= 0 AND credit >= 0 AND NOT (debit > 0 AND credit > 0))` on transaction_lines.

### Logistics: routes, route_stops, geo_tracks

- `routes`: delivery routes with driver/vehicle assignment.
- `route_stops`: ordered stops with GPS coordinates.
- `geo_tracks`: partitioned GPS telemetry data.

## auto_ver_upd Trigger

Applied to all tables listed above. On every UPDATE:

```sql
CREATE OR REPLACE FUNCTION auto_ver_upd() RETURNS trigger AS $$
BEGIN
    NEW.ver := OLD.ver + 1;
    NEW.upd := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

Tables with trigger: `users`, `phones`, `organizations`, `labors`, `entities`, `components`, `orders`, `order_items`, `stock_levels`, `stock_movements`, `shifts`, `timesheets`, `accounts`, `transactions`, `transaction_lines`, `invoices`, `routes`, `route_stops`, `geo_tracks`.
