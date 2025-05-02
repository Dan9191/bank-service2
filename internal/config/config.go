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
	CBRURL    string
}

func NewConfig() (*Config, error) {
	cfg := &Config{
		Port:      getEnv("PORT", "8080"),
		DBConn:    getEnv("DB_CONN", "host=localhost port=5436 user=test password=test dbname=bank sslmode=disable"),
		LogLevel:  getEnv("LOG_LEVEL", "INFO"),
		JWTSecret: getEnv("JWT_SECRET", "secret"),
		CBRURL:    getEnv("CBR_URL", "https://www.cbr.ru/DailyInfoWebServ/DailyInfo.asmx"),
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
