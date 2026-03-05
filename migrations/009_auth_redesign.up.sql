-- 009_auth_redesign: Migrate auth schema (workspaces→organizations, workspace_members→labors, phones, ver/upd)

-- ============================================================
-- 1. Users: add ver, upd, name, secret, aat
-- ============================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS secret VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS aat TIMESTAMPTZ;

-- ============================================================
-- 2. Phones table
-- ============================================================

CREATE TABLE IF NOT EXISTS phones (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ver       INTEGER NOT NULL DEFAULT 1,
    upd       TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat       TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id   UUID NOT NULL REFERENCES users(id),
    unformat  VARCHAR(20) NOT NULL,
    format    VARCHAR(30) NOT NULL,
    country   VARCHAR(5) NOT NULL DEFAULT 'RU'
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_phones_unformat ON phones(unformat);

-- ============================================================
-- 3. Rename workspaces → organizations
-- ============================================================

ALTER TABLE workspaces RENAME TO organizations;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS inn VARCHAR(12);
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- ============================================================
-- 4. Rename workspace_members → labors (KEEP role + permissions!)
-- ============================================================

ALTER TABLE workspace_members RENAME TO labors;
ALTER TABLE labors RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE labors ADD COLUMN IF NOT EXISTS admin BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE labors ADD COLUMN IF NOT EXISTS id UUID DEFAULT uuid_generate_v4();
ALTER TABLE labors ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE labors ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE labors ADD COLUMN IF NOT EXISTS iat TIMESTAMPTZ NOT NULL DEFAULT now();

-- ============================================================
-- 5. Rename workspace_id → organization_id in all business tables
-- ============================================================

ALTER TABLE entities RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE components RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE orders RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE order_items RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE stock_levels RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE stock_movements RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE shifts RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE timesheets RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE accounts RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE transactions RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE transaction_lines RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE invoices RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE routes RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE route_stops RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE geo_tracks RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE events RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE process_definitions RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE process_instances RENAME COLUMN workspace_id TO organization_id;
ALTER TABLE order_number_sequences RENAME COLUMN workspace_id TO organization_id;

-- ============================================================
-- 6. Add ver/upd to business tables that don't have them
-- ============================================================

-- entities
ALTER TABLE entities ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE entities ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- components
ALTER TABLE components ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE components ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- orders
ALTER TABLE orders ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- order_items
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- stock_levels
ALTER TABLE stock_levels ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE stock_levels ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- stock_movements
ALTER TABLE stock_movements ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE stock_movements ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- shifts
ALTER TABLE shifts ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE shifts ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- timesheets
ALTER TABLE timesheets ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE timesheets ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- accounts
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- transactions
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- transaction_lines
ALTER TABLE transaction_lines ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE transaction_lines ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- invoices
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- routes
ALTER TABLE routes ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE routes ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- route_stops
ALTER TABLE route_stops ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE route_stops ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- geo_tracks
ALTER TABLE geo_tracks ADD COLUMN IF NOT EXISTS ver INTEGER NOT NULL DEFAULT 1;
ALTER TABLE geo_tracks ADD COLUMN IF NOT EXISTS upd TIMESTAMPTZ NOT NULL DEFAULT now();

-- ============================================================
-- 7. Auto ver/upd trigger function + triggers for all tables
-- ============================================================

CREATE OR REPLACE FUNCTION auto_ver_upd() RETURNS trigger AS $$
BEGIN
    NEW.ver := OLD.ver + 1;
    NEW.upd := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tbl TEXT;
BEGIN
    FOR tbl IN
        SELECT unnest(ARRAY[
            'users', 'phones', 'organizations', 'labors',
            'entities', 'components',
            'orders', 'order_items',
            'stock_levels', 'stock_movements',
            'shifts', 'timesheets',
            'accounts', 'transactions', 'transaction_lines', 'invoices',
            'routes', 'route_stops', 'geo_tracks'
        ])
    LOOP
        EXECUTE format(
            'DROP TRIGGER IF EXISTS trg_ver_upd ON %I; CREATE TRIGGER trg_ver_upd BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION auto_ver_upd();',
            tbl, tbl
        );
    END LOOP;
END;
$$;

-- ============================================================
-- 8. Rename indexes for clarity
-- ============================================================

ALTER INDEX IF EXISTS idx_entities_ws_kind RENAME TO idx_entities_org_kind;
ALTER INDEX IF EXISTS idx_entities_ws_status RENAME TO idx_entities_org_status;
ALTER INDEX IF EXISTS idx_components_ws_type RENAME TO idx_components_org_type;
ALTER INDEX IF EXISTS idx_events_ws_ts RENAME TO idx_events_org_ts;
