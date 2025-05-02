package models

// User represents a user in the system
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // Not serialized
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
