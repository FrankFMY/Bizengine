CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'info',
    read BOOLEAN NOT NULL DEFAULT false,
    reference_type VARCHAR(50),
    reference_id UUID,
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user ON notifications(organization_id, user_id, read);
