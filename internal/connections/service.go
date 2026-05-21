package connections

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ashikkabeer/cassandra-gui/internal/crypto"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

const (
	defaultPort           = 9042
	defaultConsistency    = "LOCAL_QUORUM"
	defaultConnectTimeout = 10_000
	defaultRequestTimeout = 15_000
)

// validConsistencies caps what the UI can ask for. The gocql driver accepts
// more (e.g. `SERIAL`), but those don't apply to ordinary reads/writes so we
// don't surface them in M2.
var validConsistencies = map[string]bool{
	"ONE": true, "LOCAL_ONE": true,
	"TWO": true, "THREE": true,
	"QUORUM": true, "LOCAL_QUORUM": true, "EACH_QUORUM": true,
	"ALL": true, "ANY": true,
}

var (
	ErrInvalidRequest = errors.New("invalid request")
)

// Service composes the connections repo with the cipher used to encrypt
// secrets at rest.
type Service struct {
	repo   *metastore.Connections
	cipher *crypto.Cipher
}

func NewService(repo *metastore.Connections, cipher *crypto.Cipher) *Service {
	return &Service{repo: repo, cipher: cipher}
}

func (s *Service) Create(ctx context.Context, ownerID string, req CreateRequest) (*metastore.Connection, error) {
	if err := validateCreate(&req); err != nil {
		return nil, err
	}
	pwEnc, err := s.encryptIfPresent(req.Password)
	if err != nil {
		return nil, err
	}
	keyEnc, err := s.encryptIfPresent(req.TLSClientKey)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, metastore.CreateConnectionParams{
		OwnerID:          ownerID,
		Name:             strings.TrimSpace(req.Name),
		Hosts:            cleanHosts(req.Hosts),
		Port:             req.Port,
		Datacenter:       strings.TrimSpace(req.Datacenter),
		DefaultKeyspace:  strings.TrimSpace(req.DefaultKeyspace),
		AuthUsername:     strings.TrimSpace(req.AuthUsername),
		AuthPasswordEnc:  pwEnc,
		TLSEnabled:       req.TLSEnabled,
		TLSSkipVerify:    req.TLSSkipVerify,
		TLSCACert:        req.TLSCACert,
		TLSClientCert:    req.TLSClientCert,
		TLSClientKeyEnc:  keyEnc,
		ReadOnly:         req.ReadOnly,
		Consistency:      req.Consistency,
		ConnectTimeoutMS: req.ConnectTimeoutMS,
		RequestTimeoutMS: req.RequestTimeoutMS,
	})
}

// BuildEphemeral converts an unsaved CreateRequest into a *Connection that
// callers (e.g. POST /connections/test) can hand to the Cluster Manager
// without ever persisting it. The returned struct has no ID.
func (s *Service) BuildEphemeral(req CreateRequest) (*metastore.Connection, error) {
	if err := validateCreate(&req); err != nil {
		return nil, err
	}
	pwEnc, err := s.encryptIfPresent(req.Password)
	if err != nil {
		return nil, err
	}
	keyEnc, err := s.encryptIfPresent(req.TLSClientKey)
	if err != nil {
		return nil, err
	}
	c := &metastore.Connection{
		Name:             strings.TrimSpace(req.Name),
		Hosts:            cleanHosts(req.Hosts),
		Port:             req.Port,
		AuthPasswordEnc:  pwEnc,
		TLSEnabled:       req.TLSEnabled,
		TLSSkipVerify:    req.TLSSkipVerify,
		TLSClientKeyEnc:  keyEnc,
		ReadOnly:         req.ReadOnly,
		Consistency:      req.Consistency,
		ConnectTimeoutMS: req.ConnectTimeoutMS,
		RequestTimeoutMS: req.RequestTimeoutMS,
	}
	c.Datacenter = nullString(req.Datacenter)
	c.DefaultKeyspace = nullString(req.DefaultKeyspace)
	c.AuthUsername = nullString(req.AuthUsername)
	c.TLSCACert = nullString(req.TLSCACert)
	c.TLSClientCert = nullString(req.TLSClientCert)
	return c, nil
}

func nullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func (s *Service) Update(ctx context.Context, ownerID, id string, req UpdateRequest) (*metastore.Connection, error) {
	// Existence + ownership check first; this also makes scoping explicit.
	if _, err := s.repo.GetForOwner(ctx, ownerID, id); err != nil {
		return nil, err
	}
	if err := validateUpdate(&req); err != nil {
		return nil, err
	}
	params := metastore.UpdateConnectionParams{
		Name:             req.Name,
		Hosts:            req.Hosts,
		Port:             req.Port,
		Datacenter:       req.Datacenter,
		DefaultKeyspace:  req.DefaultKeyspace,
		AuthUsername:     req.AuthUsername,
		TLSEnabled:       req.TLSEnabled,
		TLSSkipVerify:    req.TLSSkipVerify,
		TLSCACert:        req.TLSCACert,
		TLSClientCert:    req.TLSClientCert,
		ReadOnly:         req.ReadOnly,
		Consistency:      req.Consistency,
		ConnectTimeoutMS: req.ConnectTimeoutMS,
		RequestTimeoutMS: req.RequestTimeoutMS,
	}
	if req.Password != nil {
		enc, err := s.encryptIfPresent(*req.Password)
		if err != nil {
			return nil, err
		}
		params.AuthPasswordEnc = &enc
	}
	if req.TLSClientKey != nil {
		enc, err := s.encryptIfPresent(*req.TLSClientKey)
		if err != nil {
			return nil, err
		}
		params.TLSClientKeyEnc = &enc
	}
	if err := s.repo.Update(ctx, ownerID, id, params); err != nil {
		return nil, err
	}
	return s.repo.GetForOwner(ctx, ownerID, id)
}

func (s *Service) Delete(ctx context.Context, ownerID, id string) error {
	return s.repo.Delete(ctx, ownerID, id)
}

func (s *Service) Get(ctx context.Context, ownerID, id string) (*metastore.Connection, error) {
	return s.repo.GetForOwner(ctx, ownerID, id)
}

func (s *Service) List(ctx context.Context, ownerID string) ([]metastore.Connection, error) {
	return s.repo.ListByOwner(ctx, ownerID)
}

// TouchLastUsed is a best-effort fire-and-forget update of the connection's
// last_used_at column. Schema / query / data-edit services call it on every
// successful introspection so the Connections page can show a fresh "Used …"
// timestamp.
func (s *Service) TouchLastUsed(connectionID string) {
	s.repo.TouchLastUsed(context.Background(), connectionID)
}

func (s *Service) encryptIfPresent(plain string) ([]byte, error) {
	if plain == "" {
		return nil, nil
	}
	enc, err := s.cipher.Encrypt([]byte(plain))
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	return enc, nil
}

func validateCreate(req *CreateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	clean := cleanHosts(req.Hosts)
	if len(clean) == 0 {
		return fmt.Errorf("%w: at least one host is required", ErrInvalidRequest)
	}
	req.Hosts = clean
	if req.Port == 0 {
		req.Port = defaultPort
	}
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("%w: port must be 1-65535", ErrInvalidRequest)
	}
	if req.Consistency == "" {
		req.Consistency = defaultConsistency
	} else if !validConsistencies[strings.ToUpper(req.Consistency)] {
		return fmt.Errorf("%w: unsupported consistency %q", ErrInvalidRequest, req.Consistency)
	}
	req.Consistency = strings.ToUpper(req.Consistency)
	if req.ConnectTimeoutMS == 0 {
		req.ConnectTimeoutMS = defaultConnectTimeout
	}
	if req.RequestTimeoutMS == 0 {
		req.RequestTimeoutMS = defaultRequestTimeout
	}
	if req.TLSSkipVerify && !req.TLSEnabled {
		return fmt.Errorf("%w: tls_skip_verify requires tls_enabled", ErrInvalidRequest)
	}
	return nil
}

func validateUpdate(req *UpdateRequest) error {
	if req.Hosts != nil {
		clean := cleanHosts(*req.Hosts)
		if len(clean) == 0 {
			return fmt.Errorf("%w: at least one host is required", ErrInvalidRequest)
		}
		*req.Hosts = clean
	}
	if req.Port != nil && (*req.Port < 1 || *req.Port > 65535) {
		return fmt.Errorf("%w: port must be 1-65535", ErrInvalidRequest)
	}
	if req.Consistency != nil && *req.Consistency != "" {
		up := strings.ToUpper(*req.Consistency)
		if !validConsistencies[up] {
			return fmt.Errorf("%w: unsupported consistency %q", ErrInvalidRequest, *req.Consistency)
		}
		*req.Consistency = up
	}
	if req.TLSSkipVerify != nil && *req.TLSSkipVerify && req.TLSEnabled != nil && !*req.TLSEnabled {
		return fmt.Errorf("%w: tls_skip_verify requires tls_enabled", ErrInvalidRequest)
	}
	return nil
}

func cleanHosts(in []string) []string {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		// Defensive: a single entry may itself be a comma/space-separated list
		// (e.g. a pasted "10.0.0.1, 10.0.0.2") — split it so gocql never tries
		// to resolve the whole string as one hostname.
		for _, h := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
			if h != "" {
				out = append(out, h)
			}
		}
	}
	return out
}
