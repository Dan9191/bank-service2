package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/Dan9191/bank-service/internal/config"
	"github.com/Dan9191/bank-service/internal/models"
	"github.com/Dan9191/bank-service/internal/repository"
	"github.com/Dan9191/bank-service/internal/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// Service handles business logic
type Service struct {
	repo   *repository.Repository
	log    *logrus.Logger
	config *config.Config
}

// NewService initializes a new service
func NewService(repo *repository.Repository, log *logrus.Logger, cfg *config.Config) *Service {
	return &Service{repo: repo, log: log, config: cfg}
}

// Register creates a new user with hashed password
func (s *Service) Register(username, email, password string) (*models.User, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedPassword),
	}

	if err := s.repo.CreateUser(user); err != nil {
		return nil, err
	}

	s.log.Infof("User registered: %s", user.Email)
	return user, nil
}

// Login authenticates a user and returns a JWT token
func (s *Service) Login(email, password string) (string, error) {
	user, err := s.repo.FindUserByEmail(email)
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   fmt.Sprintf("%d", user.ID),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	})
	tokenString, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	s.log.Infof("User logged in: %s", user.Email)
	return tokenString, nil
}

// CreateAccount creates a new account for the authenticated user
func (s *Service) CreateAccount(ctx context.Context, currency string) (*models.Account, error) {
	userIDStr, ok := ctx.Value("userID").(string)
	if !ok || userIDStr == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	account := &models.Account{
		UserID:   userID,
		Balance:  0.0,
		Currency: currency,
	}

	if err := s.repo.CreateAccount(account); err != nil {
		return nil, err
	}

	s.log.Infof("Account created for user %d: %s", userID, account.Currency)
	return account, nil
}

// CreateCard creates a new card for the specified account
func (s *Service) CreateCard(ctx context.Context, accountID int64) (*models.Card, error) {
	userIDStr, ok := ctx.Value("userID").(string)
	if !ok || userIDStr == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Verify account belongs to user
	accountUserID, err := s.repo.FindAccountByID(accountID)
	if err != nil {
		return nil, err
	}
	if accountUserID != userID {
		return nil, fmt.Errorf("account does not belong to user")
	}

	// Generate card details
	cardNumber, err := utils.GenerateCardNumber("400000", 16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate card number: %w", err)
	}
	expiryDate := utils.GenerateExpiryDate()
	cvv := utils.GenerateCVV()

	// Decode encryption key from hex
	encryptionKey, err := hex.DecodeString(s.config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}
	if len(encryptionKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(encryptionKey))
	}

	// Encrypt card number and expiry date
	encryptedCardNumber, err := utils.Encrypt(cardNumber, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt card number: %w", err)
	}
	encryptedExpiryDate, err := utils.Encrypt(expiryDate, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt expiry date: %w", err)
	}

	// Hash CVV
	cvvHash, err := bcrypt.GenerateFromPassword([]byte(cvv), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash CVV: %w", err)
	}

	// Generate HMAC
	hmac := utils.GenerateHMAC(cardNumber, expiryDate, cvv, s.config.HMACSecret)

	card := &models.Card{
		AccountID:  accountID,
		CardNumber: encryptedCardNumber,
		ExpiryDate: encryptedExpiryDate,
		CVV:        string(cvvHash),
		HMAC:       hmac,
	}

	// Store card with encrypted fields
	if err := s.repo.CreateCard(card); err != nil {
		return nil, err
	}

	// Return card with decrypted fields for response
	card.CardNumber = cardNumber
	card.ExpiryDate = expiryDate
	s.log.Infof("Card created for account %d", accountID)
	return card, nil
}

// Deposit adds funds to an account
func (s *Service) Deposit(ctx context.Context, accountID int64, amount float64) (*models.Transaction, error) {
	userIDStr, ok := ctx.Value("userID").(string)
	if !ok || userIDStr == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Verify account belongs to user
	accountUserID, err := s.repo.FindAccountByID(accountID)
	if err != nil {
		return nil, err
	}
	if accountUserID != userID {
		return nil, fmt.Errorf("account does not belong to user")
	}

	// Validate amount
	if amount <= 0 {
		return nil, fmt.Errorf("deposit amount must be positive")
	}

	transaction := &models.Transaction{
		AccountID:   accountID,
		Amount:      amount,
		Type:        "deposit",
		Description: "Deposit to account",
	}

	if err := s.repo.Deposit(ctx, transaction); err != nil {
		return nil, err
	}

	s.log.Infof("Deposit of %f to account %d", amount, accountID)
	return transaction, nil
}

// Withdraw removes funds from an account
func (s *Service) Withdraw(ctx context.Context, accountID int64, amount float64) (*models.Transaction, error) {
	userIDStr, ok := ctx.Value("userID").(string)
	if !ok || userIDStr == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Verify account belongs to user
	accountUserID, err := s.repo.FindAccountByID(accountID)
	if err != nil {
		return nil, err
	}
	if accountUserID != userID {
		return nil, fmt.Errorf("account does not belong to user")
	}

	// Validate amount
	if amount <= 0 {
		return nil, fmt.Errorf("withdrawal amount must be positive")
	}

	// Check balance
	balance, err := s.repo.GetAccountBalance(accountID)
	if err != nil {
		return nil, err
	}
	if balance < amount {
		return nil, fmt.Errorf("insufficient funds")
	}

	transaction := &models.Transaction{
		AccountID:   accountID,
		Amount:      -amount, // Negative for withdrawal
		Type:        "withdrawal",
		Description: "Withdrawal from account",
	}

	if err := s.repo.Withdraw(ctx, transaction); err != nil {
		return nil, err
	}

	s.log.Infof("Withdrawal of %f from account %d", amount, accountID)
	return transaction, nil
}

// Transfer moves funds between accounts
func (s *Service) Transfer(ctx context.Context, fromAccountID, toAccountID int64, amount float64) ([]*models.Transaction, error) {
	userIDStr, ok := ctx.Value("userID").(string)
	if !ok || userIDStr == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Verify from_account belongs to user
	fromAccountUserID, err := s.repo.FindAccountByID(fromAccountID)
	if err != nil {
		return nil, err
	}
	if fromAccountUserID != userID {
		return nil, fmt.Errorf("from account does not belong to user")
	}

	// Verify to_account exists
	_, err = s.repo.FindAccountByID(toAccountID)
	if err != nil {
		return nil, err
	}

	// Validate amount
	if amount <= 0 {
		return nil, fmt.Errorf("transfer amount must be positive")
	}

	// Check balance
	balance, err := s.repo.GetAccountBalance(fromAccountID)
	if err != nil {
		return nil, err
	}
	if balance < amount {
		return nil, fmt.Errorf("insufficient funds")
	}

	// Create transactions
	withdrawal := &models.Transaction{
		AccountID:   fromAccountID,
		Amount:      -amount,
		Type:        "transfer_out",
		Description: fmt.Sprintf("Transfer to account %d", toAccountID),
	}
	deposit := &models.Transaction{
		AccountID:   toAccountID,
		Amount:      amount,
		Type:        "transfer_in",
		Description: fmt.Sprintf("Transfer from account %d", fromAccountID),
	}

	if err := s.repo.Transfer(ctx, withdrawal, deposit); err != nil {
		return nil, err
	}

	s.log.Infof("Transfer of %f from account %d to account %d", amount, fromAccountID, toAccountID)
	return []*models.Transaction{withdrawal, deposit}, nil
}

// ListTransactions retrieves a list of transactions for an account
func (s *Service) ListTransactions(ctx context.Context, accountID int64, transactionType string, limit, offset int) ([]*models.Transaction, error) {
	userIDStr, ok := ctx.Value("userID").(string)
	if !ok || userIDStr == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Verify account belongs to user
	accountUserID, err := s.repo.FindAccountByID(accountID)
	if err != nil {
		return nil, err
	}
	if accountUserID != userID {
		return nil, fmt.Errorf("account does not belong to user")
	}

	// Validate pagination
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if offset < 0 {
		offset = 0
	}

	transactions, err := s.repo.ListTransactions(accountID, transactionType, limit, offset)
	if err != nil {
		return nil, err
	}

	s.log.Infof("Retrieved %d transactions for account %d", len(transactions), accountID)
	return transactions, nil
}

// ListCards retrieves a list of cards for a user or specific account
func (s *Service) ListCards(ctx context.Context, accountID int64, limit, offset int) ([]*models.Card, error) {
	userIDStr, ok := ctx.Value("userID").(string)
	if !ok || userIDStr == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Validate pagination
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if offset < 0 {
		offset = 0
	}

	// If account_id is specified, verify it belongs to user
	if accountID != 0 {
		accountUserID, err := s.repo.FindAccountByID(accountID)
		if err != nil {
			return nil, err
		}
		if accountUserID != userID {
			return nil, fmt.Errorf("account does not belong to user")
		}
	}

	cards, err := s.repo.ListCards(userID, accountID, limit, offset)
	if err != nil {
		return nil, err
	}

	// Decode encryption key from hex
	encryptionKey, err := hex.DecodeString(s.config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}
	if len(encryptionKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(encryptionKey))
	}

	// Decrypt card_number and expiry_date
	for _, card := range cards {
		decryptedCardNumber, err := utils.Decrypt(card.CardNumber, encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt card number: %w", err)
		}
		decryptedExpiryDate, err := utils.Decrypt(card.ExpiryDate, encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt expiry date: %w", err)
		}
		card.CardNumber = decryptedCardNumber
		card.ExpiryDate = decryptedExpiryDate
	}

	s.log.Infof("Retrieved %d cards for user %d", len(cards), userID)
	return cards, nil
}
