-- Inventory (stocktaking) sessions
CREATE TABLE IF NOT EXISTS inventories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    warehouse_id    UUID NOT NULL,
    status          TEXT NOT NULL DEFAULT 'in_progress',
    notes           TEXT NOT NULL DEFAULT '',
    actor_id        UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    applied_at      TIMESTAMPTZ
);

CREATE INDEX idx_inventories_org ON inventories(organization_id);
CREATE INDEX idx_inventories_warehouse ON inventories(organization_id, warehouse_id);

-- Inventory items (per-product count)
CREATE TABLE IF NOT EXISTS inventory_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id    UUID NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    product_id      UUID NOT NULL,
    expected        DOUBLE PRECISION NOT NULL DEFAULT 0,
    actual          DOUBLE PRECISION,
    discrepancy     DOUBLE PRECISION NOT NULL DEFAULT 0,
    counted_at      TIMESTAMPTZ
);

CREATE INDEX idx_inventory_items_inv ON inventory_items(inventory_id);
CREATE INDEX idx_inventory_items_product ON inventory_items(inventory_id, product_id);
