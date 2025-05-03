package main

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"os"
	"time"

	"database/sql"

	"github.com/Dan9191/bank-service/internal/config"
	"github.com/Dan9191/bank-service/internal/handler"
	"github.com/Dan9191/bank-service/internal/integrations/cbr"
	"github.com/Dan9191/bank-service/internal/middleware"
	"github.com/Dan9191/bank-service/internal/repository"
	"github.com/Dan9191/bank-service/internal/service"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logLevel, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := sql.Open("postgres", cfg.DBConn)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		logger.Fatalf("Failed to ping database: %v", err)
	}

	// Run migrations
	if err := runMigrations(db, logger); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize layers
	repo := repository.NewRepository(db)
	svc := service.NewService(repo, logger, cfg)
	h := handler.NewHandler(svc)
	cbrClient := cbr.NewCBRClient(cfg, logger)

	// Setup router
	r := mux.NewRouter()
	// Public routes
	r.HandleFunc("/register", h.Register).Methods("POST")
	r.HandleFunc("/login", h.Login).Methods("POST")
	// Test token endpoint
	r.HandleFunc("/test-token", func(w http.ResponseWriter, r *http.Request) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		})
		tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
	}).Methods("GET")
	// CBR key rate endpoint
	r.HandleFunc("/key-rate", func(w http.ResponseWriter, r *http.Request) {
		rate, err := cbrClient.GetKeyRate()
		if err != nil {
			logger.Errorf("Failed to get key rate: %v", err)
			http.Error(w, fmt.Sprintf("Failed to get key rate: %v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]float64{"key_rate": rate})
	}).Methods("GET")
	// Protected routes
	authRouter := r.PathPrefix("/").Subrouter()
	authRouter.Use(middleware.AuthMiddleware(cfg))
	authRouter.HandleFunc("/accounts", h.CreateAccount).Methods("POST")
	authRouter.HandleFunc("/cards", h.CreateCard).Methods("POST")
	authRouter.HandleFunc("/cards", h.ListCards).Methods("GET")
	authRouter.HandleFunc("/transactions/deposit", h.Deposit).Methods("POST")
	authRouter.HandleFunc("/transactions/withdraw", h.Withdraw).Methods("POST")
	authRouter.HandleFunc("/transactions/transfer", h.Transfer).Methods("POST")
	authRouter.HandleFunc("/transactions", h.ListTransactions).Methods("GET")

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	logger.Infof("Starting server on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Server failed: %v", err)
	}
}

func runMigrations(db *sql.DB, logger *logrus.Logger) error {
	logger.Debug("Creating schema bank")
	_, err := db.Exec("CREATE SCHEMA IF NOT EXISTS bank")
	if err != nil {
		return fmt.Errorf("failed to create schema bank: %w", err)
	}

	logger.Debug("Creating table bank.users")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bank.users (
			id BIGSERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create bank.users table: %w", err)
	}

	logger.Debug("Creating table bank.accounts")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bank.accounts (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT REFERENCES bank.users(id) ON DELETE CASCADE,
			balance NUMERIC(15, 2) DEFAULT 0.0,
			currency VARCHAR(3) DEFAULT 'RUB',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create bank.accounts table: %w", err)
	}

	logger.Debug("Creating table bank.cards")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bank.cards (
			id BIGSERIAL PRIMARY KEY,
			account_id BIGINT REFERENCES bank.accounts(id) ON DELETE CASCADE,
			card_number TEXT NOT NULL,
			expiry_date TEXT NOT NULL,
			cvv_hash TEXT NOT NULL,
			hmac TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create bank.cards table: %w", err)
	}

	logger.Debug("Creating table bank.transactions")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bank.transactions (
			id BIGSERIAL PRIMARY KEY,
			account_id BIGINT REFERENCES bank.accounts(id) ON DELETE CASCADE,
			amount NUMERIC(15, 2) NOT NULL,
			type VARCHAR(50) NOT NULL,
			description TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create bank.transactions table: %w", err)
	}

	logger.Debug("Creating table bank.credits")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bank.credits (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT REFERENCES bank.users(id) ON DELETE CASCADE,
			account_id BIGINT REFERENCES bank.accounts(id) ON DELETE CASCADE,
			amount NUMERIC(15, 2) NOT NULL,
			interest_rate NUMERIC(5, 2) NOT NULL,
			term_months INTEGER NOT NULL,
			hmac TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create bank.credits table: %w", err)
	}

	logger.Debug("Creating table bank.payment_schedules")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bank.payment_schedules (
			id BIGSERIAL PRIMARY KEY,
			credit_id BIGINT REFERENCES bank.credits(id) ON DELETE CASCADE,
			payment_date DATE NOT NULL,
			amount NUMERIC(15, 2) NOT NULL,
			paid BOOLEAN DEFAULT FALSE,
			penalty NUMERIC(15, 2) DEFAULT 0.0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create bank.payment_schedules table: %w", err)
	}

	logger.Info("Database migrations completed successfully")
	return nil
}
