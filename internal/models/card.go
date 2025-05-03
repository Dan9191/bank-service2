package models

// Card represents a bank card
type Card struct {
	ID         int64  `json:"id"`
	AccountID  int64  `json:"account_id"`
	CardNumber string `json:"card_number"` // Decrypted for response
	ExpiryDate string `json:"expiry_date"` // Decrypted for response
	CVV        string `json:"-"`           // Not serialized
	HMAC       string `json:"hmac"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
