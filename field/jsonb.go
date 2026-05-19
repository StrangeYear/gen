package field

import (
	"database/sql/driver"
	"encoding/json"
	"strings"

	"gorm.io/gorm/clause"
)

// JSONB represents a PostgreSQL jsonb query field.
type JSONB struct {
	genericsField[driver.Valuer]
}

type jsonValuer struct {
	value interface{}
}

func (v jsonValuer) Value() (driver.Value, error) {
	if v.value == nil {
		return nil, nil
	}
	if valuer, ok := v.value.(driver.Valuer); ok {
		return valuer.Value()
	}
	bs, err := json.Marshal(v.value)
	if err != nil {
		return nil, err
	}
	return string(bs), nil
}

type jsonbOpExpr struct {
	column expression
	op     string
	value  interface{}
	cast   bool
}

func (e jsonbOpExpr) Build(builder clause.Builder) {
	builder.AddVar(builder, e.column)
	builder.WriteByte(' ')
	builder.WriteString(e.op)
	builder.WriteByte(' ')
	builder.AddVar(builder, e.value)
	if e.cast {
		builder.WriteString("::jsonb")
	}
}

type jsonbKeysExpr struct {
	column   expression
	op       string
	keys     []string
	emptySQL string
}

func (e jsonbKeysExpr) Build(builder clause.Builder) {
	if len(e.keys) == 0 {
		builder.WriteString(e.emptySQL)
		return
	}

	builder.AddVar(builder, e.column)
	builder.WriteByte(' ')
	builder.WriteString(e.op)
	builder.WriteString(" array[")
	for i, key := range e.keys {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.AddVar(builder, key)
	}
	builder.WriteByte(']')
}

func jsonbPathClause(column expression, singleOp, multiOp string, path []string) clause.Expr {
	if len(path) == 1 {
		return clause.Expr{SQL: "? " + singleOp + " ?", Vars: []interface{}{column, path[0]}}
	}

	sql := "? " + multiOp + " array[" + strings.Repeat("?,", len(path))
	sql = strings.TrimSuffix(sql, ",") + "]"
	vars := make([]interface{}, 0, len(path)+1)
	vars = append(vars, column)
	for _, item := range path {
		vars = append(vars, item)
	}
	return clause.Expr{SQL: sql, Vars: vars}
}

type jsonbArrayPathExpr struct {
	column expression
	op     string
	path   []string
}

func (e jsonbArrayPathExpr) Build(builder clause.Builder) {
	builder.AddVar(builder, e.column)
	builder.WriteByte(' ')
	builder.WriteString(e.op)
	builder.WriteString(" array[")
	for i, item := range e.path {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.AddVar(builder, item)
	}
	builder.WriteByte(']')
}

// NewJSONB create new PostgreSQL jsonb field.
func NewJSONB(table, column string, opts ...Option) JSONB {
	return JSONB{genericsField: newGenerics[driver.Valuer](expr{col: toColumn(table, column, opts...)})}
}

func jsonValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	if _, ok := value.(driver.Valuer); ok {
		return value
	}
	return jsonValuer{value: value}
}

func jsonValues(values []interface{}) []interface{} {
	out := make([]interface{}, len(values))
	for i, value := range values {
		out[i] = jsonValue(value)
	}
	return out
}

// Eq judge equal after converting ordinary Go values to JSON.
func (field JSONB) Eq(value interface{}) Expr {
	return expr{e: clause.Eq{Column: field.RawExpr(), Value: jsonValue(value)}}
}

// Neq judge not equal after converting ordinary Go values to JSON.
func (field JSONB) Neq(value interface{}) Expr {
	return expr{e: clause.Neq{Column: field.RawExpr(), Value: jsonValue(value)}}
}

// In tests whether the jsonb column equals any of the provided JSON values.
func (field JSONB) In(values ...interface{}) Expr {
	return expr{e: clause.IN{Column: field.RawExpr(), Values: jsonValues(values)}}
}

// Value assigns an ordinary Go value to a jsonb column.
func (field JSONB) Value(value interface{}) AssignExpr {
	return field.value(jsonValue(value))
}

// IfNull returns IFNULL(column, value) with value converted to JSON.
func (field JSONB) IfNull(value interface{}) Expr {
	return field.ifNull(jsonValue(value))
}

// Contains builds column @> ?::jsonb.
func (field JSONB) Contains(value interface{}) Expr {
	return expr{e: jsonbOpExpr{column: field.RawExpr(), op: "@>", value: jsonValue(value), cast: true}}
}

// ContainedBy builds column <@ ?::jsonb.
func (field JSONB) ContainedBy(value interface{}) Expr {
	return expr{e: jsonbOpExpr{column: field.RawExpr(), op: "<@", value: jsonValue(value), cast: true}}
}

// HasKey tests whether a top-level key exists.
func (field JSONB) HasKey(key string) Expr {
	return expr{e: jsonbOpExpr{column: field.RawExpr(), op: "?", value: key}}
}

// HasAnyKeys tests whether any top-level key exists. Empty keys is always false.
func (field JSONB) HasAnyKeys(keys ...string) Expr {
	return expr{e: jsonbKeysExpr{column: field.RawExpr(), op: "?|", keys: keys, emptySQL: "1 = 0"}}
}

// HasAllKeys tests whether all top-level keys exist. Empty keys is vacuously true.
func (field JSONB) HasAllKeys(keys ...string) Expr {
	return expr{e: jsonbKeysExpr{column: field.RawExpr(), op: "?&", keys: keys, emptySQL: "1 = 1"}}
}

// Text extracts JSON text by path. Empty path falls back to the current column expression.
func (field JSONB) Text(path ...string) String {
	if len(path) == 0 {
		return newChars[string](field.expr)
	}
	return newChars[string](field.setE(jsonbPathClause(field.RawExpr(), "->>", "#>>", path)))
}

// Path extracts JSON by path. Empty path falls back to the current jsonb expression.
func (field JSONB) Path(path ...string) JSONB {
	if len(path) == 0 {
		return field
	}
	return JSONB{genericsField: newGenerics[driver.Valuer](field.setE(jsonbPathClause(field.RawExpr(), "->", "#>", path)))}
}

// ArrayLength returns jsonb_array_length(column).
func (field JSONB) ArrayLength() Int {
	return newNumber[int](field.setE(clause.Expr{SQL: "jsonb_array_length(?)", Vars: []interface{}{field.RawExpr()}}))
}

// IsEmptyObject tests column = '{}'::jsonb.
func (field JSONB) IsEmptyObject() Expr {
	return expr{e: clause.Expr{SQL: "? = '{}'::jsonb", Vars: []interface{}{field.RawExpr()}}}
}

// IsEmptyArray tests column = '[]'::jsonb.
func (field JSONB) IsEmptyArray() Expr {
	return expr{e: clause.Expr{SQL: "? = '[]'::jsonb", Vars: []interface{}{field.RawExpr()}}}
}

// TypeOf returns jsonb_typeof(column).
func (field JSONB) TypeOf() String {
	return newChars[string](field.setE(clause.Expr{SQL: "jsonb_typeof(?)", Vars: []interface{}{field.RawExpr()}}))
}

func (field JSONB) isType(t string) Expr {
	return field.TypeOf().Eq(t)
}

// IsObject tests jsonb_typeof(column) = 'object'.
func (field JSONB) IsObject() Expr {
	return field.isType("object")
}

// IsArray tests jsonb_typeof(column) = 'array'.
func (field JSONB) IsArray() Expr {
	return field.isType("array")
}

// IsString tests jsonb_typeof(column) = 'string'.
func (field JSONB) IsString() Expr {
	return field.isType("string")
}

// IsNumber tests jsonb_typeof(column) = 'number'.
func (field JSONB) IsNumber() Expr {
	return field.isType("number")
}

// IsBoolean tests jsonb_typeof(column) = 'boolean'.
func (field JSONB) IsBoolean() Expr {
	return field.isType("boolean")
}

// IsNullValue tests jsonb_typeof(column) = 'null'.
func (field JSONB) IsNullValue() Expr {
	return field.isType("null")
}

// DeleteKey assigns column = column - ?.
func (field JSONB) DeleteKey(key string) AssignExpr {
	return field.value(clause.Expr{SQL: "? - ?", Vars: []interface{}{field.RawExpr(), key}})
}

// DeletePath assigns column = column #- array[?, ...]. Empty path is a no-op.
func (field JSONB) DeletePath(path ...string) AssignExpr {
	if len(path) == 0 {
		return field.value(field.RawExpr())
	}
	return field.value(jsonbArrayPathExpr{column: field.RawExpr(), op: "#-", path: path})
}

// SetPath assigns column = jsonb_set(column, array[?, ...], ?::jsonb, true). Empty path is a no-op.
func (field JSONB) SetPath(value interface{}, path ...string) AssignExpr {
	if len(path) == 0 {
		return field.value(field.RawExpr())
	}

	sql := "jsonb_set(?, array[" + strings.Repeat("?,", len(path))
	sql = strings.TrimSuffix(sql, ",") + "], ?::jsonb, true)"
	vars := make([]interface{}, 0, len(path)+2)
	vars = append(vars, field.RawExpr())
	for _, item := range path {
		vars = append(vars, item)
	}
	vars = append(vars, jsonValue(value))
	return field.value(clause.Expr{SQL: sql, Vars: vars})
}
