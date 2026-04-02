package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// Transaction represents one row from the transactions table.
// Note: Go convention is UserID not UserId (acronyms are all-caps)
type Transaction struct {
	ID              int64  `json:"id"`
	UserID          int64  `json:"user_id"`
	Amount          string `json:"amount"`
	TransactionType string `json:"transaction_type"`
	Description     string `json:"description"`
	CreatedAt       string `json:"created_at"`
}

// ListTransactions handles:
//
//	GET /transactions?user_id=X          → last 50 transactions (Query 1)
//	GET /transactions?user_id=X&date=... → filter by date        (Query 2)
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "user_id must be a number", http.StatusBadRequest)
		return
	}

	start := time.Now()

	// pgx.Rows is the interface returned by any pgx Query call
	var rows pgx.Rows
	dateStr := r.URL.Query().Get("date")

	if dateStr != "" {
		// Query 2: filter by user + specific calendar date
		// DATE(created_at) strips the time component for comparison
		rows, err = h.db.Query(context.Background(), `
			SELECT id, user_id, amount, transaction_type, description, created_at
			FROM transactions
			WHERE user_id = $1
			  AND DATE(created_at) = $2
			ORDER BY created_at DESC
		`, userID, dateStr)
	} else {
		// Query 1: most recent 50 transactions for this user
		rows, err = h.db.Query(context.Background(), `
			SELECT id, user_id, amount, transaction_type, description, created_at
			FROM transactions
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT 50
		`, userID)
	}

	// Capture query time immediately after the DB call returns
	queryMs := time.Since(start).Milliseconds()

	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var createdAt time.Time
		// Scan maps each column in SELECT order into the struct fields
		if err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.TransactionType, &t.Description, &createdAt); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		t.CreatedAt = createdAt.Format(time.RFC3339)
		transactions = append(transactions, t)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Query-Time-Ms", fmt.Sprintf("%d", queryMs))

	json.NewEncoder(w).Encode(map[string]any{
		"user_id":      userID,
		"count":        len(transactions),
		"query_ms":     queryMs,
		"transactions": transactions,
	})
}

// GetSummary handles:
//
//	GET /transactions/summary?user_id=X → aggregate totals (Query 3)
func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "user_id must be a number", http.StatusBadRequest)
		return
	}

	start := time.Now()

	// Query 3: aggregate — SUM and COUNT grouped by credit/debit
	// This is the most expensive query at baseline: full scan + aggregation
	row := h.db.QueryRow(context.Background(), `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'credit'), 0),
			COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'debit'), 0),
			COUNT(*) FILTER (WHERE transaction_type = 'credit'),
			COUNT(*) FILTER (WHERE transaction_type = 'debit')
		FROM transactions
		WHERE user_id = $1
	`, userID)

	queryMs := time.Since(start).Milliseconds()

	var totalCredited, totalDebited float64
	var creditCount, debitCount int64

	if err := row.Scan(&totalCredited, &totalDebited, &creditCount, &debitCount); err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Query-Time-Ms", fmt.Sprintf("%d", queryMs))

	json.NewEncoder(w).Encode(map[string]any{
		"user_id":        userID,
		"total_credited": fmt.Sprintf("%.2f", totalCredited),
		"total_debited":  fmt.Sprintf("%.2f", totalDebited),
		"credit_count":   creditCount,
		"debit_count":    debitCount,
		"query_ms":       queryMs,
	})
}
