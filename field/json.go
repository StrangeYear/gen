package field

import (
	"database/sql/driver"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JSON represents a database-neutral JSON query field.
type JSON struct {
	genericsField[driver.Valuer]
}

type jsonDialectExpr struct {
	build func(*gorm.Statement)
}

func (e jsonDialectExpr) Build(builder clause.Builder) {
	stmt, ok := builder.(*gorm.Statement)
	if !ok || stmt.DB == nil || stmt.DB.Dialector == nil {
		clause.Expr{SQL: "1 = 0"}.Build(builder)
		return
	}
	e.build(stmt)
}

func jsonDialect(stmt *gorm.Statement) string {
	return strings.ToLower(stmt.DB.Dialector.Name())
}

func jsonDynamicExpr(build func(*gorm.Statement)) clause.Expr {
	return clause.Expr{SQL: "?", Vars: []interface{}{jsonDialectExpr{build: build}}}
}

func jsonQuote(stmt *gorm.Statement, value interface{}) {
	stmt.AddVar(stmt, value)
}

func jsonVar(stmt *gorm.Statement, value interface{}) {
	stmt.AddVar(stmt, value)
}

func jsonArrayPath(stmt *gorm.Statement, path []string) {
	stmt.WriteString("array[")
	for i, item := range path {
		if i > 0 {
			stmt.WriteString(",")
		}
		jsonVar(stmt, item)
	}
	stmt.WriteByte(']')
}

func mysqlJSONPath(path []string) string {
	if len(path) == 0 {
		return "$"
	}
	var builder strings.Builder
	builder.WriteByte('$')
	for _, item := range path {
		builder.WriteString(".")
		builder.WriteString(strings.ReplaceAll(item, `"`, `\"`))
	}
	return builder.String()
}

func sqliteJSONPath(path []string) string {
	return mysqlJSONPath(path)
}

func sqlServerJSONPath(path []string) string {
	return mysqlJSONPath(path)
}

// NewJSON create new JSON field.
func NewJSON(table, column string, opts ...Option) JSON {
	return JSON{genericsField: newGenerics[driver.Valuer](expr{col: toColumn(table, column, opts...)})}
}

// Eq judge equal after converting ordinary Go values to JSON.
func (field JSON) Eq(value interface{}) Expr {
	return expr{e: clause.Eq{Column: field.RawExpr(), Value: jsonValue(value)}}
}

// Neq judge not equal after converting ordinary Go values to JSON.
func (field JSON) Neq(value interface{}) Expr {
	return expr{e: clause.Neq{Column: field.RawExpr(), Value: jsonValue(value)}}
}

// In tests whether the JSON column equals any of the provided JSON values.
func (field JSON) In(values ...interface{}) Expr {
	return expr{e: clause.IN{Column: field.RawExpr(), Values: jsonValues(values)}}
}

// Value assigns an ordinary Go value to a JSON column.
func (field JSON) Value(value interface{}) AssignExpr {
	return field.value(jsonValue(value))
}

// IfNull returns IFNULL(column, value) with value converted to JSON.
func (field JSON) IfNull(value interface{}) Expr {
	return field.ifNull(jsonValue(value))
}

// Contains tests whether a JSON document contains value.
func (field JSON) Contains(value interface{}) Expr {
	return expr{e: jsonDialectExpr{build: func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "postgres":
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString("::jsonb @> ")
			jsonVar(stmt, jsonValue(value))
			stmt.WriteString("::jsonb")
		case "mysql":
			stmt.WriteString("JSON_CONTAINS(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, jsonValue(value))
			stmt.WriteByte(')')
		default:
			stmt.WriteString("1 = 0")
		}
	}}}
}

// HasKey tests whether a top-level key exists.
func (field JSON) HasKey(key string) Expr {
	return expr{e: jsonDialectExpr{build: func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "postgres":
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString("::jsonb ? ")
			jsonVar(stmt, key)
		case "mysql":
			stmt.WriteString("JSON_CONTAINS_PATH(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",'one',")
			jsonVar(stmt, mysqlJSONPath([]string{key}))
			stmt.WriteByte(')')
		case "sqlite":
			stmt.WriteString("json_type(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, sqliteJSONPath([]string{key}))
			stmt.WriteString(") IS NOT NULL")
		case "sqlserver":
			stmt.WriteString("JSON_VALUE(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, sqlServerJSONPath([]string{key}))
			stmt.WriteString(") IS NOT NULL")
		default:
			stmt.WriteString("1 = 0")
		}
	}}}
}

// HasAnyKeys tests whether any top-level key exists. Empty keys is always false.
func (field JSON) HasAnyKeys(keys ...string) Expr {
	if len(keys) == 0 {
		return expr{e: clause.Expr{SQL: "1 = 0"}}
	}
	return expr{e: jsonDialectExpr{build: func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "postgres":
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString("::jsonb ?| ")
			jsonArrayPath(stmt, keys)
		case "mysql":
			stmt.WriteString("JSON_CONTAINS_PATH(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",'one'")
			for _, key := range keys {
				stmt.WriteString(",")
				jsonVar(stmt, mysqlJSONPath([]string{key}))
			}
			stmt.WriteByte(')')
		case "sqlite", "sqlserver":
			for i, key := range keys {
				if i > 0 {
					stmt.WriteString(" OR ")
				}
				field.HasKey(key).expression().Build(stmt)
			}
		default:
			stmt.WriteString("1 = 0")
		}
	}}}
}

// HasAllKeys tests whether all top-level keys exist. Empty keys is vacuously true.
func (field JSON) HasAllKeys(keys ...string) Expr {
	if len(keys) == 0 {
		return expr{e: clause.Expr{SQL: "1 = 1"}}
	}
	return expr{e: jsonDialectExpr{build: func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "postgres":
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString("::jsonb ?& ")
			jsonArrayPath(stmt, keys)
		case "mysql":
			stmt.WriteString("JSON_CONTAINS_PATH(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",'all'")
			for _, key := range keys {
				stmt.WriteString(",")
				jsonVar(stmt, mysqlJSONPath([]string{key}))
			}
			stmt.WriteByte(')')
		case "sqlite", "sqlserver":
			for i, key := range keys {
				if i > 0 {
					stmt.WriteString(" AND ")
				}
				field.HasKey(key).expression().Build(stmt)
			}
		default:
			stmt.WriteString("1 = 0")
		}
	}}}
}

// Text extracts JSON text by path. Empty path falls back to the current column expression.
func (field JSON) Text(path ...string) String {
	if len(path) == 0 {
		return newChars[string](field.expr)
	}
	return newChars[string](field.setE(jsonDynamicExpr(func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "postgres":
			jsonQuote(stmt, field.RawExpr())
			if len(path) == 1 {
				stmt.WriteString(" ->> ")
				jsonVar(stmt, path[0])
				return
			}
			stmt.WriteString(" #>> ")
			jsonArrayPath(stmt, path)
		case "mysql":
			stmt.WriteString("JSON_UNQUOTE(JSON_EXTRACT(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, mysqlJSONPath(path))
			stmt.WriteString("))")
		case "sqlite":
			stmt.WriteString("json_extract(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, sqliteJSONPath(path))
			stmt.WriteByte(')')
		case "sqlserver":
			stmt.WriteString("JSON_VALUE(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, sqlServerJSONPath(path))
			stmt.WriteByte(')')
		default:
			jsonQuote(stmt, field.RawExpr())
		}
	})))
}

// Path extracts JSON by path. Empty path falls back to the current JSON expression.
func (field JSON) Path(path ...string) JSON {
	if len(path) == 0 {
		return field
	}
	return JSON{genericsField: newGenerics[driver.Valuer](field.setE(jsonDynamicExpr(func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "postgres":
			jsonQuote(stmt, field.RawExpr())
			if len(path) == 1 {
				stmt.WriteString(" -> ")
				jsonVar(stmt, path[0])
				return
			}
			stmt.WriteString(" #> ")
			jsonArrayPath(stmt, path)
		case "mysql":
			stmt.WriteString("JSON_EXTRACT(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, mysqlJSONPath(path))
			stmt.WriteByte(')')
		case "sqlite":
			stmt.WriteString("json_extract(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, sqliteJSONPath(path))
			stmt.WriteByte(')')
		case "sqlserver":
			stmt.WriteString("JSON_QUERY(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString(",")
			jsonVar(stmt, sqlServerJSONPath(path))
			stmt.WriteByte(')')
		default:
			jsonQuote(stmt, field.RawExpr())
		}
	})))}
}

// ArrayLength returns the JSON array length.
func (field JSON) ArrayLength() Int {
	return newNumber[int](field.setE(jsonDynamicExpr(func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "postgres":
			stmt.WriteString("jsonb_array_length(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString("::jsonb)")
		case "mysql":
			stmt.WriteString("JSON_LENGTH(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteByte(')')
		case "sqlite":
			stmt.WriteString("json_array_length(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteByte(')')
		default:
			stmt.WriteString("0")
		}
	})))
}

// TypeOf returns the database JSON type name for the field value.
func (field JSON) TypeOf() String {
	return newChars[string](field.setE(jsonDynamicExpr(func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "postgres":
			stmt.WriteString("jsonb_typeof(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString("::jsonb)")
		case "mysql":
			stmt.WriteString("LOWER(JSON_TYPE(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteString("))")
		case "sqlite":
			stmt.WriteString("json_type(")
			jsonQuote(stmt, field.RawExpr())
			stmt.WriteByte(')')
		default:
			stmt.WriteString("''")
		}
	})))
}

func (field JSON) isType(t string) Expr {
	return field.TypeOf().Eq(t)
}

// IsObject tests whether the JSON value is an object.
func (field JSON) IsObject() Expr {
	return field.isType("object")
}

// IsArray tests whether the JSON value is an array.
func (field JSON) IsArray() Expr {
	return field.isType("array")
}

// IsString tests whether the JSON value is a string.
func (field JSON) IsString() Expr {
	return field.isType("string")
}

// IsNumber tests whether the JSON value is a number.
func (field JSON) IsNumber() Expr {
	return expr{e: jsonDialectExpr{build: func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "sqlite":
			stmt.WriteByte('(')
			field.TypeOf().Eq("integer").expression().Build(stmt)
			stmt.WriteString(" OR ")
			field.TypeOf().Eq("real").expression().Build(stmt)
			stmt.WriteByte(')')
		default:
			field.isType("number").expression().Build(stmt)
		}
	}}}
}

// IsBoolean tests whether the JSON value is a boolean.
func (field JSON) IsBoolean() Expr {
	return expr{e: jsonDialectExpr{build: func(stmt *gorm.Statement) {
		switch jsonDialect(stmt) {
		case "sqlite":
			stmt.WriteByte('(')
			field.TypeOf().Eq("true").expression().Build(stmt)
			stmt.WriteString(" OR ")
			field.TypeOf().Eq("false").expression().Build(stmt)
			stmt.WriteByte(')')
		default:
			field.isType("boolean").expression().Build(stmt)
		}
	}}}
}

// IsNullValue tests whether the JSON value is JSON null.
func (field JSON) IsNullValue() Expr {
	return field.isType("null")
}
