-- 021_bank_reconciliations rollback
DROP TRIGGER IF EXISTS trg_ver_upd ON bank_reconciliation_entries;
DROP TRIGGER IF EXISTS trg_ver_upd ON bank_reconciliations;
DROP TABLE IF EXISTS bank_reconciliation_entries;
DROP TABLE IF EXISTS bank_reconciliations;
