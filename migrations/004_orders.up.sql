-- 004_orders: Orders, order items, number sequences

CREATE TABLE IF NOT EXISTS orders (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id   UUID NOT NULL,
    entity_id      UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    number         TEXT NOT NULL,
    customer_id    UUID REFERENCES entities(id),
    status         TEXT NOT NULL DEFAULT 'draft',
    subtotal       BIGINT NOT NULL DEFAULT 0,
    discount       BIGINT NOT NULL DEFAULT 0,
    tax            BIGINT NOT NULL DEFAULT 0,
    total          BIGINT NOT NULL DEFAULT 0,
    currency       TEXT NOT NULL DEFAULT 'RUB',
    notes          TEXT NOT NULL DEFAULT '',
    source         TEXT NOT NULL DEFAULT 'manual',
    warehouse_id   UUID REFERENCES entities(id),
    assigned_to    UUID REFERENCES users(id),
    paid_at        TIMESTAMPTZ,
    shipped_at     TIMESTAMPTZ,
    delivered_at   TIMESTAMPTZ,
    cancelled_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, number)
);
CREATE INDEX IF NOT EXISTS idx_orders_ws_status ON orders(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders(workspace_id, customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_orders_ws_created ON orders(workspace_id, created_at DESC);
ALTER TABLE orders ENABLE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS order_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id     UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    product_id   UUID NOT NULL REFERENCES entities(id) ON DELETE RESTRICT,
    name         TEXT NOT NULL,
    sku          TEXT NOT NULL DEFAULT '',
    quantity     NUMERIC(15,4) NOT NULL,
    unit         TEXT NOT NULL DEFAULT 'шт',
    unit_price   BIGINT NOT NULL,
    discount     BIGINT NOT NULL DEFAULT 0,
    tax          BIGINT NOT NULL DEFAULT 0,
    total        BIGINT NOT NULL,
    sort_order   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_oi_order ON order_items(order_id);
ALTER TABLE order_items ENABLE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS order_number_sequences (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    last_number  BIGINT NOT NULL DEFAULT 0
);
