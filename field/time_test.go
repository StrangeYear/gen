package field_test

import (
	"sync"
	"testing"

	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func buildTimeToString(t *testing.T, dialect string, e field.Expr) string {
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
	return stmt.SQL.String()
}

func buildTimeWithVars(t *testing.T, dialect string, e field.Expr) (string, []interface{}) {
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

func TestTimeTruncDialect(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		got := buildTimeToString(t, "postgres", field.NewTime("orders", "created_at").Trunc("month"))
		want := "date_trunc('month', `orders`.`created_at`)"
		if got != want {
			t.Fatalf("SQL = %q, want %q", got, want)
		}
	})

	t.Run("mysql year", func(t *testing.T) {
		got := buildTimeToString(t, "mysql", field.NewTime("orders", "created_at").Trunc("year"))
		want := "STR_TO_DATE(DATE_FORMAT(`orders`.`created_at`, '%Y-01-01'), '%Y-%m-%d')"
		if got != want {
			t.Fatalf("SQL = %q, want %q", got, want)
		}
	})

	t.Run("mysql month", func(t *testing.T) {
		got := buildTimeToString(t, "mysql", field.NewTime("orders", "created_at").Trunc("month"))
		want := "STR_TO_DATE(DATE_FORMAT(`orders`.`created_at`, '%Y-%m-01'), '%Y-%m-%d')"
		if got != want {
			t.Fatalf("SQL = %q, want %q", got, want)
		}
	})

	t.Run("mysql week", func(t *testing.T) {
		got := buildTimeToString(t, "mysql", field.NewTime("orders", "created_at").Trunc("week"))
		want := "DATE_SUB(DATE(`orders`.`created_at`), INTERVAL WEEKDAY(`orders`.`created_at`) DAY)"
		if got != want {
			t.Fatalf("SQL = %q, want %q", got, want)
		}
	})

	t.Run("mysql day", func(t *testing.T) {
		got := buildTimeToString(t, "mysql", field.NewValuerTime[password]("orders", "created_at").Trunc("day"))
		want := "DATE(`orders`.`created_at`)"
		if got != want {
			t.Fatalf("SQL = %q, want %q", got, want)
		}
	})

	t.Run("postgres timezone", func(t *testing.T) {
		sql, vars := buildTimeWithVars(t, "postgres", field.NewTime("orders", "created_at").Trunc("month", "Asia/Shanghai"))
		want := "date_trunc('month', timezone(?, `orders`.`created_at`))"
		if sql != want {
			t.Fatalf("SQL = %q, want %q", sql, want)
		}
		if len(vars) != 1 || vars[0] != "Asia/Shanghai" {
			t.Fatalf("Vars = %#v, want %#v", vars, []interface{}{"Asia/Shanghai"})
		}
	})

	t.Run("mysql timezone month", func(t *testing.T) {
		sql, vars := buildTimeWithVars(t, "mysql", field.NewTime("orders", "created_at").Trunc("month", "Asia/Shanghai"))
		want := "STR_TO_DATE(DATE_FORMAT(CONVERT_TZ(`orders`.`created_at`, @@session.time_zone, ?), '%Y-%m-01'), '%Y-%m-%d')"
		if sql != want {
			t.Fatalf("SQL = %q, want %q", sql, want)
		}
		if len(vars) != 1 || vars[0] != "Asia/Shanghai" {
			t.Fatalf("Vars = %#v, want %#v", vars, []interface{}{"Asia/Shanghai"})
		}
	})

	t.Run("mysql timezone week", func(t *testing.T) {
		sql, vars := buildTimeWithVars(t, "mysql", field.NewTime("orders", "created_at").Trunc("week", "Asia/Shanghai"))
		want := "DATE_SUB(DATE(CONVERT_TZ(`orders`.`created_at`, @@session.time_zone, ?)), INTERVAL WEEKDAY(CONVERT_TZ(`orders`.`created_at`, @@session.time_zone, ?)) DAY)"
		if sql != want {
			t.Fatalf("SQL = %q, want %q", sql, want)
		}
		wantVars := []interface{}{"Asia/Shanghai", "Asia/Shanghai"}
		if len(vars) != len(wantVars) {
			t.Fatalf("Vars = %#v, want %#v", vars, wantVars)
		}
		for i := range wantVars {
			if vars[i] != wantVars[i] {
				t.Fatalf("Vars = %#v, want %#v", vars, wantVars)
			}
		}
	})

	t.Run("mysql source and target timezone", func(t *testing.T) {
		sql, vars := buildTimeWithVars(t, "mysql", field.NewTime("orders", "created_at").Trunc("day", "SYSTEM", "Asia/Shanghai"))
		want := "DATE(CONVERT_TZ(`orders`.`created_at`, ?, ?))"
		if sql != want {
			t.Fatalf("SQL = %q, want %q", sql, want)
		}
		if len(vars) != 2 || vars[0] != "SYSTEM" || vars[1] != "Asia/Shanghai" {
			t.Fatalf("Vars = %#v, want %#v", vars, []interface{}{"SYSTEM", "Asia/Shanghai"})
		}
	})

	t.Run("postgres source and target timezone", func(t *testing.T) {
		sql, vars := buildTimeWithVars(t, "postgres", field.NewTime("orders", "created_at").Trunc("day", "America/New_York", "Asia/Shanghai"))
		want := "date_trunc('day', timezone(?, timezone(?, `orders`.`created_at`)))"
		if sql != want {
			t.Fatalf("SQL = %q, want %q", sql, want)
		}
		if len(vars) != 2 || vars[0] != "Asia/Shanghai" || vars[1] != "America/New_York" {
			t.Fatalf("Vars = %#v, want %#v", vars, []interface{}{"Asia/Shanghai", "America/New_York"})
		}
	})
}
