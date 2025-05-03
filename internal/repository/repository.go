package repository

import (
	"context"
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

// CreateTransaction creates a new transaction and updates account balance
func (r *Repository) CreateTransaction(tx *sql.Tx, transaction *models.Transaction) error {
	// Insert transaction
	query := `
		INSERT INTO bank.transactions (
			account_id,
			amount,
			type,
			description,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := tx.QueryRow(
		query,
		transaction.AccountID,
		transaction.Amount,
		transaction.Type,
		transaction.Description,
	).Scan(&transaction.ID, &transaction.CreatedAt, &transaction.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Update account balance
	updateQuery := `
		UPDATE bank.accounts
		SET balance = balance + $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`
	_, err = tx.Exec(updateQuery, transaction.Amount, transaction.AccountID)
	if err != nil {
		return fmt.Errorf("failed to update account balance: %w", err)
	}

	return nil
}

// GetAccountBalance retrieves the current balance of an account
func (r *Repository) GetAccountBalance(accountID int64) (float64, error) {
	var balance float64
	query := `SELECT balance FROM bank.accounts WHERE id = $1`
	err := r.db.QueryRow(query, accountID).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("account not found")
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get account balance: %w", err)
	}
	return balance, nil
}

// Deposit adds funds to an account
func (r *Repository) Deposit(ctx context.Context, transaction *models.Transaction) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := r.CreateTransaction(tx, transaction); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Withdraw removes funds from an account
func (r *Repository) Withdraw(ctx context.Context, transaction *models.Transaction) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := r.CreateTransaction(tx, transaction); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Transfer moves funds between accounts
func (r *Repository) Transfer(ctx context.Context, withdrawal, deposit *models.Transaction) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := r.CreateTransaction(tx, withdrawal); err != nil {
		return err
	}
	if err := r.CreateTransaction(tx, deposit); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// ListTransactions retrieves a list of transactions for an account
func (r *Repository) ListTransactions(accountID int64, transactionType string, limit, offset int) ([]*models.Transaction, error) {
	query := `
		SELECT id, account_id, amount, type, description, created_at, updated_at
		FROM bank.transactions
		WHERE account_id = $1`
	args := []interface{}{accountID}

	if transactionType != "" {
		query += ` AND type = $2`
		args = append(args, transactionType)
	}

	query += ` ORDER BY created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		tx := &models.Transaction{}
		err := rows.Scan(
			&tx.ID,
			&tx.AccountID,
			&tx.Amount,
			&tx.Type,
			&tx.Description,
			&tx.CreatedAt,
			&tx.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transactions: %w", err)
	}

	return transactions, nil
}
