-- 000001_init.up.sql
-- Placeholder migration for Phase 1 verification
-- Full schemas for users, products, stock_transactions, and reconciliations will be added in subsequent phases.
CREATE TABLE IF NOT EXISTS system_info (
    key VARCHAR(50) PRIMARY KEY,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO system_info (key, value)
VALUES ('phase', 'Phase 1 - Database Foundation')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

