-- 009_auth_redesign (down): Reverse auth schema migration

-- ============================================================
-- 1. Drop triggers and function
-- ============================================================

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
        EXECUTE format('DROP TRIGGER IF EXISTS trg_ver_upd ON %I;', tbl);
    END LOOP;
END;
$$;

DROP FUNCTION IF EXISTS auto_ver_upd();

-- ============================================================
-- 2. Rename indexes back
-- ============================================================

ALTER INDEX IF EXISTS idx_entities_org_kind RENAME TO idx_entities_ws_kind;
ALTER INDEX IF EXISTS idx_entities_org_status RENAME TO idx_entities_ws_status;
ALTER INDEX IF EXISTS idx_components_org_type RENAME TO idx_components_ws_type;
ALTER INDEX IF EXISTS idx_events_org_ts RENAME TO idx_events_ws_ts;

-- ============================================================
-- 3. Drop ver/upd from business tables
-- ============================================================

ALTER TABLE geo_tracks DROP COLUMN IF EXISTS upd;
ALTER TABLE geo_tracks DROP COLUMN IF EXISTS ver;
ALTER TABLE route_stops DROP COLUMN IF EXISTS upd;
ALTER TABLE route_stops DROP COLUMN IF EXISTS ver;
ALTER TABLE routes DROP COLUMN IF EXISTS upd;
ALTER TABLE routes DROP COLUMN IF EXISTS ver;
ALTER TABLE invoices DROP COLUMN IF EXISTS upd;
ALTER TABLE invoices DROP COLUMN IF EXISTS ver;
ALTER TABLE transaction_lines DROP COLUMN IF EXISTS upd;
ALTER TABLE transaction_lines DROP COLUMN IF EXISTS ver;
ALTER TABLE transactions DROP COLUMN IF EXISTS upd;
ALTER TABLE transactions DROP COLUMN IF EXISTS ver;
ALTER TABLE accounts DROP COLUMN IF EXISTS upd;
ALTER TABLE accounts DROP COLUMN IF EXISTS ver;
ALTER TABLE timesheets DROP COLUMN IF EXISTS upd;
ALTER TABLE timesheets DROP COLUMN IF EXISTS ver;
ALTER TABLE shifts DROP COLUMN IF EXISTS upd;
ALTER TABLE shifts DROP COLUMN IF EXISTS ver;
ALTER TABLE stock_movements DROP COLUMN IF EXISTS upd;
ALTER TABLE stock_movements DROP COLUMN IF EXISTS ver;
ALTER TABLE stock_levels DROP COLUMN IF EXISTS upd;
ALTER TABLE stock_levels DROP COLUMN IF EXISTS ver;
ALTER TABLE order_items DROP COLUMN IF EXISTS upd;
ALTER TABLE order_items DROP COLUMN IF EXISTS ver;
ALTER TABLE orders DROP COLUMN IF EXISTS upd;
ALTER TABLE orders DROP COLUMN IF EXISTS ver;
ALTER TABLE components DROP COLUMN IF EXISTS upd;
ALTER TABLE components DROP COLUMN IF EXISTS ver;
ALTER TABLE entities DROP COLUMN IF EXISTS upd;
ALTER TABLE entities DROP COLUMN IF EXISTS ver;

-- ============================================================
-- 4. Rename organization_id → workspace_id in all business tables
-- ============================================================

ALTER TABLE entities RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE components RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE orders RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE order_items RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE stock_levels RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE stock_movements RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE shifts RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE timesheets RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE accounts RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE transactions RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE transaction_lines RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE invoices RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE routes RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE route_stops RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE geo_tracks RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE events RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE process_definitions RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE process_instances RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE order_number_sequences RENAME COLUMN organization_id TO workspace_id;

-- ============================================================
-- 5. Rename labors → workspace_members
-- ============================================================

ALTER TABLE labors DROP COLUMN IF EXISTS iat;
ALTER TABLE labors DROP COLUMN IF EXISTS upd;
ALTER TABLE labors DROP COLUMN IF EXISTS ver;
ALTER TABLE labors DROP COLUMN IF EXISTS id;
ALTER TABLE labors DROP COLUMN IF EXISTS admin;
ALTER TABLE labors RENAME COLUMN organization_id TO workspace_id;
ALTER TABLE labors RENAME TO workspace_members;

-- ============================================================
-- 6. Rename organizations → workspaces
-- ============================================================

ALTER TABLE organizations DROP COLUMN IF EXISTS upd;
ALTER TABLE organizations DROP COLUMN IF EXISTS ver;
ALTER TABLE organizations DROP COLUMN IF EXISTS inn;
ALTER TABLE organizations RENAME TO workspaces;

-- ============================================================
-- 7. Drop phones table
-- ============================================================

DROP TABLE IF EXISTS phones;

-- ============================================================
-- 8. Drop new columns from users
-- ============================================================

ALTER TABLE users DROP COLUMN IF EXISTS aat;
ALTER TABLE users DROP COLUMN IF EXISTS secret;
ALTER TABLE users DROP COLUMN IF EXISTS name;
ALTER TABLE users DROP COLUMN IF EXISTS upd;
ALTER TABLE users DROP COLUMN IF EXISTS ver;
