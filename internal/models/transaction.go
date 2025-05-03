package models

// Transaction represents a financial transaction
type Transaction struct {
	ID          int64   `json:"id"`
	AccountID   int64   `json:"account_id"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}
