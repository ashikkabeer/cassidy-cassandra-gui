package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestArgon2RoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword("correct horse battery staple", h)
	if err != nil || !ok {
		t.Fatalf("verify good: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword("wrong password", h)
	if err != nil {
		t.Fatalf("verify bad: err=%v", err)
	}
	if ok {
		t.Fatal("verify wrong password returned true")
	}
}

func TestArgon2Encoding(t *testing.T) {
	h1, _ := HashPassword("same input")
	h2, _ := HashPassword("same input")
	if h1 == h2 {
		t.Fatalf("expected different hashes due to random salt, got identical")
	}
}

func TestAESGCMRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("super-secret-cassandra-password!")
	ct, err := c.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := c.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pt, plain) {
		t.Fatalf("decrypt mismatch: %q vs %q", pt, plain)
	}
}

func TestAESGCMTamperedRejected(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	c, _ := NewCipher(key)
	ct, _ := c.Encrypt([]byte("hello"))
	// Flip a byte in the ciphertext (after the nonce).
	ct[len(ct)-1] ^= 0xff
	if _, err := c.Decrypt(ct); err == nil {
		t.Fatal("expected decrypt to reject tampered ciphertext")
	}
}

func TestAESGCMNonceUnique(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	c, _ := NewCipher(key)
	a, _ := c.Encrypt([]byte("x"))
	b, _ := c.Encrypt([]byte("x"))
	if bytes.Equal(a[:12], b[:12]) {
		t.Fatal("nonces are not unique")
	}
}

func TestKeyLengthValidation(t *testing.T) {
	if _, err := NewCipher(make([]byte, 16)); err == nil {
		t.Fatal("expected error for 16-byte key")
	}
}
