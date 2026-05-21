package schema

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"github.com/ashikkabeer/cassandra-gui/internal/cluster"
	"github.com/ashikkabeer/cassandra-gui/internal/connections"
)

// Service answers introspection queries for a saved Cassandra connection.
// It owns the short-TTL cache that keeps the UI snappy without staleness
// drifting more than ~30 seconds.
type Service struct {
	mgr   *cluster.Manager
	conns *connections.Service
	cache *ttlCache
}

func NewService(mgr *cluster.Manager, conns *connections.Service) *Service {
	return &Service{mgr: mgr, conns: conns, cache: newTTLCache(30 * time.Second)}
}

// InvalidateConn drops every cached entry for connID. Called whenever a
// connection's config changes or it's deleted, so the next read rebuilds
// against the new config / proves the row is gone.
func (s *Service) InvalidateConn(connID string) {
	if s == nil || s.cache == nil {
		return
	}
	for _, p := range cachePrefixes(connID) {
		s.cache.invalidatePrefix(p)
	}
}

func cachePrefixes(connID string) []string {
	return []string{
		"cluster:" + connID,
		"keyspaces:" + connID,
		"tables:" + connID + ":",
		"table:" + connID + ":",
		"types:" + connID + ":",
	}
}

// ─── ClusterInfo ──────────────────────────────────────────────────────────

func (s *Service) ClusterInfo(ctx context.Context, ownerID, connID string) (*ClusterInfo, error) {
	key := "cluster:" + connID
	if v, ok := s.cache.get(key); ok {
		return v.(*ClusterInfo), nil
	}
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return nil, err
	}
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	info := &ClusterInfo{}
	if err := sess.Query(
		"SELECT cluster_name, release_version, partitioner, data_center FROM system.local",
	).WithContext(ctx).Consistency(gocql.LocalOne).Scan(
		&info.ClusterName, &info.Version, &info.Partitioner, &info.LocalDC,
	); err != nil {
		return nil, fmt.Errorf("query system.local: %w", err)
	}

	peers, peerErr := s.fetchPeers(ctx, sess)
	if peerErr != nil {
		// Non-fatal: an old cluster might not have system.peers_v2 and that
		// fallback could also fail (rare). Surface host_count=1 (just the local
		// node) but keep going.
		info.HostCount = 1
	} else {
		info.Peers = peers
		info.HostCount = 1 + len(peers)
	}
	s.cache.set(key, info)
	s.touchLastUsed(connID)
	return info, nil
}

// fetchPeers tries system.peers_v2 first (Cassandra 4+), falls back to
// system.peers (Cassandra 3). Returns the union of (peer, dc, version).
func (s *Service) fetchPeers(ctx context.Context, sess *gocql.Session) ([]Peer, error) {
	for _, table := range []string{"system.peers_v2", "system.peers"} {
		iter := sess.Query(
			"SELECT peer, data_center, release_version FROM " + table,
		).WithContext(ctx).Consistency(gocql.LocalOne).Iter()
		var peers []Peer
		var addr net.IP
		var dc, ver string
		for iter.Scan(&addr, &dc, &ver) {
			peers = append(peers, Peer{Address: addr.String(), DC: dc, Version: ver})
		}
		if err := iter.Close(); err != nil {
			// Likely "unconfigured table". Try the next one.
			continue
		}
		return peers, nil
	}
	return nil, errors.New("no peer table available")
}

// ─── Keyspaces ────────────────────────────────────────────────────────────

func (s *Service) Keyspaces(ctx context.Context, ownerID, connID string) ([]Keyspace, error) {
	key := "keyspaces:" + connID
	if v, ok := s.cache.get(key); ok {
		return v.([]Keyspace), nil
	}
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return nil, err
	}
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	iter := sess.Query(
		"SELECT keyspace_name, durable_writes, replication FROM system_schema.keyspaces",
	).WithContext(ctx).Consistency(gocql.LocalOne).Iter()

	var (
		name string
		dw   bool
		repl map[string]string
		out  []Keyspace
	)
	for iter.Scan(&name, &dw, &repl) {
		ks := Keyspace{
			Name:          name,
			DurableWrites: dw,
			Replication:   parseReplication(repl),
			System:        IsSystemKeyspace(name),
		}
		out = append(out, ks)
		repl = nil // gocql reuses the destination map; reset for next row.
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("scan keyspaces: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		// User keyspaces first, system last, then alphabetical.
		if out[i].System != out[j].System {
			return !out[i].System
		}
		return out[i].Name < out[j].Name
	})
	s.cache.set(key, out)
	s.touchLastUsed(connID)
	return out, nil
}

func parseReplication(m map[string]string) Replication {
	r := Replication{Raw: m, Class: ""}
	if class, ok := m["class"]; ok {
		// Strip the org.apache.cassandra.locator. prefix that some versions emit.
		if idx := strings.LastIndex(class, "."); idx >= 0 {
			class = class[idx+1:]
		}
		r.Class = class
	}
	switch r.Class {
	case "SimpleStrategy":
		if v, ok := m["replication_factor"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				r.Factor = n
			}
		}
	case "NetworkTopologyStrategy":
		r.DCFactors = map[string]int{}
		for k, v := range m {
			if k == "class" || k == "replication_factor" {
				continue
			}
			if n, err := strconv.Atoi(v); err == nil {
				r.DCFactors[k] = n
			}
		}
	}
	return r
}

// ─── Tables ───────────────────────────────────────────────────────────────

func (s *Service) Tables(ctx context.Context, ownerID, connID, keyspace string) ([]TableSummary, error) {
	key := "tables:" + connID + ":" + keyspace
	if v, ok := s.cache.get(key); ok {
		return v.([]TableSummary), nil
	}
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return nil, err
	}
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	iter := sess.Query(
		"SELECT table_name, comment, default_time_to_live, gc_grace_seconds, compaction "+
			"FROM system_schema.tables WHERE keyspace_name = ?",
		keyspace,
	).WithContext(ctx).Consistency(gocql.LocalOne).Iter()

	var (
		name    string
		comment string
		ttl     int
		gcg     int
		compact map[string]string
		out     []TableSummary
	)
	for iter.Scan(&name, &comment, &ttl, &gcg, &compact) {
		out = append(out, TableSummary{
			Name:            name,
			Comment:         comment,
			DefaultTTL:      ttl,
			GCGraceSeconds:  gcg,
			CompactionClass: shortClass(compact["class"]),
		})
		compact = nil
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("scan tables: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	s.cache.set(key, out)
	s.touchLastUsed(connID)
	return out, nil
}

// ─── Table detail ─────────────────────────────────────────────────────────

func (s *Service) Table(ctx context.Context, ownerID, connID, keyspace, table string) (*TableDetail, error) {
	key := "table:" + connID + ":" + keyspace + ":" + table
	if v, ok := s.cache.get(key); ok {
		return v.(*TableDetail), nil
	}
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return nil, err
	}
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	detail := &TableDetail{Keyspace: keyspace, Name: table}

	// Properties row.
	var (
		comment     string
		ttl         int
		gcg         int
		caching     map[string]string
		compaction  map[string]string
		compression map[string]string
		bloomFP     float64
		specRetry   string
		flags       []string
	)
	err = sess.Query(
		`SELECT comment, default_time_to_live, gc_grace_seconds, caching,
		         compaction, compression, bloom_filter_fp_chance, speculative_retry, flags
		   FROM system_schema.tables
		  WHERE keyspace_name = ? AND table_name = ?`,
		keyspace, table,
	).WithContext(ctx).Consistency(gocql.LocalOne).Scan(
		&comment, &ttl, &gcg, &caching, &compaction, &compression, &bloomFP, &specRetry, &flags,
	)
	if errors.Is(err, gocql.ErrNotFound) {
		return nil, ErrTableNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query system_schema.tables: %w", err)
	}
	detail.Comment = comment
	detail.DefaultTTL = ttl
	detail.GCGraceSeconds = gcg
	detail.Caching = caching
	detail.Compaction = compaction
	detail.Compression = compression
	detail.BloomFilterFP = bloomFP
	detail.SpeculativeRetry = specRetry
	detail.Flags = flags

	// Columns.
	colIter := sess.Query(
		`SELECT column_name, type, kind, position, clustering_order
		   FROM system_schema.columns
		  WHERE keyspace_name = ? AND table_name = ?`,
		keyspace, table,
	).WithContext(ctx).Consistency(gocql.LocalOne).Iter()
	var (
		cName, cType, cKind, cOrder string
		cPosition                   int
	)
	for colIter.Scan(&cName, &cType, &cKind, &cPosition, &cOrder) {
		col := Column{
			Name:     cName,
			Type:     cType,
			Kind:     ColumnKind(cKind),
			Position: cPosition,
		}
		if strings.ToLower(cOrder) != "none" && cOrder != "" {
			col.ClusteringOrder = strings.ToLower(cOrder)
		}
		detail.Columns = append(detail.Columns, col)
	}
	if err := colIter.Close(); err != nil {
		return nil, fmt.Errorf("scan columns: %w", err)
	}

	// Indexes.
	idxIter := sess.Query(
		`SELECT index_name, kind, options
		   FROM system_schema.indexes
		  WHERE keyspace_name = ? AND table_name = ?`,
		keyspace, table,
	).WithContext(ctx).Consistency(gocql.LocalOne).Iter()
	var (
		iName, iKind string
		iOpts        map[string]string
	)
	for idxIter.Scan(&iName, &iKind, &iOpts) {
		detail.Indexes = append(detail.Indexes, Index{Name: iName, Kind: iKind, Options: iOpts})
		iOpts = nil
	}
	if err := idxIter.Close(); err != nil {
		return nil, fmt.Errorf("scan indexes: %w", err)
	}

	s.cache.set(key, detail)
	s.touchLastUsed(connID)
	return detail, nil
}

// ─── DDL ──────────────────────────────────────────────────────────────────

func (s *Service) DDL(ctx context.Context, ownerID, connID, keyspace, table string) (string, error) {
	t, err := s.Table(ctx, ownerID, connID, keyspace, table)
	if err != nil {
		return "", err
	}
	return BuildCreateTable(t), nil
}

// ─── Types (UDTs) ─────────────────────────────────────────────────────────

func (s *Service) Types(ctx context.Context, ownerID, connID, keyspace string) ([]UDT, error) {
	key := "types:" + connID + ":" + keyspace
	if v, ok := s.cache.get(key); ok {
		return v.([]UDT), nil
	}
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return nil, err
	}
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return nil, err
	}
	iter := sess.Query(
		`SELECT type_name, field_names, field_types
		   FROM system_schema.types WHERE keyspace_name = ?`,
		keyspace,
	).WithContext(ctx).Consistency(gocql.LocalOne).Iter()

	var (
		typeName            string
		fieldNames, fieldTs []string
		out                 []UDT
	)
	for iter.Scan(&typeName, &fieldNames, &fieldTs) {
		fields := make([]UDTField, 0, len(fieldNames))
		for i := range fieldNames {
			t := ""
			if i < len(fieldTs) {
				t = fieldTs[i]
			}
			fields = append(fields, UDTField{Name: fieldNames[i], Type: t})
		}
		out = append(out, UDT{Name: typeName, Fields: fields})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("scan types: %w", err)
	}
	s.cache.set(key, out)
	s.touchLastUsed(connID)
	return out, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────

// ErrTableNotFound is returned when the requested keyspace/table pair has no
// row in system_schema.tables. Mapped to 404 at the HTTP layer.
var ErrTableNotFound = errors.New("table not found")

// touchLastUsed updates the connection's last_used_at in the metastore on
// every successful introspection — keeps the Connections page's "Used …"
// timestamp fresh. Best-effort, ignored on error.
func (s *Service) touchLastUsed(connID string) {
	// Service-layer access to the connections repo is via the Service's
	// underlying field. We avoid plumbing the repo separately by exposing a
	// public TouchLastUsed on connections.Service.
	s.conns.TouchLastUsed(connID)
}

// shortClass extracts the simple class name from a Cassandra FQCN.
func shortClass(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}
