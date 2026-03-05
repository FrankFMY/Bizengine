-- Organization settings
CREATE TABLE IF NOT EXISTS organization_settings (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id),
    currency        VARCHAR(3) NOT NULL DEFAULT 'RUB',
    timezone        VARCHAR(50) NOT NULL DEFAULT 'Europe/Moscow',
    order_number_format VARCHAR(50) NOT NULL DEFAULT 'ORD-{SEQ}',
    logo_file_id    UUID,
    requisites      JSONB NOT NULL DEFAULT '{}',
    integrations    JSONB NOT NULL DEFAULT '{}',
    features        JSONB NOT NULL DEFAULT '{}',
    ver             INTEGER NOT NULL DEFAULT 1,
    upd             TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat             TIMESTAMPTZ NOT NULL DEFAULT now()
);
