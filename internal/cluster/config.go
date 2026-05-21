package cluster

import (
	"fmt"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"github.com/ashikkabeer/cassandra-gui/internal/crypto"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

// BuildClusterConfig turns a stored Connection into a *gocql.ClusterConfig
// ready for `cluster.CreateSession()`. Secrets (auth password, TLS client key)
// are decrypted via `cipher` and never persist outside the returned config.
func BuildClusterConfig(c *metastore.Connection, cipher *crypto.Cipher) (*gocql.ClusterConfig, error) {
	if c == nil {
		return nil, fmt.Errorf("nil connection")
	}
	if len(c.Hosts) == 0 {
		return nil, fmt.Errorf("connection has no hosts")
	}

	cfg := gocql.NewCluster(c.Hosts...)
	if c.Port > 0 {
		cfg.Port = c.Port
	}
	if c.DefaultKeyspace.Valid {
		cfg.Keyspace = c.DefaultKeyspace.String
	}
	if c.Consistency != "" {
		cfg.Consistency = gocql.ParseConsistency(c.Consistency)
	}
	if c.ConnectTimeoutMS > 0 {
		cfg.ConnectTimeout = time.Duration(c.ConnectTimeoutMS) * time.Millisecond
	}
	if c.RequestTimeoutMS > 0 {
		cfg.Timeout = time.Duration(c.RequestTimeoutMS) * time.Millisecond
	}

	if c.Datacenter.Valid && c.Datacenter.String != "" {
		cfg.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(
			gocql.DCAwareRoundRobinPolicy(c.Datacenter.String),
		)
	}

	if c.AuthUsername.Valid && c.AuthUsername.String != "" {
		password := ""
		if len(c.AuthPasswordEnc) > 0 {
			plain, err := cipher.Decrypt(c.AuthPasswordEnc)
			if err != nil {
				return nil, fmt.Errorf("decrypt password: %w", err)
			}
			password = string(plain)
		}
		cfg.Authenticator = gocql.PasswordAuthenticator{
			Username: c.AuthUsername.String,
			Password: password,
		}
	}

	tlsCfg, err := BuildTLSConfig(c, cipher)
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		cfg.SslOpts = &gocql.SslOptions{
			Config:                 tlsCfg,
			EnableHostVerification: !c.TLSSkipVerify,
		}
	}
	return cfg, nil
}
