-- 011_audit_fixes: Rollback

-- N5: Drop geo_tracks partitions
DROP TABLE IF EXISTS geo_tracks_2026_q4;
DROP TABLE IF EXISTS geo_tracks_2026_q3;
DROP TABLE IF EXISTS geo_tracks_2026_q2;
DROP TABLE IF EXISTS geo_tracks_2026_q1;

-- N5: Drop events partitions
DROP TABLE IF EXISTS events_2026_12;
DROP TABLE IF EXISTS events_2026_11;
DROP TABLE IF EXISTS events_2026_10;
DROP TABLE IF EXISTS events_2026_09;
DROP TABLE IF EXISTS events_2026_08;
DROP TABLE IF EXISTS events_2026_07;
DROP TABLE IF EXISTS events_2026_06;
DROP TABLE IF EXISTS events_2026_05;
DROP TABLE IF EXISTS events_2026_04;
DROP TABLE IF EXISTS events_2026_03;
DROP TABLE IF EXISTS events_2026_02;
DROP TABLE IF EXISTS events_2026_01;

-- N4: Drop double-entry trigger
DROP TRIGGER IF EXISTS trg_check_double_entry_balance ON transactions;
DROP FUNCTION IF EXISTS check_double_entry_balance();

-- I1: Remove cost_per_unit from stock_levels
ALTER TABLE stock_levels DROP COLUMN IF EXISTS cost_per_unit;
