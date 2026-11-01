package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultServerPort = "8080"
	defaultJWTTTL     = 24 * time.Hour
	minimumSecretSize = 32
)

// Config contains all settings needed to start the API.
type Config struct {
	DatabaseURL string
	JWTSecret   string
	ServerPort  string
	JWTTTL      time.Duration
	LogFile     string
}

// Load reads an optional .env file and validates the resulting environment.
// Environment variables that are already set take precedence over .env values.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) < minimumSecretSize {
		return nil, fmt.Errorf("JWT_SECRET must contain at least %d characters", minimumSecretSize)
	}

	serverPort := envOrDefault("SERVER_PORT", defaultServerPort)
	portNumber, err := strconv.Atoi(serverPort)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, errors.New("SERVER_PORT must be a number between 1 and 65535")
	}

	jwtTTL, err := time.ParseDuration(envOrDefault("JWT_TTL", defaultJWTTTL.String()))
	if err != nil || jwtTTL < time.Second {
		return nil, errors.New("JWT_TTL must be at least 1s, for example 24h")
	}

	return &Config{
		DatabaseURL: databaseURL,
		JWTSecret:   jwtSecret,
		ServerPort:  serverPort,
		JWTTTL:      jwtTTL,
		LogFile:     envOrDefault("LOG_FILE", "log.txt"),
	}, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
