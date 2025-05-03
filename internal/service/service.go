package service

import (
	"context"
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

	// Encrypt card number and expiry date
	encryptedCardNumber, err := utils.Encrypt(cardNumber, s.config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt card number: %w", err)
	}
	encryptedExpiryDate, err := utils.Encrypt(expiryDate, s.config.EncryptionKey)
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
