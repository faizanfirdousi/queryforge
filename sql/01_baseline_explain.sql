-- ============================================================
-- QueryForge — Phase 0: BASELINE (no indexes beyond PK)
-- Run these AFTER seeding 50M rows.
-- ============================================================
-- HOW TO RUN:
--   docker exec -it paytrack_postgres psql -U paytrack -d paytrack -f /sql/01_baseline_explain.sql
-- or paste each block manually into psql / pgAdmin.
-- ============================================================

-- Pick a real user_id that actually has data.
-- Quick sanity check first:
SELECT user_id, COUNT(*) AS tx_count
FROM transactions
GROUP BY user_id
ORDER BY tx_count DESC
LIMIT 5;

-- ────────────────────────────────────────────────────────────
-- QUERY 1 — Last 50 transactions for a user (no index)
-- ────────────────────────────────────────────────────────────
-- Replace 42 with any user_id returned above.
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, user_id, amount, transaction_type, description, created_at
FROM transactions
WHERE user_id = 42
ORDER BY created_at DESC
LIMIT 50;

-- What to look for:
--   Seq Scan on transactions → full table scan (~50M rows)
--   "rows removed by filter" will be enormous
--   Execution Time will be > 2000ms

-- ────────────────────────────────────────────────────────────
-- QUERY 2 — Transactions for a user on a specific date (no index)
-- ────────────────────────────────────────────────────────────
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, user_id, amount, transaction_type, description, created_at
FROM transactions
WHERE user_id = 42
  AND DATE(created_at) = '2023-06-15'
ORDER BY created_at DESC;

-- What to look for:
--   Seq Scan again — it must evaluate DATE() on every single row
--   This prevents any future index on created_at from being used
--   Execution Time > 3000ms typically

-- ────────────────────────────────────────────────────────────
-- QUERY 3 — Aggregate totals for a user (no index)
-- ────────────────────────────────────────────────────────────
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'credit'), 0) AS total_credited,
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'debit'),  0) AS total_debited,
    COUNT(*)  FILTER (WHERE transaction_type = 'credit')               AS credit_count,
    COUNT(*)  FILTER (WHERE transaction_type = 'debit')                AS debit_count
FROM transactions
WHERE user_id = 42;

-- What to look for:
--   Seq Scan + Aggregate node
--   Reads every row in the table to compute SUM/COUNT for just one user
--   Execution Time > 2500ms

-- ────────────────────────────────────────────────────────────
-- CHECK: current indexes on transactions
-- ────────────────────────────────────────────────────────────
\d transactions

-- Expected at baseline:
--   Only "transactions_pkey" on (id)
--   NO index on user_id, created_at, or transaction_type
