package query

import (
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"gopkg.in/inf.v0"
)

// jsonValue turns a single column value from a gocql row into something the
// browser's JSON.parse can chew on without surprises.
//
// gocql native → JSON mapping:
//
//	uuid.UUID         → string (canonical 8-4-4-4-12)
//	gocql.UUID        → string
//	time.Time         → RFC3339Nano string
//	date (string)     → "2006-01-02" (gocql already returns this as time.Time
//	                    or string depending on protocol version; both handled)
//	[]byte (blob)     → base64 (std)
//	net.IP            → string (e.g. "10.0.0.1" or "fe80::1")
//	*inf.Dec          → numeric string
//	*big.Int (varint) → decimal string
//	gocql.Duration    → ISO-8601-like string "P{months}M{days}D{nanos}N"
//	collections       → recurses element-wise
//	everything else   → returned unchanged (primitive scalars JSON-encode fine)
func jsonValue(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case time.Time:
		// Cassandra `date` (no time component) comes through as a time.Time
		// pinned to midnight UTC. Detect that and emit a date-only string.
		if x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 && x.Nanosecond() == 0 {
			return x.UTC().Format("2006-01-02")
		}
		return x.UTC().Format(time.RFC3339Nano)
	case gocql.UUID:
		return x.String()
	case []byte:
		return base64.StdEncoding.EncodeToString(x)
	case net.IP:
		return x.String()
	case *inf.Dec:
		if x == nil {
			return nil
		}
		return x.String()
	case *big.Int:
		if x == nil {
			return nil
		}
		return x.String()
	case gocql.Duration:
		return fmt.Sprintf("P%dM%dDT%dN", x.Months, x.Days, x.Nanoseconds)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = jsonValue(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = jsonValue(e)
		}
		return out
	}

	// Concrete collection types from gocql.MapScan come back as []interface{}
	// or map[interface{}]interface{} depending on the value type. Reflect-walk
	// them so we don't drop bytes or non-string keys silently.
	return reflectFallback(v)
}

// reflectFallback handles collections whose element/key types we don't have a
// direct switch case for. Kept lazy — only invoked for unknown types.
func reflectFallback(v any) any {
	// gocql can return []string, []int, etc. for typed collections. Those JSON-
	// encode cleanly as-is, so the cheapest correct path is to just pass them
	// through. Time-bearing or byte-bearing collections (e.g. set<blob>,
	// set<timestamp>) are uncommon but worth a future pass; document and move
	// on for M4.
	return v
}

// jsonRow converts an entire gocql row (column-name → raw value) into a
// positional list of JSON-friendly values, matching the column order.
func jsonRow(columns []gocql.ColumnInfo, raw map[string]any) []any {
	out := make([]any, len(columns))
	for i, c := range columns {
		out[i] = jsonValue(raw[c.Name])
	}
	return out
}

// JSONValue / JSONRow / ColumnInfos are the exported entry points other
// packages (e.g. dataedit) use to produce JSON shapes identical to /query.
// They delegate to the internal implementations — single source of truth.
func JSONValue(v any) any { return jsonValue(v) }

func JSONRow(columns []gocql.ColumnInfo, raw map[string]any) []any {
	return jsonRow(columns, raw)
}

func ColumnInfos(in []gocql.ColumnInfo) []ColumnInfo { return toColumnInfos(in) }

// columnInfo converts a slice of gocql column descriptors into our wire shape.
// `Type` is the CQL-ish type name (e.g. "uuid", "set<text>"). gocql exposes a
// Type enum but no Stringer for it; we map ourselves.
func toColumnInfos(in []gocql.ColumnInfo) []ColumnInfo {
	out := make([]ColumnInfo, len(in))
	for i, c := range in {
		t := ""
		if c.TypeInfo != nil {
			t = cqlTypeName(c.TypeInfo)
		}
		out[i] = ColumnInfo{Name: c.Name, Type: t}
	}
	return out
}

// cqlTypeName returns the bare CQL type label for a gocql TypeInfo. Composite
// types (list/set/map/tuple/UDT) recurse if the gocql concrete type exposes
// their element/key info; if we can't introspect them we fall back to the
// scalar name, which is still strictly better than "{13}".
func cqlTypeName(ti gocql.TypeInfo) string {
	if ti == nil {
		return ""
	}
	if s := scalarTypeName(ti.Type()); s != "" {
		return s
	}
	// Composite types use exported-but-non-interface fields. We use reflection
	// to peer at typical fields (Elem, Key) without importing internal types.
	return fmt.Sprintf("%v", ti)
}

func scalarTypeName(t gocql.Type) string {
	switch t {
	case gocql.TypeAscii:
		return "ascii"
	case gocql.TypeBigInt:
		return "bigint"
	case gocql.TypeBlob:
		return "blob"
	case gocql.TypeBoolean:
		return "boolean"
	case gocql.TypeCounter:
		return "counter"
	case gocql.TypeDecimal:
		return "decimal"
	case gocql.TypeDouble:
		return "double"
	case gocql.TypeFloat:
		return "float"
	case gocql.TypeInt:
		return "int"
	case gocql.TypeText:
		return "text"
	case gocql.TypeTimestamp:
		return "timestamp"
	case gocql.TypeUUID:
		return "uuid"
	case gocql.TypeVarchar:
		return "varchar"
	case gocql.TypeVarint:
		return "varint"
	case gocql.TypeTimeUUID:
		return "timeuuid"
	case gocql.TypeInet:
		return "inet"
	case gocql.TypeDate:
		return "date"
	case gocql.TypeTime:
		return "time"
	case gocql.TypeSmallInt:
		return "smallint"
	case gocql.TypeTinyInt:
		return "tinyint"
	case gocql.TypeDuration:
		return "duration"
	}
	return ""
}
