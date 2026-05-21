package query

import (
	"context"
	"sort"
	"strings"
)

// cqlKeywords / cqlTypes / cqlFunctions are the static dictionaries the editor
// suggests in addition to live schema items. They mirror the frontend's
// `CQL_KW` / `CQL_TY` lists in `web/src/components/cql-highlight.tsx` — keep
// these two in sync until we extract a single source of truth (post-MVP task).
var cqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "LIMIT", "ORDER", "BY",
	"ASC", "DESC", "INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE",
	"CREATE", "TABLE", "KEYSPACE", "TYPE", "INDEX", "MATERIALIZED", "VIEW",
	"DROP", "ALTER", "TRUNCATE", "USE", "WITH", "PRIMARY", "KEY", "IF",
	"EXISTS", "TTL", "USING", "TOKEN", "ALLOW", "FILTERING", "BATCH", "APPLY",
	"BEGIN", "GROUP", "CLUSTERING",
}
var cqlTypes = []string{
	"text", "uuid", "timeuuid", "bigint", "int", "float", "double", "boolean",
	"timestamp", "date", "blob", "inet", "decimal", "varchar", "varint",
	"counter", "ascii", "duration", "set", "list", "map", "tuple", "frozen",
}
var cqlFunctions = []string{
	"now", "uuid", "toTimestamp", "toUnixTimestamp", "currentTimestamp",
	"currentDate", "currentTime", "minTimeuuid", "maxTimeuuid", "dateOf",
	"unixTimestampOf", "writetime", "ttl", "count", "min", "max", "sum", "avg",
	"token", "blobAsText", "textAsBlob",
}

// Suggest returns a merged list of schema and static-dictionary completions.
// `prefix` is matched case-insensitively. `keyspace` (optional) scopes table
// and column suggestions; when empty, the suggestion list spans every keyspace
// the M3 schema service has cached + each keyspace as a top-level entry.
func (s *Service) Suggest(ctx context.Context, ownerID, connID, prefix, keyspace string) ([]CompletionSuggestion, error) {
	prefixLow := strings.ToLower(strings.TrimSpace(prefix))

	var out []CompletionSuggestion

	// Schema-aware completions. We only walk the cache the schema service has
	// already populated for this connection; we don't drive new introspection
	// from autocomplete (would be too aggressive). The workspace primes the
	// cache by listing keyspaces eagerly + tables on expand, so by the time
	// the user is typing the cache is warm.
	if s.schema != nil {
		// 1. Keyspaces.
		if kss, err := s.schema.Keyspaces(ctx, ownerID, connID); err == nil {
			for _, k := range kss {
				if matchesPrefix(k.Name, prefixLow) {
					out = append(out, CompletionSuggestion{
						Label:  k.Name,
						Detail: "keyspace",
						Kind:   CompletionKeyspace,
					})
				}
			}
		}
		// 2. Tables for the active keyspace.
		if keyspace != "" {
			if ts, err := s.schema.Tables(ctx, ownerID, connID, keyspace); err == nil {
				for _, t := range ts {
					if matchesPrefix(t.Name, prefixLow) {
						out = append(out, CompletionSuggestion{
							Label:  t.Name,
							Detail: keyspace + " · table",
							Kind:   CompletionTable,
						})
					}
				}
			}
		}
	}

	// 3. Static keyword / type / function lists.
	for _, kw := range cqlKeywords {
		if matchesPrefix(kw, prefixLow) {
			out = append(out, CompletionSuggestion{Label: kw, Kind: CompletionKeyword})
		}
	}
	for _, t := range cqlTypes {
		if matchesPrefix(t, prefixLow) {
			out = append(out, CompletionSuggestion{Label: t, Detail: "type", Kind: CompletionType})
		}
	}
	for _, fn := range cqlFunctions {
		if matchesPrefix(fn, prefixLow) {
			out = append(out, CompletionSuggestion{Label: fn + "()", Detail: "function", Kind: CompletionFunction})
		}
	}

	// Stable order: kind > label.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})

	// Cap at 50 — keep the popover scannable.
	if len(out) > 50 {
		out = out[:50]
	}
	return out, nil
}

func kindOrder(k CompletionKind) int {
	switch k {
	case CompletionColumn:
		return 0
	case CompletionTable:
		return 1
	case CompletionKeyspace:
		return 2
	case CompletionFunction:
		return 3
	case CompletionType:
		return 4
	default:
		return 5
	}
}

func matchesPrefix(s, lowerPrefix string) bool {
	if lowerPrefix == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(s), lowerPrefix)
}
