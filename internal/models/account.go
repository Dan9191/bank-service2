package models

type Account struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"user_id"`
	Balance   float64 `json:"balance"`
	Currency  string  `json:"currency"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}
