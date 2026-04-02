-- ============================================================
-- QueryForge — Phase 1 & 2: INDEX OPTIMIZATION
-- Apply AFTER capturing baseline results from 01_baseline_explain.sql
-- ============================================================

-- ────────────────────────────────────────────────────────────
-- PHASE 1 — Simple index on user_id
-- ────────────────────────────────────────────────────────────
-- Use CONCURRENTLY so it doesn't lock the table (safe for production)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_user_id
    ON transactions(user_id);

-- Verify it was created:
\d transactions

-- Now re-run Query 1 — should be dramatically faster:
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, user_id, amount, transaction_type, description, created_at
FROM transactions
WHERE user_id = 42
ORDER BY created_at DESC
LIMIT 50;

-- What to look for NOW:
--   Index Scan using idx_transactions_user_id → hits only ~5000 rows (avg per user)
--   Execution Time < 50ms typically

-- Re-run Query 3 (summary) — also benefits from user_id index:
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'credit'), 0),
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'debit'),  0),
    COUNT(*)  FILTER (WHERE transaction_type = 'credit'),
    COUNT(*)  FILTER (WHERE transaction_type = 'debit')
FROM transactions
WHERE user_id = 42;

-- ────────────────────────────────────────────────────────────
-- PHASE 2 — Composite index: (user_id, created_at DESC)
-- Optimizes ORDER BY + date-range lookups in one shot
-- ────────────────────────────────────────────────────────────
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_user_id_created_at
    ON transactions(user_id, created_at DESC);

-- Query 1 re-run with composite index:
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, user_id, amount, transaction_type, description, created_at
FROM transactions
WHERE user_id = 42
ORDER BY created_at DESC
LIMIT 50;

-- What to look for:
--   Index Scan using idx_transactions_user_id_created_at
--   Sort node GONE — index already delivers rows in order (Index Only Scan possible)
--   Execution Time < 5ms

-- Query 2 re-run — fix the function blocker with range predicate instead of DATE()
-- NOTE: DATE(created_at) = 'X' is not index-friendly.
-- Rewrite it as a range to use the index:
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, user_id, amount, transaction_type, description, created_at
FROM transactions
WHERE user_id = 42
  AND created_at >= '2023-06-15 00:00:00'
  AND created_at <  '2023-06-16 00:00:00'
ORDER BY created_at DESC;

-- What to look for:
--   Index Range Scan using composite index
--   Zero rows removed by filter — extremely tight scan
--   Execution Time < 10ms

-- ────────────────────────────────────────────────────────────
-- PHASE 2b — Covering index for summary query
-- Includes all columns the aggregate reads → possible Index Only Scan
-- ────────────────────────────────────────────────────────────
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_user_covering
    ON transactions(user_id)
    INCLUDE (amount, transaction_type);

EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'credit'), 0),
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'debit'),  0),
    COUNT(*)  FILTER (WHERE transaction_type = 'credit'),
    COUNT(*)  FILTER (WHERE transaction_type = 'debit')
FROM transactions
WHERE user_id = 42;

-- What to look for:
--   "Index Only Scan" — never touches heap at all!
--   Execution Time < 20ms

-- ────────────────────────────────────────────────────────────
-- Summary: list all indexes on transactions
-- ────────────────────────────────────────────────────────────
SELECT
    indexname,
    indexdef
FROM pg_indexes
WHERE tablename = 'transactions'
ORDER BY indexname;
