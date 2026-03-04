-- 006_finance: Finance tables (double-entry bookkeeping)

CREATE TABLE accounts (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL,
    code         TEXT NOT NULL,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL,
    parent_id    UUID REFERENCES accounts(id),
    is_system    BOOLEAN NOT NULL DEFAULT FALSE,
    currency     TEXT NOT NULL DEFAULT 'RUB',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, code)
);
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;

CREATE TABLE transactions (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id   UUID NOT NULL,
    date           DATE NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    reference_type TEXT,
    reference_id   UUID,
    is_posted      BOOLEAN NOT NULL DEFAULT FALSE,
    actor_id       UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_txn_ws_date ON transactions(workspace_id, date DESC);
CREATE INDEX idx_txn_reference ON transactions(reference_type, reference_id) WHERE reference_id IS NOT NULL;
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;

CREATE TABLE transaction_lines (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    workspace_id   UUID NOT NULL,
    account_id     UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    debit          BIGINT NOT NULL DEFAULT 0,
    credit         BIGINT NOT NULL DEFAULT 0,
    description    TEXT NOT NULL DEFAULT '',
    entity_id      UUID REFERENCES entities(id)
);
CREATE INDEX idx_tl_transaction ON transaction_lines(transaction_id);
CREATE INDEX idx_tl_account ON transaction_lines(workspace_id, account_id);
ALTER TABLE transaction_lines ENABLE ROW LEVEL SECURITY;

ALTER TABLE transaction_lines ADD CONSTRAINT chk_debit_credit
    CHECK (debit >= 0 AND credit >= 0 AND NOT (debit > 0 AND credit > 0));

CREATE TABLE invoices (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id    UUID NOT NULL,
    number          TEXT NOT NULL,
    type            TEXT NOT NULL,
    counterparty_id UUID REFERENCES entities(id),
    order_id        UUID,
    subtotal        BIGINT NOT NULL DEFAULT 0,
    tax             BIGINT NOT NULL DEFAULT 0,
    total           BIGINT NOT NULL DEFAULT 0,
    currency        TEXT NOT NULL DEFAULT 'RUB',
    status          TEXT NOT NULL DEFAULT 'draft',
    issued_at       DATE,
    due_at          DATE,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, number)
);
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
