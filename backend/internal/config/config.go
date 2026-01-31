package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const secretFileName = ".session_secret"

type Config struct {
	// GitHub OAuth
	GitHubClientID     string
	GitHubClientSecret string

	// Session
	SessionSecret string

	// Database
	DatabasePath string

	// Server
	Port string
	Host string

	// Environment
	Env         string
	IsDev       bool
	FrontendURL string

	// Data directory
	DataDir string
}

func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	dataDir := getEnv("DATA_DIR", "./data")

	cfg := &Config{
		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", DefaultGitHubClientID),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", DefaultGitHubClientSecret),
		SessionSecret:      getEnv("SESSION_SECRET", ""),
		DatabasePath:       getEnv("DATABASE_PATH", filepath.Join(dataDir, "council.db")),
		Port:               getEnv("PORT", "8080"),
		Host:               getEnv("HOST", "0.0.0.0"),
		Env:                getEnv("ENV", "development"),
		DataDir:            dataDir,
	}

	cfg.IsDev = cfg.Env == "development"

	// Set frontend URL based on environment
	if cfg.IsDev {
		cfg.FrontendURL = getEnv("FRONTEND_URL", "http://localhost:5173")
	} else {
		cfg.FrontendURL = getEnv("FRONTEND_URL", fmt.Sprintf("http://%s:%s", cfg.Host, cfg.Port))
	}

	// Auto-generate session secret if not provided
	if cfg.SessionSecret == "" {
		secret, err := loadOrGenerateSecret(cfg.DataDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load/generate session secret: %w", err)
		}
		cfg.SessionSecret = secret
	}

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadOrGenerateSecret(dataDir string) (string, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	secretPath := filepath.Join(dataDir, secretFileName)

	// Try to read existing secret
	if data, err := os.ReadFile(secretPath); err == nil {
		secret := strings.TrimSpace(string(data))
		if len(secret) >= 32 {
			log.Printf("Loaded session secret from %s", secretPath)
			return secret, nil
		}
	}

	// Generate new secret
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	secret := hex.EncodeToString(bytes)

	// Save secret to file
	if err := os.WriteFile(secretPath, []byte(secret), 0600); err != nil {
		return "", fmt.Errorf("failed to save session secret: %w", err)
	}

	log.Printf("Generated and saved new session secret to %s", secretPath)
	return secret, nil
}

func (c *Config) validate() error {
	if c.GitHubClientID == "" {
		return fmt.Errorf("GITHUB_CLIENT_ID is required")
	}
	if c.GitHubClientSecret == "" {
		return fmt.Errorf("GITHUB_CLIENT_SECRET is required")
	}
	if c.SessionSecret == "" {
		return fmt.Errorf("SESSION_SECRET could not be generated")
	}
	if len(c.SessionSecret) < 32 {
		return fmt.Errorf("SESSION_SECRET must be at least 32 characters")
	}
	return nil
}

func (c *Config) ServerAddress() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

func (c *Config) OAuthCallbackURL() string {
	if c.IsDev {
		return fmt.Sprintf("http://localhost:%s/auth/callback", c.Port)
	}
	return fmt.Sprintf("%s/auth/callback", c.FrontendURL)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
