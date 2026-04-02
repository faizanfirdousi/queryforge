package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/faizanfirdousi/queryforge/api/handler"
	"github.com/faizanfirdousi/queryforge/api/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// read db url from env
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL env variable is not set ")
	}

	// create a connection pool
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	//verify the connection if it works
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}
	log.Println("✓ Database Connected")

	// handler to inject the db pool
	h := handler.New(pool)

	// 5. Setup router and routes
	r := chi.NewRouter()
	r.Use(middleware.Timer) // log every request: method, path, status, duration
	r.Get("/transactions", h.ListTransactions)
	r.Get("/transactions/summary", h.GetSummary)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("✓ Server starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
