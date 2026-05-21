package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// SetupToken holds the one-time first-run token used to claim the admin account.
// It is created at process start: either from the CASSIDY_SETUP_TOKEN env var, or
// loaded from data/setup-token.txt, or generated and written there with 0600.
type SetupToken struct {
	mu    sync.Mutex
	value string
	path  string // file to delete when token is consumed
}

// LoadOrCreateSetupToken returns a SetupToken according to the rules above.
// `firstRun` should be true when the database has no active admin.
//   - If !firstRun, returns a SetupToken with no value (Verify will always fail).
//   - If firstRun and envToken != "", uses that (no file persisted).
//   - Otherwise reads tokenPath if present, or generates + writes.
func LoadOrCreateSetupToken(firstRun bool, envToken, tokenPath string) (*SetupToken, error) {
	if !firstRun {
		// Ensure no stale token file persists.
		_ = os.Remove(tokenPath)
		return &SetupToken{}, nil
	}
	if envToken != "" {
		slog.Warn("first-run setup token loaded from CASSIDY_SETUP_TOKEN env var")
		return &SetupToken{value: envToken}, nil
	}
	if b, err := os.ReadFile(tokenPath); err == nil {
		t := string(b)
		// Re-emit on startup so operators can find it without digging through files.
		slog.Warn(
			"first-run setup pending — paste this token at /first-run to claim the admin account",
			"setup_token", t,
			"path", tokenPath,
		)
		return &SetupToken{value: t, path: tokenPath}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read setup token: %w", err)
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate setup token: %w", err)
	}
	t := "cs_setup_" + base64.RawURLEncoding.EncodeToString(raw)
	if err := os.WriteFile(tokenPath, []byte(t), 0o600); err != nil {
		return nil, fmt.Errorf("write setup token: %w", err)
	}
	slog.Warn(
		"first-run setup pending — paste this token at /first-run to claim the admin account",
		"setup_token", t,
		"path", tokenPath,
	)
	return &SetupToken{value: t, path: tokenPath}, nil
}

// Verify constant-time compares the supplied token against the stored value.
// Returns false if setup is closed (no value).
func (s *SetupToken) Verify(supplied string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.value == "" {
		return false
	}
	if len(supplied) != len(s.value) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(s.value)) == 1
}

// Consume invalidates the token, deleting the on-disk copy if any.
func (s *SetupToken) Consume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = ""
	if s.path != "" {
		_ = os.Remove(s.path)
	}
}

// Open reports whether setup is still pending.
func (s *SetupToken) Open() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value != ""
}
