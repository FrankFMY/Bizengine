-- 011_audit_fixes: Weighted average cost, double-entry constraint, event partitions

-- I1: Add cost_per_unit to stock_levels for weighted average tracking
ALTER TABLE stock_levels ADD COLUMN IF NOT EXISTS cost_per_unit BIGINT;

-- N4: Trigger to enforce double-entry balance on transaction posting
CREATE OR REPLACE FUNCTION check_double_entry_balance()
RETURNS TRIGGER AS $$
DECLARE
    balance BIGINT;
BEGIN
    SELECT SUM(debit) - SUM(credit) INTO balance
    FROM transaction_lines
    WHERE transaction_id = NEW.id;

    IF balance IS NULL OR balance != 0 THEN
        RAISE EXCEPTION 'Transaction % is unbalanced: SUM(debit) != SUM(credit)', NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_check_double_entry_balance
    BEFORE UPDATE OF is_posted ON transactions
    FOR EACH ROW
    WHEN (NEW.is_posted = TRUE AND OLD.is_posted = FALSE)
    EXECUTE FUNCTION check_double_entry_balance();

-- N5: Monthly partitions for events table (2026)
CREATE TABLE IF NOT EXISTS events_2026_01 PARTITION OF events FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE IF NOT EXISTS events_2026_02 PARTITION OF events FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE IF NOT EXISTS events_2026_03 PARTITION OF events FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE IF NOT EXISTS events_2026_04 PARTITION OF events FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS events_2026_05 PARTITION OF events FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS events_2026_06 PARTITION OF events FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS events_2026_07 PARTITION OF events FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE IF NOT EXISTS events_2026_08 PARTITION OF events FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE IF NOT EXISTS events_2026_09 PARTITION OF events FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE IF NOT EXISTS events_2026_10 PARTITION OF events FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE IF NOT EXISTS events_2026_11 PARTITION OF events FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE IF NOT EXISTS events_2026_12 PARTITION OF events FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

-- N5: Quarterly partitions for geo_tracks (2026)
CREATE TABLE IF NOT EXISTS geo_tracks_2026_q1 PARTITION OF geo_tracks FOR VALUES FROM ('2026-01-01') TO ('2026-04-01');
CREATE TABLE IF NOT EXISTS geo_tracks_2026_q2 PARTITION OF geo_tracks FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS geo_tracks_2026_q3 PARTITION OF geo_tracks FOR VALUES FROM ('2026-07-01') TO ('2026-10-01');
CREATE TABLE IF NOT EXISTS geo_tracks_2026_q4 PARTITION OF geo_tracks FOR VALUES FROM ('2026-10-01') TO ('2027-01-01');
