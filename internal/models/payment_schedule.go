package models

import "time"

// PaymentSchedule represents a scheduled payment for a credit
type PaymentSchedule struct {
	ID          int64     `json:"id"`
	CreditID    int64     `json:"credit_id"`
	PaymentDate time.Time `json:"payment_date"`
	Amount      float64   `json:"amount"`
	Paid        bool      `json:"paid"`
	Penalty     float64   `json:"penalty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
