package field

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Time time type field
type Time struct {
	ValuerTime[time.Time]
}

func newValuerTime[T any](e expr) ValuerTime[T] {
	return ValuerTime[T]{genericsField: newGenerics[T](e)}
}

func newTime(e expr) Time {
	return Time{ValuerTime: newValuerTime[time.Time](e)}
}

func truncUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "year":
		return "year"
	case "month":
		return "month"
	case "week":
		return "week"
	case "day":
		return "day"
	default:
		return "day"
	}
}

func truncTimezones(timezones []string) (sourceTimezone, targetTimezone string) {
	if len(timezones) == 0 {
		return "", ""
	}
	targetTimezone = strings.TrimSpace(timezones[0])
	if len(timezones) > 1 {
		sourceTimezone = targetTimezone
		targetTimezone = strings.TrimSpace(timezones[1])
	}
	if targetTimezone == "" {
		return "", ""
	}
	return sourceTimezone, targetTimezone
}

type timeTruncExpr struct {
	unit           string
	col            interface{}
	sourceTimezone string
	targetTimezone string
}

func (e timeTruncExpr) Build(builder clause.Builder) {
	unit := truncUnit(e.unit)
	stmt, _ := builder.(*gorm.Statement)
	dialect := ""
	if stmt != nil && stmt.DB != nil && stmt.DB.Dialector != nil {
		dialect = strings.ToLower(stmt.DB.Dialector.Name())
	}

	var sql string
	var vars []interface{}
	switch dialect {
	case "mysql":
		colSQL := "?"
		colVars := []interface{}{e.col}
		if e.targetTimezone != "" {
			colSQL = "CONVERT_TZ(?, ?, ?)"
			sourceTimezone := e.sourceTimezone
			if sourceTimezone == "" {
				colVars = []interface{}{e.col, gorm.Expr("@@session.time_zone"), e.targetTimezone}
			} else {
				colVars = []interface{}{e.col, sourceTimezone, e.targetTimezone}
			}
		}

		switch unit {
		case "year":
			sql = fmt.Sprintf("STR_TO_DATE(DATE_FORMAT(%s, '%%Y-01-01'), '%%Y-%%m-%%d')", colSQL)
			vars = colVars
		case "month":
			sql = fmt.Sprintf("STR_TO_DATE(DATE_FORMAT(%s, '%%Y-%%m-01'), '%%Y-%%m-%%d')", colSQL)
			vars = colVars
		case "week":
			sql = fmt.Sprintf("DATE_SUB(DATE(%s), INTERVAL WEEKDAY(%s) DAY)", colSQL, colSQL)
			vars = append(append([]interface{}{}, colVars...), colVars...)
		default:
			sql = fmt.Sprintf("DATE(%s)", colSQL)
			vars = colVars
		}
	default:
		if e.targetTimezone != "" {
			if e.sourceTimezone == "" {
				sql = fmt.Sprintf("date_trunc('%s', timezone(?, ?))", unit)
				vars = []interface{}{e.targetTimezone, e.col}
			} else {
				sql = fmt.Sprintf("date_trunc('%s', timezone(?, timezone(?, ?)))", unit)
				vars = []interface{}{e.targetTimezone, e.sourceTimezone, e.col}
			}
		} else {
			sql = fmt.Sprintf("date_trunc('%s', ?)", unit)
			vars = []interface{}{e.col}
		}
	}
	clause.Expr{SQL: sql, Vars: vars}.Build(builder)
}

// ValuerTime time-like valuer type field
type ValuerTime[T any] struct {
	genericsField[T]
}

// Between ...
func (field ValuerTime[T]) Between(left T, right T) Expr {
	return field.between([]interface{}{left, right})
}

// NotBetween ...
func (field ValuerTime[T]) NotBetween(left T, right T) Expr {
	return Not(field.Between(left, right))
}

// NotIn ...
func (field ValuerTime[T]) NotIn(values ...T) Expr {
	return expr{e: clause.Not(field.In(values...).expression())}
}

// Add ...
func (field ValuerTime[T]) Add(value time.Duration) ValuerTime[T] {
	return newValuerTime[T](field.add(value))
}

// Sub ...
func (field ValuerTime[T]) Sub(value time.Duration) ValuerTime[T] {
	return newValuerTime[T](field.sub(value))
}

// Date convert to data, equal to "DATE(time_expr)"
func (field ValuerTime[T]) Date() ValuerTime[T] {
	return newValuerTime[T](expr{e: clause.Expr{SQL: "DATE(?)", Vars: []interface{}{field.RawExpr()}}})
}

// Trunc truncates time to year, month, week, or day.
// With one timezone argument, it converts to that timezone before truncating.
// With two timezone arguments, it converts from the first timezone to the second.
func (field ValuerTime[T]) Trunc(unit string, timezone ...string) ValuerTime[T] {
	sourceTimezone, targetTimezone := truncTimezones(timezone)
	return newValuerTime[T](expr{e: timeTruncExpr{unit: unit, col: field.RawExpr(), sourceTimezone: sourceTimezone, targetTimezone: targetTimezone}})
}

// DateDiff equal to DATADIFF(self, value)
func (field ValuerTime[T]) DateDiff(value T) Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "DATEDIFF(?,?)", Vars: []interface{}{field.RawExpr(), value}}})
}

// DateFormat equal to DATE_FORMAT(self, value)
func (field ValuerTime[T]) DateFormat(value string) String {
	return newChars[string](expr{e: clause.Expr{SQL: "DATE_FORMAT(?,?)", Vars: []interface{}{field.RawExpr(), value}}})
}

// Now return result of NOW()
func (field ValuerTime[T]) Now() ValuerTime[T] {
	return newValuerTime[T](expr{e: clause.Expr{SQL: "NOW()"}})
}

// CurDate return result of CURDATE()
func (field ValuerTime[T]) CurDate() ValuerTime[T] {
	return newValuerTime[T](expr{e: clause.Expr{SQL: "CURDATE()"}})
}

// CurTime return result of CURTIME()
func (field ValuerTime[T]) CurTime() ValuerTime[T] {
	return newValuerTime[T](expr{e: clause.Expr{SQL: "CURTIME()"}})
}

// DayName equal to DAYNAME(self)
func (field ValuerTime[T]) DayName() String {
	return newChars[string](expr{e: clause.Expr{SQL: "DAYNAME(?)", Vars: []interface{}{field.RawExpr()}}})

}

// MonthName equal to MONTHNAME(self)
func (field ValuerTime[T]) MonthName() String {
	return newChars[string](expr{e: clause.Expr{SQL: "MONTHNAME(?)", Vars: []interface{}{field.RawExpr()}}})
}

// Year equal to YEAR(self)
func (field ValuerTime[T]) Year() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "YEAR(?)", Vars: []interface{}{field.RawExpr()}}})
}

// Month equal to MONTH(self)
func (field ValuerTime[T]) Month() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "MONTH(?)", Vars: []interface{}{field.RawExpr()}}})
}

// Day equal to DAY(self)
func (field ValuerTime[T]) Day() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "DAY(?)", Vars: []interface{}{field.RawExpr()}}})
}

// Hour equal to HOUR(self)
func (field ValuerTime[T]) Hour() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "HOUR(?)", Vars: []interface{}{field.RawExpr()}}})
}

// Minute equal to MINUTE(self)
func (field ValuerTime[T]) Minute() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "MINUTE(?)", Vars: []interface{}{field.RawExpr()}}})
}

// Second equal to SECOND(self)
func (field ValuerTime[T]) Second() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "SECOND(?)", Vars: []interface{}{field.RawExpr()}}})
}

// MicroSecond equal to MICROSECOND(self)
func (field ValuerTime[T]) MicroSecond() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "MICROSECOND(?)", Vars: []interface{}{field.RawExpr()}}})
}

// DayOfWeek equal to DAYOFWEEK(self)
func (field ValuerTime[T]) DayOfWeek() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "DAYOFWEEK(?)", Vars: []interface{}{field.RawExpr()}}})
}

// DayOfMonth equal to DAYOFMONTH(self)
func (field ValuerTime[T]) DayOfMonth() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "DAYOFMONTH(?)", Vars: []interface{}{field.RawExpr()}}})
}

// DayOfYear equal to DAYOFYEAR(self)
func (field ValuerTime[T]) DayOfYear() Number[int] {
	return newNumber[int](expr{e: clause.Expr{SQL: "DAYOFYEAR(?)", Vars: []interface{}{field.RawExpr()}}})
}

// FromDays equal to FROM_DAYS(self)
func (field ValuerTime[T]) FromDays(value int) ValuerTime[T] {
	return newValuerTime[T](expr{e: clause.Expr{SQL: fmt.Sprintf("FROM_DAYS(%d)", value)}})
}

// FromUnixTime equal to FROM_UNIXTIME(self)
func (field ValuerTime[T]) FromUnixTime(value int) ValuerTime[T] {
	return newValuerTime[T](expr{e: clause.Expr{SQL: fmt.Sprintf("FROM_UNIXTIME(%d)", value)}})
}

// Zero set zero value
func (field ValuerTime[T]) Zero() AssignExpr {
	var zero T
	return field.value(zero)
}

// Add ...
func (field Time) Add(value time.Duration) Time {
	return newTime(field.add(value))
}

// Sub ...
func (field Time) Sub(value time.Duration) Time {
	return newTime(field.sub(value))
}

// Date convert to data, equal to "DATE(time_expr)"
func (field Time) Date() Time {
	return newTime(expr{e: clause.Expr{SQL: "DATE(?)", Vars: []interface{}{field.RawExpr()}}})
}

// Trunc truncates time to year, month, week, or day.
// With one timezone argument, it converts to that timezone before truncating.
// With two timezone arguments, it converts from the first timezone to the second.
func (field Time) Trunc(unit string, timezone ...string) Time {
	sourceTimezone, targetTimezone := truncTimezones(timezone)
	return newTime(expr{e: timeTruncExpr{unit: unit, col: field.RawExpr(), sourceTimezone: sourceTimezone, targetTimezone: targetTimezone}})
}

// Now return result of NOW()
func (field Time) Now() Time {
	return newTime(expr{e: clause.Expr{SQL: "NOW()"}})
}

// CurDate return result of CURDATE()
func (field Time) CurDate() Time {
	return newTime(expr{e: clause.Expr{SQL: "CURDATE()"}})
}

// CurTime return result of CURTIME()
func (field Time) CurTime() Time {
	return newTime(expr{e: clause.Expr{SQL: "CURTIME()"}})
}

// FromDays equal to FROM_DAYS(self)
func (field Time) FromDays(value int) Time {
	return newTime(expr{e: clause.Expr{SQL: fmt.Sprintf("FROM_DAYS(%d)", value)}})
}

// FromUnixTime equal to FROM_UNIXTIME(self)
func (field Time) FromUnixTime(value int) Time {
	return newTime(expr{e: clause.Expr{SQL: fmt.Sprintf("FROM_UNIXTIME(%d)", value)}})
}
