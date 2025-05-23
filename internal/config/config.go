package config

import (
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	Port          string
	DBConn        string
	LogLevel      string
	JWTSecret     string
	CBRURL        string
	HMACSecret    string
	EncryptionKey string
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SenderEmail   string
}

// NewConfig loads configuration from environment variables
func NewConfig() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		DBConn:        getEnv("DB_CONN", "host=localhost port=5436 user=test password=test dbname=bank sslmode=disable"),
		LogLevel:      getEnv("LOG_LEVEL", "INFO"),
		JWTSecret:     getEnv("JWT_SECRET", "secret"),
		CBRURL:        getEnv("CBR_URL", "https://www.cbr.ru/DailyInfoWebServ/DailyInfo.asmx"),
		HMACSecret:    getEnv("HMAC_SECRET", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"),
		EncryptionKey: getEnv("ENCRYPTION_KEY", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"),
		SMTPHost:      getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:      getEnv("SMTP_PORT", "587"),
		SMTPUsername:  getEnv("SMTP_USERNAME", "-"),
		SMTPPassword:  getEnv("SMTP_PASSWORD", "-"),
		SenderEmail:   getEnv("SENDER_EMAIL", "-"),
	}

	if cfg.DBConn == "" {
		return nil, fmt.Errorf("DB_CONN is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.HMACSecret == "" {
		return nil, fmt.Errorf("HMAC_SECRET is required")
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required")
	}
	if cfg.SMTPUsername == "" || cfg.SMTPPassword == "" {
		return nil, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be set")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
