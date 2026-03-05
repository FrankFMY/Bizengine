-- Finance periods (monthly accounting periods)
CREATE TABLE IF NOT EXISTS finance_periods (
    organization_id UUID NOT NULL REFERENCES organizations(id),
    year            INT NOT NULL,
    month           INT NOT NULL CHECK (month >= 1 AND month <= 12),
    status          TEXT NOT NULL DEFAULT 'open',
    closed_at       TIMESTAMPTZ,
    closed_by       UUID,
    PRIMARY KEY (organization_id, year, month)
);

-- Cash operations
CREATE TABLE IF NOT EXISTS cash_operations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    type            TEXT NOT NULL CHECK (type IN ('deposit', 'withdrawal')),
    amount          BIGINT NOT NULL CHECK (amount > 0),
    description     TEXT NOT NULL DEFAULT '',
    account_code    TEXT NOT NULL DEFAULT '50',
    actor_id        UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cash_operations_org ON cash_operations(organization_id);
CREATE INDEX idx_cash_operations_created ON cash_operations(organization_id, created_at DESC);
