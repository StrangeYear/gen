package field_test

import (
	"database/sql/driver"
	"sync"
	"testing"

	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils/tests"
)

type namedDialector struct {
	tests.DummyDialector
	name string
}

func (d namedDialector) Name() string {
	return d.name
}

func buildJSONToString(t *testing.T, dialect string, e field.Expr) (string, []interface{}) {
	t.Helper()

	db, err := gorm.Open(namedDialector{name: dialect}, nil)
	if err != nil {
		t.Fatalf("gorm.Open(%q) error: %v", dialect, err)
	}
	user, err := schema.Parse(&field.User{}, &sync.Map{}, db.NamingStrategy)
	if err != nil {
		t.Fatalf("schema.Parse error: %v", err)
	}
	stmt := &gorm.Statement{DB: db, Table: user.Table, Schema: user, Clauses: map[string]clause.Clause{}}
	e.BeCond().(clause.Expression).Build(stmt)
	return stmt.SQL.String(), stmt.Vars
}

func checkJSONDialectExpr(t *testing.T, dialect string, e field.Expr, result string, vars []interface{}) {
	t.Helper()

	sql, gotVars := buildJSONToString(t, dialect, e)
	if sql != result {
		t.Fatalf("%s SQL = %q, want %q", dialect, sql, result)
	}
	if len(gotVars) != len(vars) {
		t.Fatalf("%s Vars len = %d, want %d: %#v", dialect, len(gotVars), len(vars), gotVars)
	}
	for i, want := range vars {
		if wantValuer, ok := want.(jsonString); ok {
			gotValuer, ok := gotVars[i].(interface{ Value() (driver.Value, error) })
			if !ok {
				t.Fatalf("%s Vars[%d] = %T, want driver.Valuer", dialect, i, gotVars[i])
			}
			gotValue, gotErr := gotValuer.Value()
			wantValue, wantErr := wantValuer.Value()
			if gotErr != nil || wantErr != nil {
				t.Fatalf("%s valuer errors at %d: got %v, want %v", dialect, i, gotErr, wantErr)
			}
			if gotValue != wantValue {
				t.Fatalf("%s Vars[%d].Value() = %#v, want %#v", dialect, i, gotValue, wantValue)
			}
			continue
		}
		if gotVars[i] != want {
			t.Fatalf("%s Vars[%d] = %#v, want %#v", dialect, i, gotVars[i], want)
		}
	}
}

func TestJSONValue(t *testing.T) {
	payload := field.NewJSON("orders", "payload")
	assign := payload.Value(map[string]interface{}{"a": 1})

	if assign.ColumnName().String() != "payload" {
		t.Fatalf("ColumnName() = %q, want payload", assign.ColumnName().String())
	}

	clauseEq, ok := assign.AssignExpr().(clause.Eq)
	if ok {
		assertJSONValuer(t, clauseEq.Value, `{"a":1}`)
	} else {
		t.Fatalf("AssignExpr() = %T, want clause.Eq", assign.AssignExpr())
	}

	raw := jsonString(`{"raw":true}`)
	assign = payload.Value(raw)
	clauseEq, ok = assign.AssignExpr().(clause.Eq)
	if !ok {
		t.Fatalf("AssignExpr() = %T, want clause.Eq", assign.AssignExpr())
	}
	if clauseEq.Value != raw {
		t.Fatalf("driver.Valuer was wrapped: got %T, want %T", clauseEq.Value, raw)
	}
}

func TestJSONBuildExpressions(t *testing.T) {
	payload := field.NewJSON("orders", "payload")

	testcases := []struct {
		name string
		expr field.Expr
		sql  string
		vars []interface{}
	}{
		{
			name: "Eq",
			expr: payload.Eq(map[string]interface{}{"a": 1}),
			sql:  "`orders`.`payload` = ?",
			vars: []interface{}{jsonString(`{"a":1}`)},
		},
		{
			name: "Neq",
			expr: payload.Neq(map[string]interface{}{"a": 1}),
			sql:  "`orders`.`payload` <> ?",
			vars: []interface{}{jsonString(`{"a":1}`)},
		},
		{
			name: "In",
			expr: payload.In(map[string]interface{}{"a": 1}, []string{"USD"}),
			sql:  "`orders`.`payload` IN (?,?)",
			vars: []interface{}{jsonString(`{"a":1}`), jsonString(`["USD"]`)},
		},
		{
			name: "IfNull",
			expr: payload.IfNull(map[string]interface{}{"a": 1}),
			sql:  "IFNULL(`orders`.`payload`,?)",
			vars: []interface{}{jsonString(`{"a":1}`)},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			checkBuildExprWithJSONVars(t, testcase.expr, testcase.sql, testcase.vars)
		})
	}
}

func TestJSONDialectExpressions(t *testing.T) {
	payload := field.NewJSON("orders", "payload")

	checkJSONDialectExpr(t, "postgres", payload.Contains(map[string]interface{}{"a": 1}), "`orders`.`payload`::jsonb @> ?::jsonb", []interface{}{jsonString(`{"a":1}`)})
	checkJSONDialectExpr(t, "postgres", payload.HasKey("data"), "`orders`.`payload`::jsonb ? ?", []interface{}{"data"})
	checkJSONDialectExpr(t, "postgres", payload.HasAnyKeys("data", "status"), "`orders`.`payload`::jsonb ?| array[?,?]", []interface{}{"data", "status"})
	checkJSONDialectExpr(t, "postgres", payload.HasAllKeys("data", "status"), "`orders`.`payload`::jsonb ?& array[?,?]", []interface{}{"data", "status"})
	checkJSONDialectExpr(t, "postgres", payload.Text("data", "trade_id").Eq("abc"), "`orders`.`payload` #>> array[?,?] = ?", []interface{}{"data", "trade_id", "abc"})
	checkJSONDialectExpr(t, "postgres", payload.Path("data").Contains(map[string]interface{}{"a": 1}), "`orders`.`payload` -> ?::jsonb @> ?::jsonb", []interface{}{"data", jsonString(`{"a":1}`)})
	checkJSONDialectExpr(t, "postgres", payload.ArrayLength().Gt(0), "jsonb_array_length(`orders`.`payload`::jsonb) > ?", []interface{}{0})
	checkJSONDialectExpr(t, "postgres", payload.IsObject(), "jsonb_typeof(`orders`.`payload`::jsonb) = ?", []interface{}{"object"})

	checkJSONDialectExpr(t, "mysql", payload.Contains(map[string]interface{}{"a": 1}), "JSON_CONTAINS(`orders`.`payload`,?)", []interface{}{jsonString(`{"a":1}`)})
	checkJSONDialectExpr(t, "mysql", payload.HasKey("data"), "JSON_CONTAINS_PATH(`orders`.`payload`,'one',?)", []interface{}{"$.data"})
	checkJSONDialectExpr(t, "mysql", payload.HasAnyKeys("data", "status"), "JSON_CONTAINS_PATH(`orders`.`payload`,'one',?,?)", []interface{}{"$.data", "$.status"})
	checkJSONDialectExpr(t, "mysql", payload.HasAllKeys("data", "status"), "JSON_CONTAINS_PATH(`orders`.`payload`,'all',?,?)", []interface{}{"$.data", "$.status"})
	checkJSONDialectExpr(t, "mysql", payload.Text("data", "trade_id").Eq("abc"), "JSON_UNQUOTE(JSON_EXTRACT(`orders`.`payload`,?)) = ?", []interface{}{"$.data.trade_id", "abc"})
	checkJSONDialectExpr(t, "mysql", payload.Path("data").Contains(map[string]interface{}{"a": 1}), "JSON_CONTAINS(JSON_EXTRACT(`orders`.`payload`,?),?)", []interface{}{"$.data", jsonString(`{"a":1}`)})
	checkJSONDialectExpr(t, "mysql", payload.ArrayLength().Gt(0), "JSON_LENGTH(`orders`.`payload`) > ?", []interface{}{0})
	checkJSONDialectExpr(t, "mysql", payload.IsObject(), "LOWER(JSON_TYPE(`orders`.`payload`)) = ?", []interface{}{"object"})

	checkJSONDialectExpr(t, "sqlite", payload.HasKey("data"), "json_type(`orders`.`payload`,?) IS NOT NULL", []interface{}{"$.data"})
	checkJSONDialectExpr(t, "sqlite", payload.HasAnyKeys("data", "status"), "json_type(`orders`.`payload`,?) IS NOT NULL OR json_type(`orders`.`payload`,?) IS NOT NULL", []interface{}{"$.data", "$.status"})
	checkJSONDialectExpr(t, "sqlite", payload.HasAllKeys("data", "status"), "json_type(`orders`.`payload`,?) IS NOT NULL AND json_type(`orders`.`payload`,?) IS NOT NULL", []interface{}{"$.data", "$.status"})
	checkJSONDialectExpr(t, "sqlite", payload.Text("data", "trade_id").Eq("abc"), "json_extract(`orders`.`payload`,?) = ?", []interface{}{"$.data.trade_id", "abc"})
	checkJSONDialectExpr(t, "sqlite", payload.ArrayLength().Gt(0), "json_array_length(`orders`.`payload`) > ?", []interface{}{0})
	checkJSONDialectExpr(t, "sqlite", payload.IsNumber(), "(json_type(`orders`.`payload`) = ? OR json_type(`orders`.`payload`) = ?)", []interface{}{"integer", "real"})

	checkJSONDialectExpr(t, "sqlserver", payload.HasKey("data"), "JSON_VALUE(`orders`.`payload`,?) IS NOT NULL", []interface{}{"$.data"})
	checkJSONDialectExpr(t, "sqlserver", payload.Text("data", "trade_id").Eq("abc"), "JSON_VALUE(`orders`.`payload`,?) = ?", []interface{}{"$.data.trade_id", "abc"})
	checkJSONDialectExpr(t, "sqlserver", payload.Path("data").Eq(map[string]interface{}{"a": 1}), "JSON_QUERY(`orders`.`payload`,?) = ?", []interface{}{"$.data", jsonString(`{"a":1}`)})

	checkJSONDialectExpr(t, "dummy", payload.Contains(map[string]interface{}{"a": 1}), "1 = 0", nil)
	checkJSONDialectExpr(t, "dummy", payload.HasAnyKeys(), "1 = 0", nil)
	checkJSONDialectExpr(t, "dummy", payload.HasAllKeys(), "1 = 1", nil)
}
