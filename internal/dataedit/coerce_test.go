package dataedit

import (
	"math/big"
	"testing"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
)

func TestCoerce_UUID(t *testing.T) {
	v, err := coerceValue("uuid", "7c4a3b2e-9d18-4a8a-b3e1-39b22c14a55b")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(gocql.UUID); !ok {
		t.Fatalf("expected gocql.UUID, got %T", v)
	}
	if _, err := coerceValue("uuid", "not-a-uuid"); err == nil {
		t.Fatal("expected error for bad uuid")
	}
}

func TestCoerce_Timestamp(t *testing.T) {
	v, err := coerceValue("timestamp", "2026-05-20T14:58:02Z")
	if err != nil {
		t.Fatal(err)
	}
	tm, ok := v.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", v)
	}
	if tm.Year() != 2026 || tm.Month() != 5 || tm.Day() != 20 {
		t.Fatalf("wrong time: %v", tm)
	}
}

func TestCoerce_Date(t *testing.T) {
	v, err := coerceValue("date", "2026-05-20")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(time.Time); !ok {
		t.Fatalf("expected time.Time, got %T", v)
	}
	if _, err := coerceValue("date", "2026/05/20"); err == nil {
		t.Fatal("expected error for bad date format")
	}
}

func TestCoerce_BigIntPrecision(t *testing.T) {
	// A value past 2^53 must survive — proves we parse from string, not float64.
	const big1 = "9007199254740993" // 2^53 + 1
	v, err := coerceValue("bigint", big1)
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := v.(int64); !ok || n != 9007199254740993 {
		t.Fatalf("bigint precision lost: %v (%T)", v, v)
	}
}

func TestCoerce_Varint(t *testing.T) {
	v, err := coerceValue("varint", "123456789012345678901234567890")
	if err != nil {
		t.Fatal(err)
	}
	n, ok := v.(*big.Int)
	if !ok {
		t.Fatalf("expected *big.Int, got %T", v)
	}
	if n.String() != "123456789012345678901234567890" {
		t.Fatalf("varint wrong: %s", n.String())
	}
}

func TestCoerce_IntRange(t *testing.T) {
	if _, err := coerceValue("int", float64(42)); err != nil {
		t.Fatalf("int 42: %v", err)
	}
	if _, err := coerceValue("tinyint", float64(200)); err == nil {
		t.Fatal("expected tinyint 200 out of range")
	}
}

func TestCoerce_Boolean(t *testing.T) {
	v, err := coerceValue("boolean", true)
	if err != nil || v != true {
		t.Fatalf("bool: %v %v", v, err)
	}
	v, err = coerceValue("boolean", "false")
	if err != nil || v != false {
		t.Fatalf("bool from string: %v %v", v, err)
	}
}

func TestCoerce_Nil(t *testing.T) {
	v, err := coerceValue("text", nil)
	if err != nil || v != nil {
		t.Fatalf("nil should pass through: %v %v", v, err)
	}
}

func TestCoerce_Blob(t *testing.T) {
	v, err := coerceValue("blob", "3q2+7w==") // base64 of DE AD BE EF
	if err != nil {
		t.Fatal(err)
	}
	b, ok := v.([]byte)
	if !ok || len(b) != 4 || b[0] != 0xDE {
		t.Fatalf("blob decode wrong: %v", v)
	}
}

func TestCoerce_UnsupportedType(t *testing.T) {
	if _, err := coerceValue("set<text>", "{a}"); err == nil {
		t.Fatal("expected error for collection type")
	}
}
