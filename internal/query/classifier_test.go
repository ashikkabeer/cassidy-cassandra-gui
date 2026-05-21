package query

import (
	"errors"
	"testing"
)

func TestClassify_BasicKinds(t *testing.T) {
	cases := []struct {
		in   string
		want Kind
	}{
		{"SELECT * FROM ks.tbl", KindSelect},
		{"select 1", KindSelect},
		{"INSERT INTO ks.tbl (a) VALUES (1)", KindInsert},
		{"  UPDATE ks.tbl SET a = 1 WHERE b = 2", KindUpdate},
		{"DELETE FROM ks.tbl WHERE a = 1", KindDelete},
		{"TRUNCATE ks.tbl", KindTruncate},
		{"CREATE TABLE ks.tbl (id uuid PRIMARY KEY)", KindCreate},
		{"ALTER TABLE ks.tbl ADD col text", KindAlter},
		{"DROP TABLE ks.tbl", KindDrop},
		{"USE ks", KindUse},
		{"DESCRIBE KEYSPACES", KindDescribe},
		{"DESC KEYSPACE ks", KindDescribe},
	}
	for _, c := range cases {
		got, err := Classify(c.in)
		if err != nil {
			t.Errorf("Classify(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Classify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassify_CaseInsensitive(t *testing.T) {
	for _, in := range []string{"select 1", "SELECT 1", "SeLeCt 1"} {
		got, err := Classify(in)
		if err != nil || got != KindSelect {
			t.Errorf("Classify(%q) = (%q, %v), want (select, nil)", in, got, err)
		}
	}
}

func TestClassify_StripsLeadingComments(t *testing.T) {
	cases := []string{
		"-- comment\nSELECT 1",
		"// comment\nSELECT 1",
		"/* block */ SELECT 1",
		"/* multi\nline\nblock */SELECT 1",
		"   \n\t  SELECT 1",
	}
	for _, in := range cases {
		got, err := Classify(in)
		if err != nil || got != KindSelect {
			t.Errorf("Classify(%q) = (%q, %v), want (select, nil)", in, got, err)
		}
	}
}

func TestClassify_KeywordsInStringsDontFool(t *testing.T) {
	cases := []struct {
		in   string
		want Kind
	}{
		// String literal containing INSERT shouldn't reclassify a SELECT.
		{"SELECT 'INSERT INTO foo VALUES (1)' FROM ks.tbl", KindSelect},
		// `''` is an escaped quote; the SELECT keyword still leads.
		{"SELECT 'it''s a string' FROM ks.tbl", KindSelect},
		// Comment containing INSERT inside the SELECT body.
		{"SELECT /* INSERT INTO foo */ * FROM ks.tbl", KindSelect},
		// Real INSERT with comment after the keyword still classifies as INSERT.
		{"INSERT /* hi */ INTO ks.tbl (a) VALUES (1)", KindInsert},
	}
	for _, c := range cases {
		got, err := Classify(c.in)
		if err != nil {
			t.Errorf("Classify(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Classify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassify_MultiStatementRejected(t *testing.T) {
	cases := []string{
		"SELECT 1; SELECT 2",
		"SELECT 1;SELECT 2",
		"USE ks; SELECT 1",
		// Trailing `;` plus whitespace is fine (single statement).
		// Listed in the OK group below.
	}
	for _, in := range cases {
		_, err := Classify(in)
		if !errors.Is(err, ErrMultiStatement) {
			t.Errorf("Classify(%q) expected ErrMultiStatement, got %v", in, err)
		}
	}
}

func TestClassify_TrailingSemiOK(t *testing.T) {
	cases := []string{
		"SELECT 1;",
		"SELECT 1 ;  \n",
		"SELECT 1\n; -- trailing\n",
	}
	for _, in := range cases {
		got, err := Classify(in)
		if err != nil || got != KindSelect {
			t.Errorf("Classify(%q) = (%q, %v), want (select, nil)", in, got, err)
		}
	}
}

func TestClassify_Empty(t *testing.T) {
	for _, in := range []string{"", "   ", ";", "-- only a comment\n", "/* nothing */"} {
		got, err := Classify(in)
		if !errors.Is(err, ErrEmpty) {
			t.Errorf("Classify(%q) expected ErrEmpty, got (%q, %v)", in, got, err)
		}
	}
}

func TestClassify_UnsupportedStatementsFlagged(t *testing.T) {
	cases := []struct {
		in   string
		want Kind
	}{
		{"GRANT SELECT ON ks.tbl TO foo", KindGrant},
		{"REVOKE SELECT ON ks.tbl FROM foo", KindRevoke},
		{"LIST USERS", KindList},
	}
	for _, c := range cases {
		got, err := Classify(c.in)
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("Classify(%q) expected ErrUnsupported, got (%q, %v)", c.in, got, err)
		}
		if got != c.want {
			t.Errorf("Classify(%q) kind = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassify_BatchDetected(t *testing.T) {
	got, err := Classify("BEGIN BATCH INSERT INTO x (id) VALUES (1); APPLY BATCH")
	// BEGIN BATCH contains an internal `;` — that's where the multi-statement
	// guard fires first. Either ErrMultiStatement OR KindBatch is acceptable
	// for M4 (we reject batches at the API layer anyway), but we want at least
	// one of them, not e.g. KindInsert.
	if err == nil && got != KindBatch {
		t.Errorf("BATCH classify: kind=%q err=%v — want either ErrMultiStatement or kind=batch", got, err)
	}
}

func TestIsReadOnly(t *testing.T) {
	if !IsReadOnly(KindSelect) || !IsReadOnly(KindUse) || !IsReadOnly(KindDescribe) {
		t.Fatal("SELECT/USE/DESCRIBE must be read-only")
	}
	for _, k := range []Kind{KindInsert, KindUpdate, KindDelete, KindCreate, KindAlter, KindDrop, KindTruncate} {
		if IsReadOnly(k) {
			t.Errorf("%q must not be read-only", k)
		}
	}
}

func TestIsDDL(t *testing.T) {
	for _, k := range []Kind{KindCreate, KindAlter, KindDrop, KindTruncate} {
		if !IsDDL(k) {
			t.Errorf("%q should be DDL", k)
		}
	}
	for _, k := range []Kind{KindSelect, KindInsert, KindUpdate, KindDelete, KindUse, KindDescribe} {
		if IsDDL(k) {
			t.Errorf("%q should not be DDL", k)
		}
	}
}
