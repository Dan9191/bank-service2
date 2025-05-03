package repository

import (
	"database/sql"
	"fmt"

	"github.com/Dan9191/bank-service/internal/models"
)

// Repository provides database operations
type Repository struct {
	db *sql.DB
}

// NewRepository initializes a new repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateUser creates a new user in the database
func (r *Repository) CreateUser(user *models.User) error {
	query := `
		INSERT INTO bank.users (username, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRow(query, user.Username, user.Email, user.PasswordHash).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// FindUserByEmail retrieves a user by email
func (r *Repository) FindUserByEmail(email string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM bank.users
		WHERE email = $1`
	err := r.db.QueryRow(query, email).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	return user, nil
}

// CreateAccount creates a new account in the database
func (r *Repository) CreateAccount(account *models.Account) error {
	query := `
		INSERT INTO bank.accounts (user_id, balance, currency, created_at, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRow(query, account.UserID, account.Balance, account.Currency).
		Scan(&account.ID, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

// FindAccountByID retrieves the user_id for an account by its ID
func (r *Repository) FindAccountByID(accountID int64) (int64, error) {
	var userID int64
	query := `SELECT user_id FROM bank.accounts WHERE id = $1`
	err := r.db.QueryRow(query, accountID).Scan(&userID)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("account not found")
	}
	if err != nil {
		return 0, fmt.Errorf("failed to find account: %w", err)
	}
	return userID, nil
}

// CreateCard creates a new card in the database
func (r *Repository) CreateCard(card *models.Card) error {
	query := `
		INSERT INTO bank.cards (
			account_id,
			card_number,
			expiry_date,
			cvv_hash,
			hmac,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRow(
		query,
		card.AccountID,
		card.CardNumber,
		card.ExpiryDate,
		card.CVV,
		card.HMAC,
	).Scan(&card.ID, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create card: %w", err)
	}
	return nil
}
