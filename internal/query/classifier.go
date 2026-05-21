// Package query owns CQL execution: classifying the user's statement, running
// it through the pooled gocql session with paging + timeouts, and recording it
// to the on-disk history.
package query

import (
	"errors"
	"strings"
	"unicode"
)

// Kind is the broad category of a CQL statement, computed by the classifier
// purely from the leading keyword (after comments + strings are stripped).
type Kind string

const (
	KindSelect   Kind = "select"
	KindInsert   Kind = "insert"
	KindUpdate   Kind = "update"
	KindDelete   Kind = "delete"
	KindTruncate Kind = "truncate"
	KindCreate   Kind = "create"
	KindAlter    Kind = "alter"
	KindDrop     Kind = "drop"
	KindUse      Kind = "use"
	KindDescribe Kind = "describe"
	KindBatch    Kind = "batch"
	KindGrant    Kind = "grant"
	KindRevoke   Kind = "revoke"
	KindList     Kind = "list"
	KindUnknown  Kind = "unknown"
	KindEmpty    Kind = "empty" // only whitespace / comments / semicolon
)

// Errors returned by Classify. They flow up to the HTTP layer's error mapper.
var (
	ErrMultiStatement = errors.New("multi-statement payloads are not allowed; submit one statement per request")
	ErrUnsupported    = errors.New("statement type is not supported")
	ErrEmpty          = errors.New("empty statement")
)

// IsReadOnly reports whether a kind can be executed against a read-only
// connection. Only SELECT / USE / DESCRIBE qualify.
func IsReadOnly(k Kind) bool {
	switch k {
	case KindSelect, KindUse, KindDescribe:
		return true
	}
	return false
}

// IsDDL reports whether a kind is data-definition (CREATE/ALTER/DROP/TRUNCATE).
// On a successful DDL we invalidate the schema cache so the workspace tree
// reflects the change.
func IsDDL(k Kind) bool {
	switch k {
	case KindCreate, KindAlter, KindDrop, KindTruncate:
		return true
	}
	return false
}

// Classify inspects the supplied CQL and returns its Kind. Comments and string
// literals are stripped first so keywords inside `'…'` or `/* … */` don't fool
// the classifier. Multi-statement payloads (extra non-whitespace after a `;`)
// are rejected with ErrMultiStatement; in M4 we only execute one statement per
// request.
func Classify(cql string) (Kind, error) {
	stripped, hasExtraStatement := stripCommentsAndStrings(cql)
	if hasExtraStatement {
		return "", ErrMultiStatement
	}
	trimmed := strings.TrimSpace(stripped)
	trimmed = strings.TrimRight(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return KindEmpty, ErrEmpty
	}

	// Pull the first keyword.
	kw := firstKeyword(trimmed)
	switch kw {
	case "select":
		return KindSelect, nil
	case "insert":
		return KindInsert, nil
	case "update":
		return KindUpdate, nil
	case "delete":
		return KindDelete, nil
	case "truncate":
		return KindTruncate, nil
	case "create":
		return KindCreate, nil
	case "alter":
		return KindAlter, nil
	case "drop":
		return KindDrop, nil
	case "use":
		return KindUse, nil
	case "describe", "desc":
		return KindDescribe, nil
	case "begin":
		// `BEGIN BATCH …`. Treat as batch.
		return KindBatch, nil
	case "apply":
		// `APPLY BATCH;` shouldn't arrive standalone; reject as malformed.
		return KindBatch, nil
	case "grant":
		return KindGrant, ErrUnsupported
	case "revoke":
		return KindRevoke, ErrUnsupported
	case "list":
		return KindList, ErrUnsupported
	}
	return KindUnknown, nil
}

// firstKeyword returns the leading word, lowercased. It accepts identifiers
// preceded by `EXPLAIN ` (a no-op prefix we don't handle yet but want to peer
// through) by skipping it once.
func firstKeyword(s string) string {
	for _, attempt := range []string{"", "explain "} {
		// `EXPLAIN ` is the only prefix we look through; everything else
		// goes straight to the keyword pull.
		candidate := strings.TrimSpace(s)
		if attempt != "" {
			if !strings.HasPrefix(strings.ToLower(candidate), attempt) {
				continue
			}
			candidate = strings.TrimSpace(candidate[len(attempt):])
		}
		i := 0
		for i < len(candidate) {
			c := rune(candidate[i])
			if !unicode.IsLetter(c) {
				break
			}
			i++
		}
		if i == 0 {
			return ""
		}
		return strings.ToLower(candidate[:i])
	}
	return ""
}

// stripCommentsAndStrings replaces `--`, `//`, and `/* … */` comments with
// spaces and `'…'` string literals with empty quotes (`”`), preserving byte
// positions so multi-statement detection on `;` works. Double-quoted text in
// CQL is a quoted identifier — left intact. Returns the stripped string and
// hasExtraStatement=true if any non-whitespace tokens follow a top-level `;`.
func stripCommentsAndStrings(in string) (string, bool) {
	out := []byte(in)
	semiSeen := false
	hasExtra := false
	i := 0
	for i < len(out) {
		c := out[i]

		// Line comments: `--` or `//`.
		if c == '-' && i+1 < len(out) && out[i+1] == '-' {
			j := i
			for j < len(out) && out[j] != '\n' {
				out[j] = ' '
				j++
			}
			i = j
			continue
		}
		if c == '/' && i+1 < len(out) && out[i+1] == '/' {
			j := i
			for j < len(out) && out[j] != '\n' {
				out[j] = ' '
				j++
			}
			i = j
			continue
		}

		// Block comment: `/* … */`.
		if c == '/' && i+1 < len(out) && out[i+1] == '*' {
			j := i + 2
			for j+1 < len(out) && !(out[j] == '*' && out[j+1] == '/') {
				out[j-2] = ' '
				j++
			}
			// Wipe the opening `/*` and the trailing `*/` (if present).
			out[i] = ' '
			out[i+1] = ' '
			if j+1 < len(out) {
				out[j-2] = ' '
				out[j-1] = ' '
				out[j] = ' '
				out[j+1] = ' '
				j += 2
			} else {
				// Unterminated; whitespace the rest.
				for k := i; k < len(out); k++ {
					out[k] = ' '
				}
				j = len(out)
			}
			i = j
			continue
		}

		// Single-quoted string literal.
		if c == '\'' {
			out[i] = ' '
			j := i + 1
			for j < len(out) {
				if out[j] == '\'' {
					// `''` is an escaped quote.
					if j+1 < len(out) && out[j+1] == '\'' {
						out[j] = ' '
						out[j+1] = ' '
						j += 2
						continue
					}
					out[j] = ' '
					j++
					break
				}
				out[j] = ' '
				j++
			}
			i = j
			continue
		}

		// Top-level semicolon → start watching for extra statements.
		if c == ';' {
			semiSeen = true
			i++
			continue
		}

		// Anything non-whitespace after a `;` → multi-statement.
		if semiSeen && !unicode.IsSpace(rune(c)) {
			hasExtra = true
			break
		}

		i++
	}
	return string(out), hasExtra
}
