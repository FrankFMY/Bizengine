-- 021_bank_reconciliations: Bank statement reconciliation tables

CREATE TABLE bank_reconciliations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    bank_partner_id UUID REFERENCES bank_partners(id),
    date_from DATE NOT NULL,
    date_to DATE NOT NULL,
    total_entries INTEGER NOT NULL DEFAULT 0,
    matched INTEGER NOT NULL DEFAULT 0,
    unmatched INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'completed',
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bank_recon_org ON bank_reconciliations(organization_id);

CREATE TABLE bank_reconciliation_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reconciliation_id UUID NOT NULL REFERENCES bank_reconciliations(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    statement_entry JSONB NOT NULL,
    matched_type VARCHAR(30),
    matched_id UUID,
    transaction_id UUID REFERENCES transactions(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bank_recon_entry_recon ON bank_reconciliation_entries(reconciliation_id);
CREATE INDEX idx_bank_recon_entry_org ON bank_reconciliation_entries(organization_id);

-- auto_ver_upd triggers
DROP TRIGGER IF EXISTS trg_ver_upd ON bank_reconciliations;
CREATE TRIGGER trg_ver_upd BEFORE UPDATE ON bank_reconciliations FOR EACH ROW EXECUTE FUNCTION auto_ver_upd();

DROP TRIGGER IF EXISTS trg_ver_upd ON bank_reconciliation_entries;
CREATE TRIGGER trg_ver_upd BEFORE UPDATE ON bank_reconciliation_entries FOR EACH ROW EXECUTE FUNCTION auto_ver_upd();
