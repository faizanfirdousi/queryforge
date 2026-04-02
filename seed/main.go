package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config
const (
	numUsers        = 10_000
	numTransactions = 50_000_000
	batchSize       = 10_000 // rows per COPY batch
)

var (
	txTypes     = []string{"credit", "debit"}
	merchants   = []string{"Amazon", "Uber", "Spotify", "Netflix", "Zomato", "Swiggy", "Flipkart", "Apple", "Google", "Steam", "PayPal", "Razorpay", "PhonePe", "HDFC", "ICICI", "Airtel", "Jio", "BSNL", "Ola", "Rapido"}
	startDate   = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	dateRange   = int64(5 * 365 * 24 * time.Hour) // 5 years in nanoseconds
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL env variable is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("unable to connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping failed: %v", err)
	}
	log.Println("✓ Connected to database")

	// ── 1. Seed users ──────────────────────────────────────────────────────
	log.Printf("Seeding %d users...", numUsers)
	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Fatalf("acquire conn: %v", err)
	}

	_, err = conn.Exec(ctx, "TRUNCATE transactions, users RESTART IDENTITY CASCADE")
	if err != nil {
		log.Fatalf("truncate: %v", err)
	}

	userRows := make([][]any, numUsers)
	for i := 0; i < numUsers; i++ {
		userRows[i] = []any{
			fmt.Sprintf("User %d", i+1),
			fmt.Sprintf("user%d@paytrack.dev", i+1),
		}
	}

	_, err = conn.Conn().CopyFrom(
		ctx,
		pgx.Identifier{"users"},
		[]string{"name", "email"},
		pgx.CopyFromRows(userRows),
	)
	if err != nil {
		log.Fatalf("copy users: %v", err)
	}
	log.Printf("✓ %d users seeded", numUsers)

	// ── 2. Seed transactions in batches ────────────────────────────────────
	log.Printf("Seeding %d transactions in batches of %d...", numTransactions, batchSize)
	log.Println("  (this will take a few minutes — grab a coffee ☕)")

	rng := rand.New(rand.NewSource(42))
	total := 0
	batchNum := 0
	tickAt := numTransactions / 20 // print progress every 5%

	for total < numTransactions {
		remaining := numTransactions - total
		size := batchSize
		if remaining < size {
			size = remaining
		}

		rows := make([][]any, size)
		for i := 0; i < size; i++ {
			userID := int64(rng.Intn(numUsers) + 1)
			amount := fmt.Sprintf("%.2f", rng.Float64()*9990+10) // 10.00 – 10000.00
			txType := txTypes[rng.Intn(2)]
			desc := merchants[rng.Intn(len(merchants))]
			offset := time.Duration(rng.Int63n(dateRange))
			createdAt := startDate.Add(offset)

			rows[i] = []any{userID, amount, txType, desc, createdAt}
		}

		_, err = conn.Conn().CopyFrom(
			ctx,
			pgx.Identifier{"transactions"},
			[]string{"user_id", "amount", "transaction_type", "description", "created_at"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			log.Fatalf("copy batch %d: %v", batchNum, err)
		}

		total += size
		batchNum++

		if total%tickAt == 0 || total == numTransactions {
			pct := float64(total) / float64(numTransactions) * 100
			log.Printf("  → %8d / %d rows  (%.0f%%)", total, numTransactions, pct)
		}
	}

	conn.Release()

	log.Printf("✓ Done! %d transactions seeded.", numTransactions)
	log.Println("  Run ANALYZE on the tables to update planner statistics:")
	log.Println("  ANALYZE users; ANALYZE transactions;")
}
