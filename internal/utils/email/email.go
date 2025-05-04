package email

import (
	"fmt"
	"net/smtp"
	"time"

	"github.com/Dan9191/bank-service/internal/config"
	"github.com/jordan-wright/email"
	"github.com/sirupsen/logrus"
)

// Sender handles sending emails via SMTP
type Sender struct {
	cfg    *config.Config
	logger *logrus.Logger
}

// NewSender creates a new email sender
func NewSender(cfg *config.Config, logger *logrus.Logger) *Sender {
	return &Sender{
		cfg:    cfg,
		logger: logger,
	}
}

// SendPaymentReminder sends a payment reminder email
func (s *Sender) SendPaymentReminder(to, username string, paymentDate time.Time, amount, penalty float64, isOverdue bool) error {
	e := email.NewEmail()
	e.From = s.cfg.SenderEmail
	e.To = []string{to}
	if isOverdue {
		e.Subject = "Overdue Credit Payment Notification"
	} else {
		e.Subject = "Upcoming Credit Payment Reminder"
	}

	// Format email body
	body := fmt.Sprintf(
		"Dear %s,\n\n", username,
	)
	if isOverdue {
		body += fmt.Sprintf(
			"Your credit payment of %.2f RUB was due on %s and is now overdue.\n"+
				"A penalty of %.2f RUB has been applied.\n"+
				"Please make the payment as soon as possible to avoid further penalties.\n",
			amount, paymentDate.Format("2006-01-02"), penalty,
		)
	} else {
		body += fmt.Sprintf(
			"This is a reminder that your credit payment of %.2f RUB is due on %s.\n"+
				"Please ensure sufficient funds are available in your account.\n",
			amount, paymentDate.Format("2006-01-02"),
		)
	}
	body += "\nBest regards,\nBank Service"
	e.Text = []byte(body)

	// Send email
	addr := fmt.Sprintf("%s:%s", s.cfg.SMTPHost, s.cfg.SMTPPort)
	auth := smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	err := e.Send(addr, auth)
	if err != nil {
		s.logger.Errorf("Failed to send email to %s: %v", to, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Infof("Email sent to %s: %s", to, e.Subject)
	return nil
}

// SendTransactionNotification sends a notification email for deposit or withdrawal
func (s *Sender) SendTransactionNotification(to, username string, accountID int64, amount float64, transactionType string, balance float64) error {
	e := email.NewEmail()
	e.From = s.cfg.SenderEmail
	e.To = []string{to}
	e.Subject = fmt.Sprintf("%s Notification", transactionType)

	// Format email body
	body := fmt.Sprintf(
		"Dear %s,\n\n", username,
	)
	if transactionType == "Deposit" {
		body += fmt.Sprintf(
			"Your account %d has been credited with %.2f RUB.\n"+
				"Transaction time: %s\n"+
				"Current balance: %.2f RUB\n",
			accountID, amount, time.Now().Format("2006-01-02 15:04:05"), balance,
		)
	} else if transactionType == "Withdrawal" {
		body += fmt.Sprintf(
			"An amount of %.2f RUB has been withdrawn from your account %d.\n"+
				"Transaction time: %s\n"+
				"Current balance: %.2f RUB\n",
			accountID, amount, time.Now().Format("2006-01-02 15:04:05"), balance,
		)
	}
	body += "\nBest regards,\nBank Service"
	e.Text = []byte(body)

	// Send email
	addr := fmt.Sprintf("%s:%s", s.cfg.SMTPHost, s.cfg.SMTPPort)
	auth := smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	err := e.Send(addr, auth)
	if err != nil {
		s.logger.Errorf("Failed to send %s notification to %s: %v", transactionType, to, err)
		return fmt.Errorf("failed to send %s notification: %w", transactionType, err)
	}

	s.logger.Infof("Email sent to %s: %s", to, e.Subject)
	return nil
}
