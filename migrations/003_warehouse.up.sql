-- 003_warehouse: Stock levels and movements

CREATE TABLE IF NOT EXISTS stock_levels (
    workspace_id  UUID NOT NULL,
    product_id    UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    warehouse_id  UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    quantity      NUMERIC(15,4) NOT NULL DEFAULT 0,
    reserved      NUMERIC(15,4) NOT NULL DEFAULT 0,
    unit          TEXT NOT NULL DEFAULT 'шт',
    min_quantity  NUMERIC(15,4) NOT NULL DEFAULT 0,
    max_quantity  NUMERIC(15,4),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, product_id, warehouse_id)
);
CREATE INDEX IF NOT EXISTS idx_sl_low ON stock_levels(workspace_id)
    WHERE quantity - reserved <= min_quantity AND min_quantity > 0;
CREATE INDEX IF NOT EXISTS idx_sl_warehouse ON stock_levels(workspace_id, warehouse_id);
ALTER TABLE stock_levels ENABLE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS stock_movements (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id     UUID NOT NULL,
    product_id       UUID NOT NULL REFERENCES entities(id) ON DELETE RESTRICT,
    warehouse_id     UUID NOT NULL REFERENCES entities(id) ON DELETE RESTRICT,
    type             TEXT NOT NULL,
    quantity         NUMERIC(15,4) NOT NULL,
    unit             TEXT NOT NULL DEFAULT 'шт',
    cost_per_unit    BIGINT,
    reason           TEXT NOT NULL DEFAULT '',
    reference_type   TEXT,
    reference_id     UUID,
    dest_warehouse_id UUID REFERENCES entities(id),
    actor_id         UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sm_product ON stock_movements(workspace_id, product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sm_warehouse ON stock_movements(workspace_id, warehouse_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sm_reference ON stock_movements(reference_type, reference_id) WHERE reference_id IS NOT NULL;
ALTER TABLE stock_movements ENABLE ROW LEVEL SECURITY;
