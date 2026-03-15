-- 020_bank_partners: Multi-bank architecture

CREATE TABLE bank_partners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    code VARCHAR(20) NOT NULL UNIQUE,
    api_base_url TEXT,
    credentials JSONB DEFAULT '{}',
    settings JSONB DEFAULT '{}',
    white_label JSONB DEFAULT '{}',
    active BOOLEAN DEFAULT true,
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE organizations ADD COLUMN IF NOT EXISTS bank_partner_id UUID REFERENCES bank_partners(id);
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS bank_client_id VARCHAR(100);
CREATE INDEX IF NOT EXISTS idx_org_bank ON organizations(bank_partner_id) WHERE bank_partner_id IS NOT NULL;

-- auto_ver_upd trigger
DROP TRIGGER IF EXISTS trg_ver_upd ON bank_partners;
CREATE TRIGGER trg_ver_upd BEFORE UPDATE ON bank_partners FOR EACH ROW EXECUTE FUNCTION auto_ver_upd();
