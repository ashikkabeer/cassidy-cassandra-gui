package dataedit

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"gopkg.in/inf.v0"
)

// ErrUnsupportedColumn is returned for column types the inline grid can't edit
// safely (counters, collections, UDTs, tuples). Those go through the CQL editor.
var ErrUnsupportedColumn = errors.New("column type is not editable inline")

// coerceValue converts a JSON-decoded value (string / float64 / bool / nil)
// into the Go type gocql expects for a column of the given CQL type, so the
// value marshals correctly when used as a bound parameter.
//
// Big integers / decimals arrive as strings from the client to avoid the
// float64 precision loss JSON would otherwise impose.
func coerceValue(cqlType string, v any) (any, error) {
	base := strings.ToLower(strings.TrimSpace(cqlType))
	if v == nil {
		return nil, nil
	}
	switch base {
	case "text", "varchar", "ascii":
		return asString(v)
	case "uuid", "timeuuid":
		s, err := asString(v)
		if err != nil {
			return nil, err
		}
		u, err := gocql.ParseUUID(s)
		if err != nil {
			return nil, fmt.Errorf("invalid uuid %q: %w", s, err)
		}
		return u, nil
	case "boolean":
		switch x := v.(type) {
		case bool:
			return x, nil
		case string:
			return strconv.ParseBool(x)
		}
		return nil, typeErr(base, v)
	case "int":
		return toInt64Bounded(v, 32)
	case "smallint":
		return toInt64Bounded(v, 16)
	case "tinyint":
		return toInt64Bounded(v, 8)
	case "bigint", "counter":
		// counter is technically not inline-editable (caught by the builder),
		// but coerce it the same as bigint should it ever reach here.
		return toInt64(v)
	case "varint":
		s, err := asString(v)
		if err != nil {
			return nil, err
		}
		n, ok := new(big.Int).SetString(strings.TrimSpace(s), 10)
		if !ok {
			return nil, fmt.Errorf("invalid varint %q", s)
		}
		return n, nil
	case "decimal":
		s, err := asString(v)
		if err != nil {
			return nil, err
		}
		d := new(inf.Dec)
		if _, ok := d.SetString(strings.TrimSpace(s)); !ok {
			return nil, fmt.Errorf("invalid decimal %q", s)
		}
		return d, nil
	case "float":
		f, err := toFloat(v)
		if err != nil {
			return nil, err
		}
		return float32(f), nil
	case "double":
		return toFloat(v)
	case "timestamp":
		s, err := asString(v)
		if err != nil {
			return nil, err
		}
		return parseTimestamp(s)
	case "date":
		s, err := asString(v)
		if err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("invalid date %q (want YYYY-MM-DD): %w", s, err)
		}
		return t, nil
	case "blob":
		s, err := asString(v)
		if err != nil {
			return nil, err
		}
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("invalid base64 blob: %w", err)
		}
		return b, nil
	case "inet":
		s, err := asString(v)
		if err != nil {
			return nil, err
		}
		ip := net.ParseIP(strings.TrimSpace(s))
		if ip == nil {
			return nil, fmt.Errorf("invalid inet %q", s)
		}
		return ip, nil
	}

	// Collections / UDTs / tuples / frozen<...> — not inline-editable.
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedColumn, cqlType)
}

func asString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case float64:
		// JSON number that should've been a string; render without exponent.
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(x), nil
	}
	return "", typeErr("string", v)
}

func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case float64:
		return int64(x), nil
	case string:
		return strconv.ParseInt(strings.TrimSpace(x), 10, 64)
	}
	return 0, typeErr("int", v)
}

func toInt64Bounded(v any, bits int) (int, error) {
	n, err := toInt64(v)
	if err != nil {
		return 0, err
	}
	max := int64(1)<<(bits-1) - 1
	min := -(int64(1) << (bits - 1))
	if n < min || n > max {
		return 0, fmt.Errorf("value %d out of range for %d-bit int", n, bits)
	}
	return int(n), nil
}

func toFloat(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(x), 64)
	}
	return 0, typeErr("float", v)
}

// parseTimestamp accepts RFC3339 (with or without nanos) or a Unix-millis
// number-as-string.
func parseTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999-0700", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.UnixMilli(ms).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", s)
}

func typeErr(want string, v any) error {
	return fmt.Errorf("expected %s, got %T", want, v)
}
