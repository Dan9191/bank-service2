package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port      string
	DBConn    string
	LogLevel  string
	JWTSecret string
}

func NewConfig() (*Config, error) {
	cfg := &Config{
		Port:      getEnv("PORT", "8080"),
		DBConn:    getEnv("DB_CONN", "host=localhost port=5432 user=postgres password=postgres dbname=bank sslmode=disable"),
		LogLevel:  getEnv("LOG_LEVEL", "INFO"),
		JWTSecret: getEnv("JWT_SECRET", "secret"),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
