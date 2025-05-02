package models

// User represents a user in the system
type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // Not serialized
	CreatedAt    string `json:"created_at"`
}
