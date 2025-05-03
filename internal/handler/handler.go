package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Dan9191/bank-service/internal/service"
)

// Handler manages HTTP requests
type Handler struct {
	svc *service.Service
}

// NewHandler initializes a new handler
func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Register handles user registration
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.svc.Register(req.Username, req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// Login handles user authentication
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// CreateAccount handles account creation
func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Currency string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Currency == "" {
		req.Currency = "RUB"
	}

	account, err := h.svc.CreateAccount(r.Context(), req.Currency)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(account)
}

// CreateCard handles card creation
func (h *Handler) CreateCard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID int64 `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	card, err := h.svc.CreateCard(r.Context(), req.AccountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(card)
}

// Deposit handles depositing funds to an account
func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID int64   `json:"account_id"`
		Amount    float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	transaction, err := h.svc.Deposit(r.Context(), req.AccountID, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(transaction)
}

// Withdraw handles withdrawing funds from an account
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID int64   `json:"account_id"`
		Amount    float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	transaction, err := h.svc.Withdraw(r.Context(), req.AccountID, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(transaction)
}

// Transfer handles transferring funds between accounts
func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromAccountID int64   `json:"from_account_id"`
		ToAccountID   int64   `json:"to_account_id"`
		Amount        float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	transactions, err := h.svc.Transfer(r.Context(), req.FromAccountID, req.ToAccountID, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(transactions)
}
