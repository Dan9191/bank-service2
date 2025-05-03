package models

import "time"

// Credit represents a credit in the system
type Credit struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	AccountID    int64     `json:"account_id"`
	Amount       float64   `json:"amount"`
	InterestRate float64   `json:"interest_rate"`
	TermMonths   int       `json:"term_months"`
	HMAC         string    `json:"hmac"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
