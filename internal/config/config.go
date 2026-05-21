package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr      string
	DataDir         string
	MasterKeyB64    string // optional override; if empty, file-or-generate
	SetupToken      string // optional override; if empty, file-or-generate on first run
	CookieSecure    bool
	CookieDomain    string
	SessionTTL      time.Duration
	SessionIdleTTL  time.Duration
	LoginRateLimit  int           // attempts
	LoginRateWindow time.Duration // window
}

// Load reads config from environment variables, falling back to sane defaults.
func Load() (*Config, error) {
	c := &Config{
		ListenAddr:      envOr("CASSIDY_LISTEN_ADDR", ":8080"),
		DataDir:         envOr("CASSIDY_DATA_DIR", "./data"),
		MasterKeyB64:    os.Getenv("CASSIDY_MASTER_KEY"),
		SetupToken:      os.Getenv("CASSIDY_SETUP_TOKEN"),
		CookieDomain:    os.Getenv("CASSIDY_COOKIE_DOMAIN"),
		CookieSecure:    envBool("CASSIDY_COOKIE_SECURE", false),
		SessionTTL:      envDuration("CASSIDY_SESSION_TTL", 30*24*time.Hour),
		SessionIdleTTL:  envDuration("CASSIDY_SESSION_IDLE_TTL", 7*24*time.Hour),
		LoginRateLimit:  envInt("CASSIDY_LOGIN_RATE_LIMIT", 5),
		LoginRateWindow: envDuration("CASSIDY_LOGIN_RATE_WINDOW", 15*time.Minute),
	}

	absData, err := filepath.Abs(c.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	c.DataDir = absData
	if err := os.MkdirAll(c.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", c.DataDir, err)
	}
	return c, nil
}

func (c *Config) DBPath() string         { return filepath.Join(c.DataDir, "cassidy.db") }
func (c *Config) MasterKeyPath() string  { return filepath.Join(c.DataDir, "master.key") }
func (c *Config) SetupTokenPath() string { return filepath.Join(c.DataDir, "setup-token.txt") }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
