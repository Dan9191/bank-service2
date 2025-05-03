package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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

// ListCards retrieves a list of cards for a user or specific account
func (r *Repository) ListCards(userID, accountID int64, limit, offset int) ([]*models.Card, error) {
	query := `
		SELECT c.id, c.account_id, c.card_number, c.expiry_date, c.cvv_hash, c.hmac, c.created_at, c.updated_at
		FROM bank.cards c
		JOIN bank.accounts a ON c.account_id = a.id
		WHERE a.user_id = $1`
	args := []interface{}{userID}

	if accountID != 0 {
		query += ` AND c.account_id = $2`
		args = append(args, accountID)
	}

	query += ` ORDER BY c.created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list cards: %w", err)
	}
	defer rows.Close()

	var cards []*models.Card
	for rows.Next() {
		card := &models.Card{}
		err := rows.Scan(
			&card.ID,
			&card.AccountID,
			&card.CardNumber,
			&card.ExpiryDate,
			&card.CVV,
			&card.HMAC,
			&card.CreatedAt,
			&card.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan card: %w", err)
		}
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating cards: %w", err)
	}

	return cards, nil
}

// CreateCredit creates a new credit in the database
func (r *Repository) CreateCredit(credit *models.Credit) error {
	query := `
		INSERT INTO bank.credits (
			user_id,
			account_id,
			amount,
			interest_rate,
			term_months,
			hmac,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRow(
		query,
		credit.UserID,
		credit.AccountID,
		credit.Amount,
		credit.InterestRate,
		credit.TermMonths,
		credit.HMAC,
	).Scan(&credit.ID, &credit.CreatedAt, &credit.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create credit: %w", err)
	}
	return nil
}

// FindCreditByID retrieves a credit by its ID
func (r *Repository) FindCreditByID(creditID int64) (*models.Credit, error) {
	credit := &models.Credit{}
	query := `
		SELECT id, user_id, account_id, amount, interest_rate, term_months, hmac, created_at, updated_at
		FROM bank.credits
		WHERE id = $1`
	err := r.db.QueryRow(query, creditID).Scan(
		&credit.ID,
		&credit.UserID,
		&credit.AccountID,
		&credit.Amount,
		&credit.InterestRate,
		&credit.TermMonths,
		&credit.HMAC,
		&credit.CreatedAt,
		&credit.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("credit not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find credit: %w", err)
	}
	return credit, nil
}

// CreatePaymentSchedule creates a new payment schedule entry
func (r *Repository) CreatePaymentSchedule(payment *models.PaymentSchedule) error {
	query := `
		INSERT INTO bank.payment_schedules (
			credit_id,
			payment_date,
			amount,
			paid,
			penalty,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRow(
		query,
		payment.CreditID,
		payment.PaymentDate,
		payment.Amount,
		payment.Paid,
		payment.Penalty,
	).Scan(&payment.ID, &payment.CreatedAt, &payment.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create payment schedule: %w", err)
	}
	return nil
}

// ListPaymentSchedules retrieves payment schedules for a credit
func (r *Repository) ListPaymentSchedules(creditID int64) ([]*models.PaymentSchedule, error) {
	query := `
		SELECT id, credit_id, payment_date, amount, paid, penalty, created_at, updated_at
		FROM bank.payment_schedules
		WHERE credit_id = $1
		ORDER BY payment_date ASC`
	rows, err := r.db.Query(query, creditID)
	if err != nil {
		return nil, fmt.Errorf("failed to list payment schedules: %w", err)
	}
	defer rows.Close()

	var payments []*models.PaymentSchedule
	for rows.Next() {
		payment := &models.PaymentSchedule{}
		err := rows.Scan(
			&payment.ID,
			&payment.CreditID,
			&payment.PaymentDate,
			&payment.Amount,
			&payment.Paid,
			&payment.Penalty,
			&payment.CreatedAt,
			&payment.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment schedule: %w", err)
		}
		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating payment schedules: %w", err)
	}
	return payments, nil
}

// GetPendingPayments retrieves unpaid payments due today or earlier
func (r *Repository) GetPendingPayments() ([]*models.PaymentSchedule, error) {
	query := `
		SELECT id, credit_id, payment_date, amount, paid, penalty, created_at, updated_at
		FROM bank.payment_schedules
		WHERE paid = FALSE AND payment_date <= $1
		ORDER BY payment_date ASC`
	rows, err := r.db.Query(query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to list pending payments: %w", err)
	}
	defer rows.Close()

	var payments []*models.PaymentSchedule
	for rows.Next() {
		payment := &models.PaymentSchedule{}
		err := rows.Scan(
			&payment.ID,
			&payment.CreditID,
			&payment.PaymentDate,
			&payment.Amount,
			&payment.Paid,
			&payment.Penalty,
			&payment.CreatedAt,
			&payment.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment schedule: %w", err)
		}
		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating payment schedules: %w", err)
	}
	return payments, nil
}

// UpdatePaymentSchedule updates a payment schedule entry
func (r *Repository) UpdatePaymentSchedule(payment *models.PaymentSchedule) error {
	query := `
		UPDATE bank.payment_schedules
		SET amount = $1,
			paid = $2,
			penalty = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $4`
	_, err := r.db.Exec(query, payment.Amount, payment.Paid, payment.Penalty, payment.ID)
	if err != nil {
		return fmt.Errorf("failed to update payment schedule: %w", err)
	}
	return nil
}
