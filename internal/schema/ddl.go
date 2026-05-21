package schema

import (
	"fmt"
	"sort"
	"strings"
)

// BuildCreateTable produces a best-effort CREATE TABLE statement from the
// table metadata read out of system_schema. The output is meant for display
// (and copy-paste) — it is NOT guaranteed to be byte-identical to what
// `DESCRIBE TABLE` returns, since system_schema doesn't always preserve the
// original creation-time formatting (column order, whitespace, defaulted
// properties, etc.).
func BuildCreateTable(t *TableDetail) string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s.%s (\n", quoteIdent(t.Keyspace), quoteIdent(t.Name))

	// Column declarations: PK columns first (in position order), then clustering,
	// then regular/static alphabetised. This mirrors how DESCRIBE TABLE typically
	// orders columns when source ordering isn't recoverable.
	cols := append([]Column{}, t.PartitionKey()...)
	cols = append(cols, t.ClusteringKey()...)
	rest := t.RegularAndStatic()
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].Name < rest[j].Name })
	cols = append(cols, rest...)
	for _, c := range cols {
		fmt.Fprintf(&b, "  %s %s", quoteIdent(c.Name), c.Type)
		if c.Kind == ColumnStatic {
			b.WriteString(" static")
		}
		b.WriteString(",\n")
	}

	// PRIMARY KEY clause.
	pk := t.PartitionKey()
	ck := t.ClusteringKey()
	b.WriteString("  PRIMARY KEY (")
	if len(pk) > 1 {
		b.WriteString("(")
		for i, c := range pk {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quoteIdent(c.Name))
		}
		b.WriteString(")")
	} else if len(pk) == 1 {
		b.WriteString(quoteIdent(pk[0].Name))
	}
	for _, c := range ck {
		b.WriteString(", ")
		b.WriteString(quoteIdent(c.Name))
	}
	b.WriteString(")\n)")

	// WITH clause: clustering order + properties.
	with := withClause(t, ck)
	if with != "" {
		b.WriteString(" WITH ")
		b.WriteString(with)
	}
	b.WriteString(";")
	return b.String()
}

func withClause(t *TableDetail, ck []Column) string {
	var parts []string

	// CLUSTERING ORDER BY — only when at least one CK has a non-default order.
	if hasNonDefaultClusteringOrder(ck) {
		var co []string
		for _, c := range ck {
			order := strings.ToUpper(strings.TrimSpace(c.ClusteringOrder))
			if order == "" {
				order = "ASC"
			}
			co = append(co, fmt.Sprintf("%s %s", quoteIdent(c.Name), order))
		}
		parts = append(parts, fmt.Sprintf("CLUSTERING ORDER BY (%s)", strings.Join(co, ", ")))
	}

	if len(t.Compaction) > 0 {
		parts = append(parts, "compaction = "+formatStringMap(t.Compaction))
	}
	if len(t.Compression) > 0 {
		parts = append(parts, "compression = "+formatStringMap(t.Compression))
	}
	if len(t.Caching) > 0 {
		parts = append(parts, "caching = "+formatStringMap(t.Caching))
	}
	if t.GCGraceSeconds > 0 {
		parts = append(parts, fmt.Sprintf("gc_grace_seconds = %d", t.GCGraceSeconds))
	}
	if t.DefaultTTL > 0 {
		parts = append(parts, fmt.Sprintf("default_time_to_live = %d", t.DefaultTTL))
	}
	if t.SpeculativeRetry != "" {
		parts = append(parts, fmt.Sprintf("speculative_retry = '%s'", escapeStringLiteral(t.SpeculativeRetry)))
	}
	if t.BloomFilterFP > 0 {
		parts = append(parts, fmt.Sprintf("bloom_filter_fp_chance = %g", t.BloomFilterFP))
	}
	if t.Comment != "" {
		parts = append(parts, fmt.Sprintf("comment = '%s'", escapeStringLiteral(t.Comment)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n  AND ")
}

func hasNonDefaultClusteringOrder(ck []Column) bool {
	for _, c := range ck {
		order := strings.ToUpper(strings.TrimSpace(c.ClusteringOrder))
		if order != "" && order != "ASC" {
			return true
		}
	}
	return false
}

func formatStringMap(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("'%s': '%s'", escapeStringLiteral(k), escapeStringLiteral(m[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func escapeStringLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// quoteIdent wraps a CQL identifier in double quotes only when necessary
// (mixed case or otherwise non-bareword). Lowercase ASCII identifiers are
// emitted unquoted — matches Cassandra's own DESCRIBE output style.
func quoteIdent(s string) string {
	if !needsQuoting(s) {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for i, r := range s {
		if i == 0 && !(r >= 'a' && r <= 'z') && !(r == '_') {
			return true
		}
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return true
		}
	}
	return false
}
