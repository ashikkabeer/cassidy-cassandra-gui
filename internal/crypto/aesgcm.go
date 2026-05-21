package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// Cipher wraps an AES-256-GCM AEAD with a fixed key.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher constructs a Cipher from a 32-byte key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes (got %d)", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt returns nonce || ciphertext || tag. The output can be safely stored
// alongside other unencrypted data (e.g. inside a SQLite BLOB column).
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	if c == nil || c.aead == nil {
		return nil, errors.New("cipher not initialised")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	out := c.aead.Seal(nonce, nonce, plaintext, nil)
	return out, nil
}

// Decrypt parses nonce || ciphertext || tag and returns plaintext, or an error
// if the ciphertext is tampered with.
func (c *Cipher) Decrypt(blob []byte) ([]byte, error) {
	if c == nil || c.aead == nil {
		return nil, errors.New("cipher not initialised")
	}
	ns := c.aead.NonceSize()
	if len(blob) < ns+c.aead.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	return c.aead.Open(nil, nonce, ct, nil)
}
