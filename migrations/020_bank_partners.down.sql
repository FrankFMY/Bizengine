-- 020_bank_partners rollback
DROP TRIGGER IF EXISTS trg_ver_upd ON bank_partners;
ALTER TABLE organizations DROP COLUMN IF EXISTS bank_client_id;
ALTER TABLE organizations DROP COLUMN IF EXISTS bank_partner_id;
DROP TABLE IF EXISTS bank_partners;
