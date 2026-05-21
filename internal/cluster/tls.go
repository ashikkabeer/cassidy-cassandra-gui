// Package cluster owns Cassidy's gocql session pool and the helpers that turn
// a stored connection configuration (with encrypted secrets) into a live
// gocql.Session.
package cluster

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/ashikkabeer/cassandra-gui/internal/crypto"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

// BuildTLSConfig assembles a *tls.Config from a stored connection's PEM
// material. The client key (if any) is AES-GCM-decrypted via cipher.
//
//   - tls_enabled=false → returns (nil, nil) so the caller can skip TLS entirely.
//   - tls_skip_verify=true is only honored when tls_enabled=true.
//   - The CA cert (if present) becomes the only acceptable root; without one,
//     verification falls back to the system roots.
func BuildTLSConfig(c *metastore.Connection, cipher *crypto.Cipher) (*tls.Config, error) {
	if !c.TLSEnabled {
		if c.TLSSkipVerify {
			return nil, errors.New("tls_skip_verify requires tls_enabled")
		}
		return nil, nil
	}
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.TLSSkipVerify,
	}
	if c.TLSCACert.Valid && c.TLSCACert.String != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(c.TLSCACert.String)) {
			return nil, errors.New("tls_ca_cert: no PEM certificates parsed")
		}
		cfg.RootCAs = pool
	}
	if c.TLSClientCert.Valid && c.TLSClientCert.String != "" && len(c.TLSClientKeyEnc) > 0 {
		keyPEM, err := cipher.Decrypt(c.TLSClientKeyEnc)
		if err != nil {
			return nil, fmt.Errorf("decrypt client key: %w", err)
		}
		cert, err := tls.X509KeyPair([]byte(c.TLSClientCert.String), keyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse client cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}
