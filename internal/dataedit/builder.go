package dataedit

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ashikkabeer/cassandra-gui/internal/schema"
)

var (
	// ErrIncompletePK is returned when an update/delete op is missing a value
	// for any partition or clustering column. This is the core safety guard:
	// without it, an UPDATE/DELETE could hit an entire partition.
	ErrIncompletePK = errors.New("every partition and clustering key must be provided")
	// ErrNoChanges is returned for an update op with an empty Set.
	ErrNoChanges = errors.New("update has no changed columns")
	// ErrUnknownColumn is returned when an op references a column not on the table.
	ErrUnknownColumn = errors.New("unknown column")
)

// statement is the dual representation the builder produces: a parameterized
// form for execution and an inlined form for the preview dialog.
type statement struct {
	cql     string // with ? placeholders
	args    []any
	display string // inlined literals (escaped) — for the BATCH preview
}

// columnIndex builds a name → schema.Column map for quick lookups.
func columnIndex(td *schema.TableDetail) map[string]schema.Column {
	m := make(map[string]schema.Column, len(td.Columns))
	for _, c := range td.Columns {
		m[c.Name] = c
	}
	return m
}

// primaryKeyColumns returns partition + clustering columns ordered by kind then
// position (partition first, then clustering) — the WHERE-clause order.
func primaryKeyColumns(td *schema.TableDetail) []schema.Column {
	pk := append([]schema.Column{}, td.PartitionKey()...)
	return append(pk, td.ClusteringKey()...)
}

// editableColumns returns true for a column the inline grid may edit: a
// regular or static column that isn't a counter or a collection.
func isEditable(c schema.Column) bool {
	if c.Kind != schema.ColumnRegular && c.Kind != schema.ColumnStatic {
		return false
	}
	return !isUnsupportedType(c.Type)
}

func isUnsupportedType(t string) bool {
	low := strings.ToLower(strings.TrimSpace(t))
	if low == "counter" {
		return true
	}
	// Collections / UDT / tuple / frozen wrappers.
	for _, p := range []string{"set<", "list<", "map<", "tuple<", "frozen<"} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

// BuildInsert constructs an INSERT for a new row. Requires every PK + CK column.
func BuildInsert(td *schema.TableDetail, op Op) (statement, error) {
	idx := columnIndex(td)
	full := mergeRow(op)

	// All PK/CK columns must be present.
	for _, c := range primaryKeyColumns(td) {
		if _, ok := full[c.Name]; !ok {
			return statement{}, fmt.Errorf("%w: missing %q", ErrIncompletePK, c.Name)
		}
	}

	names := sortedKeys(full)
	cols := make([]string, 0, len(names))
	placeholders := make([]string, 0, len(names))
	args := make([]any, 0, len(names))
	disp := make([]string, 0, len(names))
	for _, name := range names {
		col, ok := idx[name]
		if !ok {
			return statement{}, fmt.Errorf("%w: %q", ErrUnknownColumn, name)
		}
		if isUnsupportedType(col.Type) {
			return statement{}, fmt.Errorf("%w: %s (%s)", ErrUnsupportedColumn, name, col.Type)
		}
		val, err := coerceValue(col.Type, full[name])
		if err != nil {
			return statement{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, quoteIdent(name))
		placeholders = append(placeholders, "?")
		args = append(args, val)
		disp = append(disp, displayLiteral(col.Type, full[name]))
	}

	cql := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)",
		quoteIdent(td.Keyspace), quoteIdent(td.Name),
		strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	display := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)",
		quoteIdent(td.Keyspace), quoteIdent(td.Name),
		strings.Join(cols, ", "), strings.Join(disp, ", "))
	return statement{cql: cql, args: args, display: display}, nil
}

// BuildUpdate constructs an UPDATE. Requires every PK+CK in op.PK and a
// non-empty Set of editable columns.
func BuildUpdate(td *schema.TableDetail, op Op) (statement, error) {
	idx := columnIndex(td)
	if err := requireFullPK(td, op.PK); err != nil {
		return statement{}, err
	}
	if len(op.Set) == 0 {
		return statement{}, ErrNoChanges
	}

	setNames := sortedKeys(op.Set)
	setClauses := make([]string, 0, len(setNames))
	setDisplay := make([]string, 0, len(setNames))
	args := make([]any, 0, len(setNames))
	for _, name := range setNames {
		col, ok := idx[name]
		if !ok {
			return statement{}, fmt.Errorf("%w: %q", ErrUnknownColumn, name)
		}
		if !isEditable(col) {
			return statement{}, fmt.Errorf("%w: %s (%s)", ErrUnsupportedColumn, name, col.Type)
		}
		val, err := coerceValue(col.Type, op.Set[name])
		if err != nil {
			return statement{}, fmt.Errorf("column %q: %w", name, err)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdent(name)))
		setDisplay = append(setDisplay, fmt.Sprintf("%s = %s", quoteIdent(name), displayLiteral(col.Type, op.Set[name])))
		args = append(args, val)
	}

	where, whereArgs, whereDisplay, err := buildWhere(td, idx, op.PK)
	if err != nil {
		return statement{}, err
	}
	args = append(args, whereArgs...)

	cql := fmt.Sprintf("UPDATE %s.%s SET %s WHERE %s",
		quoteIdent(td.Keyspace), quoteIdent(td.Name),
		strings.Join(setClauses, ", "), where)
	display := fmt.Sprintf("UPDATE %s.%s SET %s WHERE %s",
		quoteIdent(td.Keyspace), quoteIdent(td.Name),
		strings.Join(setDisplay, ", "), whereDisplay)
	return statement{cql: cql, args: args, display: display}, nil
}

// BuildDelete constructs a row DELETE keyed by the full primary key.
func BuildDelete(td *schema.TableDetail, op Op) (statement, error) {
	idx := columnIndex(td)
	if err := requireFullPK(td, op.PK); err != nil {
		return statement{}, err
	}
	where, args, whereDisplay, err := buildWhere(td, idx, op.PK)
	if err != nil {
		return statement{}, err
	}
	cql := fmt.Sprintf("DELETE FROM %s.%s WHERE %s",
		quoteIdent(td.Keyspace), quoteIdent(td.Name), where)
	display := fmt.Sprintf("DELETE FROM %s.%s WHERE %s",
		quoteIdent(td.Keyspace), quoteIdent(td.Name), whereDisplay)
	return statement{cql: cql, args: args, display: display}, nil
}

// requireFullPK ensures pk has a value for every partition + clustering column.
func requireFullPK(td *schema.TableDetail, pk map[string]any) error {
	for _, c := range primaryKeyColumns(td) {
		v, ok := pk[c.Name]
		if !ok || v == nil {
			return fmt.Errorf("%w: missing %q", ErrIncompletePK, c.Name)
		}
	}
	return nil
}

// buildWhere produces the `pk1 = ? AND ck1 = ?` clause in primary-key order.
func buildWhere(td *schema.TableDetail, idx map[string]schema.Column, pk map[string]any) (where string, args []any, display string, err error) {
	pkCols := primaryKeyColumns(td)
	clauses := make([]string, 0, len(pkCols))
	disp := make([]string, 0, len(pkCols))
	for _, c := range pkCols {
		val, cerr := coerceValue(c.Type, pk[c.Name])
		if cerr != nil {
			return "", nil, "", fmt.Errorf("primary key %q: %w", c.Name, cerr)
		}
		clauses = append(clauses, fmt.Sprintf("%s = ?", quoteIdent(c.Name)))
		disp = append(disp, fmt.Sprintf("%s = %s", quoteIdent(c.Name), displayLiteral(c.Type, pk[c.Name])))
		args = append(args, val)
	}
	return strings.Join(clauses, " AND "), args, strings.Join(disp, " AND "), nil
}

func mergeRow(op Op) map[string]any {
	out := make(map[string]any, len(op.PK)+len(op.Set))
	for k, v := range op.PK {
		out[k] = v
	}
	for k, v := range op.Set {
		out[k] = v
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// displayLiteral renders a value for the human-readable BATCH preview. Strings,
// uuids, timestamps, blobs are quoted/escaped; numbers + booleans are bare.
func displayLiteral(cqlType string, v any) string {
	if v == nil {
		return "null"
	}
	base := strings.ToLower(strings.TrimSpace(cqlType))
	switch base {
	case "int", "smallint", "tinyint", "bigint", "varint", "decimal", "float", "double", "counter", "boolean":
		return fmt.Sprintf("%v", v)
	}
	return "'" + strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "''") + "'"
}

// quoteIdent mirrors schema/ddl.go: bare lowercase idents stay unquoted,
// everything else is double-quoted.
func quoteIdent(s string) string {
	if s == "" {
		return `""`
	}
	bare := true
	for i, r := range s {
		if i == 0 && !(r >= 'a' && r <= 'z') && r != '_' {
			bare = false
			break
		}
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			bare = false
			break
		}
	}
	if bare {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
