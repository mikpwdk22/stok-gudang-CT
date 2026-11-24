package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ctstock/backend/internal/config"
	"ctstock/backend/internal/database"
)

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
}

type DBHealthResponse struct {
	Status    string    `json:"status"`
	Database  string    `json:"database"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

func main() {
	// 1. Load Environment Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting CTStock Backend on port :%s (env: %s)\n", cfg.Port, cfg.Env)

	// 2. Connect to PostgreSQL
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("Warning: Failed to connect to database at startup: %v\n", err)
		log.Println("Server will still start to provide health status. Ensure PostgreSQL is running.")
	} else {
		defer db.Close()

		// Initialize migrations runner
		migrator := database.NewMigrationRunner(db.Pool, "migrations")
		if err := migrator.Up(ctx); err != nil {
			log.Printf("Migration warning: %v\n", err)
		}
	}

	// 3. Set Up HTTP Routes
	mux := http.NewServeMux()

	// GET /health - Basic server liveness check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:    "ok",
			Timestamp: time.Now(),
			Service:   "ctstock-backend",
		})
	})

	// GET /health/db - Database connectivity check
	mux.HandleFunc("/health/db", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if db == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(DBHealthResponse{
				Status:    "unhealthy",
				Database:  "disconnected",
				Timestamp: time.Now(),
				Error:     "database connection not initialized",
			})
			return
		}

		pingCtx, pingCancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer pingCancel()

		if err := db.Ping(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(DBHealthResponse{
				Status:    "unhealthy",
				Database:  "disconnected",
				Timestamp: time.Now(),
				Error:     err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DBHealthResponse{
			Status:    "ok",
			Database:  "connected",
			Timestamp: time.Now(),
		})
	})

	// 4. Configure HTTP Server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 5. Start Server in a separate Goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Server listening on http://localhost:%s\n", cfg.Port)
		serverErrors <- server.ListenAndServe()
	}()

	// 6. Graceful Shutdown Setup
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("Received signal %v: initiating graceful shutdown\n", sig)

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Forced shutdown error: %v\n", err)
			_ = server.Close()
		}

		log.Println("Server gracefully stopped")
	}
}

