package field_test

import (
	"database/sql/driver"
	"reflect"
	"testing"

	"gorm.io/gen/field"
	"gorm.io/gorm/clause"
)

type jsonString string

func (j jsonString) Value() (driver.Value, error) {
	return string(j), nil
}

func assertJSONValuer(t *testing.T, got interface{}, want driver.Value) {
	t.Helper()

	valuer, ok := got.(driver.Valuer)
	if !ok {
		t.Fatalf("expected driver.Valuer, got %T", got)
	}
	value, err := valuer.Value()
	if err != nil {
		t.Fatalf("Value() returned error: %v", err)
	}
	if value != want {
		t.Fatalf("Value() = %v, want %v", value, want)
	}
}

func checkBuildExprWithJSONVars(t *testing.T, e field.Expr, result string, vars []interface{}) {
	t.Helper()

	sql, gotVars := field.BuildToString(e)
	if sql != result {
		t.Fatalf("SQL = %q, want %q", sql, result)
	}
	if len(gotVars) != len(vars) {
		t.Fatalf("Vars len = %d, want %d: %#v", len(gotVars), len(vars), gotVars)
	}
	for i, want := range vars {
		got := gotVars[i]
		if wantValuer, ok := want.(driver.Valuer); ok {
			gotValuer, ok := got.(driver.Valuer)
			if !ok {
				t.Fatalf("Vars[%d] = %T, want driver.Valuer", i, got)
			}
			gotValue, gotErr := gotValuer.Value()
			wantValue, wantErr := wantValuer.Value()
			if gotErr != nil || wantErr != nil {
				t.Fatalf("valuer errors at %d: got %v, want %v", i, gotErr, wantErr)
			}
			if gotValue != wantValue {
				t.Fatalf("Vars[%d].Value() = %v, want %v", i, gotValue, wantValue)
			}
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Vars[%d] = %#v, want %#v", i, got, want)
		}
	}
}

func TestJSONBValue(t *testing.T) {
	payload := field.NewJSONB("orders", "payload")
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

func TestJSONBBuildExpressions(t *testing.T) {
	payload := field.NewJSONB("orders", "payload")
	raw := jsonString(`{"a":1}`)

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
		{
			name: "Contains",
			expr: payload.Contains(map[string]interface{}{"status": "SUCCESS"}),
			sql:  "`orders`.`payload` @> ?::jsonb",
			vars: []interface{}{jsonString(`{"status":"SUCCESS"}`)},
		},
		{
			name: "ContainedBy",
			expr: payload.ContainedBy(map[string]interface{}{"status": "SUCCESS"}),
			sql:  "`orders`.`payload` <@ ?::jsonb",
			vars: []interface{}{jsonString(`{"status":"SUCCESS"}`)},
		},
		{
			name: "HasKey",
			expr: payload.HasKey("data"),
			sql:  "`orders`.`payload` ? ?",
			vars: []interface{}{"data"},
		},
		{
			name: "HasAnyKeys",
			expr: payload.HasAnyKeys("data", "status"),
			sql:  "`orders`.`payload` ?| array[?,?]",
			vars: []interface{}{"data", "status"},
		},
		{
			name: "HasAllKeys",
			expr: payload.HasAllKeys("data", "status"),
			sql:  "`orders`.`payload` ?& array[?,?]",
			vars: []interface{}{"data", "status"},
		},
		{
			name: "Text single",
			expr: payload.Text("status").Eq("SUCCESS"),
			sql:  "`orders`.`payload` ->> ? = ?",
			vars: []interface{}{"status", "SUCCESS"},
		},
		{
			name: "Text multi",
			expr: payload.Text("data", "trade_id").Eq("abc"),
			sql:  "`orders`.`payload` #>> array[?,?] = ?",
			vars: []interface{}{"data", "trade_id", "abc"},
		},
		{
			name: "Path Contains",
			expr: payload.Path("data").Contains(map[string]interface{}{"source": "GEP"}),
			sql:  "`orders`.`payload` -> ? @> ?::jsonb",
			vars: []interface{}{"data", jsonString(`{"source":"GEP"}`)},
		},
		{
			name: "ArrayLength",
			expr: field.NewJSONB("orders", "attachments").ArrayLength().Gt(0),
			sql:  "jsonb_array_length(`orders`.`attachments`) > ?",
			vars: []interface{}{0},
		},
		{
			name: "IsEmptyObject",
			expr: payload.IsEmptyObject(),
			sql:  "`orders`.`payload` = '{}'::jsonb",
		},
		{
			name: "IsEmptyArray",
			expr: payload.IsEmptyArray(),
			sql:  "`orders`.`payload` = '[]'::jsonb",
		},
		{
			name: "IsObject",
			expr: payload.IsObject(),
			sql:  "jsonb_typeof(`orders`.`payload`) = ?",
			vars: []interface{}{"object"},
		},
		{
			name: "IsArray",
			expr: payload.IsArray(),
			sql:  "jsonb_typeof(`orders`.`payload`) = ?",
			vars: []interface{}{"array"},
		},
		{
			name: "IsNullValue",
			expr: payload.IsNullValue(),
			sql:  "jsonb_typeof(`orders`.`payload`) = ?",
			vars: []interface{}{"null"},
		},
		{
			name: "HasAnyKeys empty",
			expr: payload.HasAnyKeys(),
			sql:  "1 = 0",
		},
		{
			name: "HasAllKeys empty",
			expr: payload.HasAllKeys(),
			sql:  "1 = 1",
		},
		{
			name: "Text empty path",
			expr: payload.Text().Eq("raw"),
			sql:  "`orders`.`payload` = ?",
			vars: []interface{}{"raw"},
		},
		{
			name: "Path empty path",
			expr: payload.Path().Contains(raw),
			sql:  "`orders`.`payload` @> ?::jsonb",
			vars: []interface{}{raw},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			checkBuildExprWithJSONVars(t, testcase.expr, testcase.sql, testcase.vars)
		})
	}
}
