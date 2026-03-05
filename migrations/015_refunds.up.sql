CREATE TABLE IF NOT EXISTS refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    order_id UUID NOT NULL REFERENCES orders(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    total BIGINT NOT NULL,
    reason TEXT,
    refund_method VARCHAR(20),
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refunds_org ON refunds(organization_id);
CREATE INDEX idx_refunds_order ON refunds(order_id);

CREATE TABLE IF NOT EXISTS refund_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    refund_id UUID NOT NULL REFERENCES refunds(id),
    order_item_id UUID NOT NULL REFERENCES order_items(id),
    quantity INTEGER NOT NULL,
    unit_price BIGINT NOT NULL,
    total BIGINT NOT NULL,
    reason TEXT,
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refund_items_refund ON refund_items(refund_id);
