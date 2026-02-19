package field

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ILike whether string matches regular expression
type ILike clause.Eq

func (like ILike) Build(builder clause.Builder) {
	builder.WriteQuoted(like.Column)
	if stmt, ok := builder.(*gorm.Statement); ok && stmt.Dialector.Name() == "postgres" {
		builder.WriteString(" ILIKE ")
	} else {
		builder.WriteString(" LIKE ")
	}
	builder.AddVar(builder, like.Value)
}

func (like ILike) NegationBuild(builder clause.Builder) {
	builder.WriteQuoted(like.Column)
	if stmt, ok := builder.(*gorm.Statement); ok && stmt.Dialector.Name() == "postgres" {
		builder.WriteString(" NOT ILIKE ")
	} else {
		builder.WriteString(" NOT LIKE ")
	}
	builder.AddVar(builder, like.Value)
}

type Filter struct {
	Column interface{}
	Cond   clause.Where
}

func (f Filter) Build(builder clause.Builder) {
	if stmt, ok := builder.(*gorm.Statement); ok && stmt.Dialector.Name() == "postgres" {
		e := clause.Expr{SQL: "? FILTER (?)", Vars: []interface{}{f.Column, f.Cond}}
		e.Build(builder)
	} else {
		e := clause.Expr{SQL: "?", Vars: []interface{}{f.Column}}
		e.Build(builder)
	}
}
