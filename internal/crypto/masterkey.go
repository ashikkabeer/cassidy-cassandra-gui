package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// LoadOrCreateMasterKey returns a 32-byte master key, sourced in this order:
//  1. If envB64 is non-empty, decode it (base64 raw or std).
//  2. If keyPath exists, read it (raw 32 bytes).
//  3. Otherwise generate a fresh key, write it to keyPath with mode 0600, and
//     log a loud warning instructing the operator to back it up.
//
// Losing the master key invalidates any previously-encrypted secrets.
func LoadOrCreateMasterKey(envB64, keyPath string) ([]byte, error) {
	if envB64 != "" {
		k, err := decodeKey(envB64)
		if err != nil {
			return nil, fmt.Errorf("decode CASSIDY_MASTER_KEY: %w", err)
		}
		return k, nil
	}
	if b, err := os.ReadFile(keyPath); err == nil {
		if len(b) != 32 {
			return nil, fmt.Errorf("master key at %q is %d bytes, expected 32", keyPath, len(b))
		}
		return b, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read master key %q: %w", keyPath, err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return nil, fmt.Errorf("write master key %q: %w", keyPath, err)
	}
	slog.Warn(
		"generated a new Cassandra-credential master key — BACK THIS FILE UP",
		"path", keyPath,
		"note", "losing this file makes saved Cassandra passwords unrecoverable",
	)
	return key, nil
}

func decodeKey(s string) ([]byte, error) {
	if k, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(k) == 32 {
		return k, nil
	}
	if k, err := base64.StdEncoding.DecodeString(s); err == nil && len(k) == 32 {
		return k, nil
	}
	return nil, errors.New("expected base64-encoded 32-byte key")
}
