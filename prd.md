# PayTrack Query Lab — PRD

> **Type:** SRE / Backend Engineering Portfolio Project  
> **Stack:** Go, PostgreSQL 16, Docker, pgAdmin, k6  
> **Goal:** Simulate real-world fintech query performance degradation and optimize it iteratively.

---

## 1. Problem Statement

In production fintech systems, transaction tables accumulate hundreds of millions of rows. Developers who don't think about query performance early on ship endpoints that work fine at 10k rows but collapse at 50M. This project simulates exactly that environment — giving you a controlled lab to reproduce, measure, and fix slow queries the way an SRE would.

---

## 2. Product Scope (MVP)

**What we're building:**  
A Go REST API backed by a PostgreSQL database with 50 million transaction rows. Three endpoints are deliberately slow at baseline (no indexes). The goal is to optimize them one by one, logging every step.

**What we're NOT building now:**

- Auth / JWT
- Write endpoints (POST/PATCH/DELETE)
- Full pagination UI
- Prometheus / Grafana (planned for Phase 2)

---

## 3. System Architecture

```
                    ┌─────────────────────┐
                    │     Client / k6     │
                    └────────┬────────────┘
                             │ HTTP
                    ┌────────▼────────────┐
                    │   Go API Server     │
                    │   (chi router)      │
                    │   port :8080        │
                    └────────┬────────────┘
                             │ pgx (TCP)
              ┌──────────────▼──────────────────┐
              │         PostgreSQL 16            │
              │         port :5432               │
              │                                  │
              │  tables: users, transactions     │
              │  50M rows (no indexes baseline)  │
              └──────────────────────────────────┘
                             │
              ┌──────────────▼──────────────────┐
              │           pgAdmin 4              │
              │           port :5050             │
              └──────────────────────────────────┘
```

**Infra:** AWS EC2 (t3.medium or higher recommended for seeding)

---

## 4. Database Schema

### `users`

| Column     | Type         | Notes |
| ---------- | ------------ | ----- |
| id         | BIGSERIAL PK |       |
| name       | TEXT         |       |
| email      | TEXT UNIQUE  |       |
| created_at | TIMESTAMPTZ  |       |

10,000 synthetic users generated via Go seeder.

### `transactions`

| Column           | Type                 | Notes                             |
| ---------------- | -------------------- | --------------------------------- |
| id               | BIGSERIAL PK         |                                   |
| user_id          | BIGINT FK → users.id |                                   |
| amount           | NUMERIC(12,2)        |                                   |
| transaction_type | TEXT                 | `'credit'` or `'debit'`           |
| description      | TEXT                 | Synthetic merchant name           |
| created_at       | TIMESTAMPTZ          | Spread across 5 years (2020–2025) |

50,000,000 rows. No indexes beyond PK at baseline.

---

## 5. API Endpoints

### `GET /transactions?user_id=:id`

Returns the last 50 transactions for a user.

**Baseline problem:** Full sequential scan on `user_id` column across 50M rows.  
**Response includes:** `X-Query-Time-Ms` header.

```json
{
  "user_id": 42,
  "count": 50,
  "transactions": [...]
}
```

---

### `GET /transactions?user_id=:id&date=YYYY-MM-DD`

Returns transactions for a user on a specific calendar date.

**Baseline problem:** Full scan + date truncation on every row.  
**Response includes:** `X-Query-Time-Ms` header.

```json
{
  "user_id": 42,
  "date": "2023-06-15",
  "transactions": [...]
}
```

---

### `GET /transactions/summary?user_id=:id`

Returns total credit/debit counts and amounts for a user.

**Baseline problem:** Full aggregation scan with no covering index.  
**Response includes:** `X-Query-Time-Ms` header.

```json
{
  "user_id": 42,
  "total_credited": "124500.00",
  "total_debited": "98320.50",
  "credit_count": 2450,
  "debit_count": 2100
}
```

---

## 6. Query Optimization Roadmap

Each phase is a Git branch + documented finding.

| Phase        | Action                                    | Expected Gain                   |
| ------------ | ----------------------------------------- | ------------------------------- |
| **Baseline** | No indexes, raw timing logged             | —                               |
| **Phase 1**  | `CREATE INDEX ON transactions(user_id)`   | 10–100x for list/summary        |
| **Phase 2**  | Composite: `(user_id, created_at)`        | Eliminates sort for date filter |
| **Phase 3**  | `EXPLAIN ANALYZE` tuning, `work_mem` bump | Fine-tune planner               |
| **Phase 4**  | Connection pooling via PgBouncer          | Reduce connection overhead      |
| **Phase 5**  | Prometheus + Grafana dashboards           | Observability layer             |
| **Phase 6**  | Simulate load scaling (more EC2 replicas) | Read replica + load balance     |

---

## 7. Benchmarking Strategy

Tool: **k6** (load testing)

For each phase:

1. Run `make bench` → baseline p95 latency
2. Apply optimization
3. Run `make bench` → new p95 latency
4. Record in `BENCHMARKS.md`

Targets:

| Endpoint                           | Baseline (expected) | After Phase 1 | After Phase 2 |
| ---------------------------------- | ------------------- | ------------- | ------------- |
| `/transactions?user_id=X`          | >2000ms             | <50ms         | <30ms         |
| `/transactions?user_id=X&date=...` | >3000ms             | <100ms        | <20ms         |
| `/transactions/summary?user_id=X`  | >2500ms             | <80ms         | <40ms         |

---

## 8. Observability

### Now (MVP)

- `X-Query-Time-Ms` header on every response
- Structured stdout logging per request
- `log_min_duration_statement = 100` in postgres config (flags slow queries)
- pgAdmin for visual query plan inspection

### Planned (Phase 5)

- `/metrics` endpoint → Prometheus scrape
- Grafana dashboard: RPS, p95 latency, slow query count, buffer hit ratio

---

## 9. Project Structure

```
paytrack-query-lab/
├── docker-compose.yml
├── postgres/
│   ├── postgresql.conf
│   └── init.sql
├── seed/
│   └── main.go
├── api/
│   ├── main.go
│   ├── handler/
│   │   └── transactions.go
│   └── middleware/
│       └── timing.go
├── bench/
│   └── k6/
│       ├── list_transactions.js
│       ├── filter_by_date.js
│       └── summary.js
├── sql/
│   ├── 01_baseline_explain.sql
│   └── 02_with_index.sql
├── Makefile
├── .env.example
├── prd.md
└── README.md
```

---

## 10. Non-Functional Requirements

| Requirement                  | Target                                            |
| ---------------------------- | ------------------------------------------------- |
| Seed time (EC2 t3.medium)    | < 10 minutes for 50M rows                         |
| API startup time             | < 2 seconds                                       |
| Zero-downtime index creation | `CREATE INDEX CONCURRENTLY`                       |
| Disk usage                   | ~15–20 GB for 50M rows                            |
| Reproducibility              | `make up && make seed && make run` one-shot setup |

---

## 11. Success Criteria for MVP

- [ ] 50M rows seeded successfully
- [ ] All 3 endpoints return correct data with query time headers
- [ ] Baseline benchmarks captured
- [ ] Phase 1 index applied and improvement documented
- [ ] README walkthrough complete
