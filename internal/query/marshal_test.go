package query

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"gopkg.in/inf.v0"
)

func TestJSONValue_PrimitivesPassThrough(t *testing.T) {
	cases := []any{
		"hello",
		int64(42),
		float64(3.14),
		true,
		nil,
	}
	for _, v := range cases {
		got := jsonValue(v)
		// Round-trip through JSON to make sure the value encodes cleanly.
		if _, err := json.Marshal(got); err != nil {
			t.Errorf("jsonValue(%v) failed to JSON-encode: %v", v, err)
		}
	}
}

func TestJSONValue_UUID(t *testing.T) {
	u, _ := gocql.RandomUUID()
	got := jsonValue(u)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if len(s) != 36 || s[8] != '-' {
		t.Fatalf("not a canonical UUID: %q", s)
	}
}

func TestJSONValue_TimestampRFC3339Nano(t *testing.T) {
	tm := time.Date(2026, 5, 20, 12, 34, 56, 789_000_000, time.UTC)
	got := jsonValue(tm)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if s != "2026-05-20T12:34:56.789Z" {
		t.Fatalf("got %q", s)
	}
}

func TestJSONValue_DateOnly(t *testing.T) {
	// Midnight UTC → date-only.
	d := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	got := jsonValue(d)
	if got != "2026-05-20" {
		t.Fatalf("got %v", got)
	}
}

func TestJSONValue_BlobBase64(t *testing.T) {
	blob := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	got := jsonValue(blob)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	want := base64.StdEncoding.EncodeToString(blob)
	if s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}

func TestJSONValue_InetString(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	if got := jsonValue(ip); got != "10.0.0.1" {
		t.Fatalf("got %v", got)
	}
}

func TestJSONValue_DecimalAndVarint(t *testing.T) {
	d := inf.NewDec(12345, 2) // 123.45
	if got := jsonValue(d); got != "123.45" {
		t.Fatalf("decimal: got %v", got)
	}
	v := new(big.Int).SetInt64(1234567890123)
	if got := jsonValue(v); got != "1234567890123" {
		t.Fatalf("varint: got %v", got)
	}
}

func TestJSONValue_Collections(t *testing.T) {
	u1, _ := gocql.RandomUUID()
	u2, _ := gocql.RandomUUID()
	list := []any{u1, u2}
	got := jsonValue(list)
	out, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d", len(out))
	}
	for _, e := range out {
		s, ok := e.(string)
		if !ok || len(s) != 36 {
			t.Fatalf("element not a uuid string: %v", e)
		}
	}

	mapV := map[string]any{
		"created_at": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"data":       []byte{0x01, 0x02},
	}
	gotMap := jsonValue(mapV).(map[string]any)
	if gotMap["created_at"] != "2026-01-01" {
		t.Fatalf("nested timestamp not coerced: %v", gotMap["created_at"])
	}
	if gotMap["data"] != base64.StdEncoding.EncodeToString([]byte{0x01, 0x02}) {
		t.Fatalf("nested blob not coerced: %v", gotMap["data"])
	}
}

func TestJSONValue_Duration(t *testing.T) {
	d := gocql.Duration{Months: 1, Days: 2, Nanoseconds: 3_000_000_000}
	got := jsonValue(d)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if s != "P1M2DT3000000000N" {
		t.Fatalf("got %q", s)
	}
}

func TestJSONValue_NilSafe(t *testing.T) {
	if got := jsonValue(nil); got != nil {
		t.Fatalf("nil should stay nil, got %v", got)
	}
	var nullDec *inf.Dec
	if got := jsonValue(nullDec); got != nil {
		t.Fatalf("nil decimal should stay nil, got %v", got)
	}
}
