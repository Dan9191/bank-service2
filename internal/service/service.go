package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Dan9191/bank-service/internal/config"
	"github.com/Dan9191/bank-service/internal/integrations/cbr"
	"github.com/Dan9191/bank-service/internal/models"
	"github.com/Dan9191/bank-service/internal/repository"
	"github.com/Dan9191/bank-service/internal/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// Service handles business logic
type Service struct {
	repo      *repository.Repository
	log       *logrus.Logger
	config    *config.Config
	cbrClient *cbr.CBRClient
	cron      *cron.Cron
}

// NewService initializes a new service
func NewService(repo *repository.Repository, log *logrus.Logger, cfg *config.Config, cbrClient *cbr.CBRClient) *Service {
	svc := &Service{
		repo:      repo,
		log:       log,
		config:    cfg,
		cbrClient: cbrClient,
		cron:      cron.New(),
	}
	svc.startScheduler()
	return svc
}

// startScheduler starts the cron job for processing payments
func (s *Service) startScheduler() {
	_, err := s.cron.AddFunc("@every 24h", s.processPendingPayments)
	if err != nil {
		s.log.Fatalf("Failed to start payment scheduler: %v", err)
	}
	s.cron.Start()
	s.log.Info("Payment scheduler started")
}

// calculateAnnuityPayment calculates the monthly annuity payment
func (s *Service) calculateAnnuityPayment(principal, annualRate float64, termMonths int) float64 {
	monthlyRate := annualRate / 100 / 12
	term := float64(termMonths)
	// Annuity formula: P = (r * PV) / (1 - (1 + r)^(-n))
	payment := (monthlyRate * principal) / (1 - math.Pow(1+monthlyRate, -term))
	return math.Round(payment*100) / 100 // Round to 2 decimal places
}

// generatePaymentSchedule generates the payment schedule for a credit
func (s *Service) generatePaymentSchedule(credit *models.Credit) ([]*models.PaymentSchedule, error) {
	payments := []*models.PaymentSchedule{}
	monthlyPayment := s.calculateAnnuityPayment(credit.Amount, credit.InterestRate, credit.TermMonths)

	for i := 0; i < credit.TermMonths; i++ {
		payment := &models.PaymentSchedule{
			CreditID:    credit.ID,
			PaymentDate: time.Now().AddDate(0, i+1, 0).Truncate(24 * time.Hour),
			Amount:      monthlyPayment,
			Paid:        false,
			Penalty:     0,
		}
		payments = append(payments, payment)
	}

	return payments, nil
}

// processPendingPayments processes all pending payments
func (s *Service) processPendingPayments() {
	ctx := context.Background()
	payments, err := s.repo.GetPendingPayments()
	if err != nil {
		s.log.Errorf("Failed to get pending payments: %v", err)
		return
	}

	for _, payment := range payments {
		s.log.Debugf("Processing payment ID %d for credit %d, amount %.2f, due %s", payment.ID, payment.CreditID, payment.Amount, payment.PaymentDate.Format("2006-01-02"))

		// Get credit to find account_id
		credit, err := s.repo.FindCreditByID(payment.CreditID)
		if err != nil {
			s.log.Errorf("Failed to find credit %d for payment %d: %v", payment.CreditID, payment.ID, err)
			continue
		}

		// Check account balance
		balance, err := s.repo.GetAccountBalance(credit.AccountID)
		if err != nil {
			s.log.Errorf("Failed to get balance for account %d: %v", credit.AccountID, err)
			continue
		}

		totalAmount := payment.Amount + payment.Penalty
		if balance >= totalAmount {
			// Process payment
			tx := &models.Transaction{
				AccountID:   credit.AccountID,
				Amount:      -totalAmount,
				Type:        "credit_payment",
				Description: fmt.Sprintf("Credit payment for credit %d, payment %d", payment.CreditID, payment.ID),
			}
			if err := s.repo.Withdraw(ctx, tx); err != nil {
				s.log.Errorf("Failed to withdraw payment %d for credit %d: %v", payment.ID, payment.CreditID, err)
				continue
			}

			// Update payment status
			payment.Paid = true
			if err := s.repo.UpdatePaymentSchedule(payment); err != nil {
				s.log.Errorf("Failed to update payment %d status: %v", payment.ID, err)
				continue
			}
			s.log.Infof("Payment %d for credit %d processed successfully, amount %.2f", payment.ID, payment.CreditID, totalAmount)
		} else {
			// Apply penalty (10% of payment amount)
			penalty := payment.Amount * 0.10
			payment.Penalty += penalty
			if err := s.repo.UpdatePaymentSchedule(payment); err != nil {
				s.log.Errorf("Failed to apply penalty for payment %d: %v", payment.ID, err)
				continue
			}
			s.log.Warnf("Payment %d for credit %d overdue, penalty %.2f applied", payment.ID, payment.CreditID, penalty)
		}
	}
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

// CreateCredit creates a new credit with payment schedule
func (s *Service) CreateCredit(ctx context.Context, accountID int64, amount float64, termMonths int) (*models.Credit, error) {
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

	// Validate input
	if amount <= 0 {
		return nil, fmt.Errorf("credit amount must be positive")
	}
	if termMonths <= 0 || termMonths > 360 {
		return nil, fmt.Errorf("term must be between 1 and 360 months")
	}

	// Get interest rate from CBR
	interestRate, err := s.cbrClient.GetKeyRate()
	if err != nil {
		return nil, fmt.Errorf("failed to get interest rate: %w", err)
	}

	// Generate HMAC for credit
	hmac := utils.GenerateHMAC(
		fmt.Sprintf("%d", userID),
		fmt.Sprintf("%d", accountID),
		fmt.Sprintf("%.2f", amount),
		s.config.HMACSecret,
	)

	credit := &models.Credit{
		UserID:       userID,
		AccountID:    accountID,
		Amount:       amount,
		InterestRate: interestRate,
		TermMonths:   termMonths,
		HMAC:         hmac,
	}

	// Create credit
	if err := s.repo.CreateCredit(credit); err != nil {
		return nil, err
	}

	// Generate and save payment schedule
	payments, err := s.generatePaymentSchedule(credit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate payment schedule: %w", err)
	}
	for _, payment := range payments {
		if err := s.repo.CreatePaymentSchedule(payment); err != nil {
			return nil, fmt.Errorf("failed to save payment schedule: %w", err)
		}
	}

	s.log.Infof("Credit created for account %d, amount %.2f, term %d months, rate %.2f%%", accountID, amount, termMonths, interestRate)
	return credit, nil
}

// ListPaymentSchedules retrieves the payment schedule for a credit
func (s *Service) ListPaymentSchedules(ctx context.Context, creditID int64) ([]*models.PaymentSchedule, error) {
	userIDStr, ok := ctx.Value("userID").(string)
	if !ok || userIDStr == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Verify credit belongs to user
	credit, err := s.repo.FindCreditByID(creditID)
	if err != nil {
		return nil, fmt.Errorf("credit not found: %w", err)
	}
	if credit.UserID != userID {
		return nil, fmt.Errorf("credit does not belong to user")
	}

	payments, err := s.repo.ListPaymentSchedules(creditID)
	if err != nil {
		return nil, err
	}

	s.log.Infof("Retrieved %d payment schedules for credit %d", len(payments), creditID)
	return payments, nil
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
		s.log.Debugf("Decrypting card ID %d: card_number=%s, expiry_date=%s", card.ID, card.CardNumber, card.ExpiryDate)
		decryptedCardNumber, err := utils.Decrypt(card.CardNumber, encryptionKey)
		if err != nil {
			s.log.Errorf("Failed to decrypt card number for card ID %d: %v", card.ID, err)
			return nil, fmt.Errorf("failed to decrypt card number for card ID %d: %w", card.ID, err)
		}
		decryptedExpiryDate, err := utils.Decrypt(card.ExpiryDate, encryptionKey)
		if err != nil {
			s.log.Errorf("Failed to decrypt expiry date for card ID %d: %v", card.ID, err)
			return nil, fmt.Errorf("failed to decrypt expiry date for card ID %d: %w", card.ID, err)
		}
		card.CardNumber = decryptedCardNumber
		card.ExpiryDate = decryptedExpiryDate
	}

	s.log.Infof("Retrieved %d cards for user %d", len(cards), userID)
	return cards, nil
}
