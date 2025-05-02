package main

import (
	"encoding/json"
	"fmt"
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

	// Initialize layers
	repo := repository.NewRepository(db)
	svc := service.NewService(repo, logger)
	h := handler.NewHandler(svc)
	cbrClient := cbr.NewCBRClient(cfg)

	// Setup router
	r := mux.NewRouter()
	// Public routes
	r.HandleFunc("/register", h.Register).Methods("POST")
	r.HandleFunc("/login", h.Login).Methods("POST")
	// Protected routes
	authRouter := r.PathPrefix("/").Subrouter()
	authRouter.Use(middleware.AuthMiddleware(cfg))
	authRouter.HandleFunc("/accounts", h.CreateAccount).Methods("POST")
	// CBR key rate endpoint
	r.HandleFunc("/key-rate", func(w http.ResponseWriter, r *http.Request) {
		rate, err := cbrClient.GetKeyRate()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get key rate: %v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]float64{"key_rate": rate})
	}).Methods("GET")

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
