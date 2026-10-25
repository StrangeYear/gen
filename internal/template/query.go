package template

// DefaultQuery default query
const DefaultQuery = `
var (
	Q =new(Query)
	{{range $name,$d :=.Data -}}
	{{$d.ModelStructName}} *{{$d.QueryStructName}}
	{{end -}}
)

func SetDefault(db *gorm.DB, opts ...gen.DOOption) {
	*Q = *Use(db,opts...)
	{{range $name,$d :=.Data -}}
	{{$d.ModelStructName}} = &Q.{{$d.ModelStructName}}
	{{end -}}
}

`

// QueryMethod query method template
const QueryMethod = `
func Use(db *gorm.DB, opts ...gen.DOOption) *Query {
	return &Query{
		db: db,
		{{range $name,$d :=.Data -}}
		{{$d.ModelStructName}}: new{{$d.ModelStructName}}(db,opts...),
		{{end -}}
	}
}

type Query struct{
	db *gorm.DB

	{{range $name,$d :=.Data -}}
	{{$d.ModelStructName}} {{$d.QueryStructName}}
	{{end}}
}

func (q *Query) Available() bool { return q.db != nil }

func (q *Query) UnderlyingDB() *gorm.DB { return q.db }

func (q *Query) clone(db *gorm.DB) *Query {
	return &Query{
		db: db,
		{{range $name,$d :=.Data -}}
		{{$d.ModelStructName}}: q.{{$d.ModelStructName}}.clone(db),
		{{end}}
	}
}

func (q *Query) ReadDB() *Query {
	return q.clone(q.db.Clauses(dbresolver.Read))
}

func (q *Query) WriteDB() *Query {
	return q.clone(q.db.Clauses(dbresolver.Write))
}

func (q *Query) ReplaceDB(db *gorm.DB) *Query {
	return &Query{
		db: db,
		{{range $name,$d :=.Data -}}
		{{$d.ModelStructName}}: q.{{$d.ModelStructName}}.replaceDB(db),
		{{end}}
	}
}

type queryCtx struct{ 
	{{range $name,$d :=.Data -}}
	{{$d.ModelStructName}} {{$d.ReturnObject}}
	{{end}}
}

func (q *Query) WithContext(ctx context.Context) *queryCtx  {
	return &queryCtx{
		{{range $name,$d :=.Data -}}
		{{$d.ModelStructName}}: q.{{$d.ModelStructName}}.WithContext(ctx),
		{{end}}
	}
}

func (q *Query) Transaction({{if not .WithoutContext}}ctx context.Context, {{end}}fc func(tx *Query) error, opts ...*sql.TxOptions) error {
	{{if not .WithoutContext}}if err := ctx.Err(); err != nil {
		return err
	}
	{{end -}}
	return q.db{{if not .WithoutContext}}.WithContext(ctx){{end}}.Transaction(func(tx *gorm.DB) error { return fc(q.clone(tx)) }, opts...)
}

func (q *Query) Begin({{if not .WithoutContext}}ctx context.Context, {{end}}opts ...*sql.TxOptions) *QueryTx {
	tx := q.db{{if not .WithoutContext}}.WithContext(ctx){{end}}.Begin(opts...)
	return &QueryTx{Query: q.clone(tx), Error: tx.Error}
}

type QueryTx struct {
	*Query
	Error error
}

func (q *QueryTx) Commit() error {
	return q.db.Commit().Error
}

func (q *QueryTx) Rollback() error {
	return q.db.Rollback().Error
}

func (q *QueryTx) SavePoint(name string) error {
	return q.db.SavePoint(name).Error
}

func (q *QueryTx) RollbackTo(name string) error {
	return q.db.RollbackTo(name).Error
}

`

// QueryMethodTest query method test template
const QueryMethodTest = `

const _gen_test_db_name = "gen_test.db"

var _gen_test_db *gorm.DB
var _gen_test_once sync.Once

func init() {
	InitializeDB()
	_gen_test_db.AutoMigrate(&_another{})
}

func InitializeDB() {
	_gen_test_once.Do(func() {
		var err error
		_gen_test_db, err = gorm.Open(sqlite.Open(_gen_test_db_name), &gorm.Config{})
		if err != nil {
			panic(fmt.Errorf("open sqlite %q fail: %w", _gen_test_db_name, err))
		}
	})
}

func assert(t *testing.T, methodName string, res, exp interface{}) {
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("%v() gotResult = %v, want %v", methodName, res, exp)
	}
}

type _another struct {
	ID uint64 ` + "`" + `gorm:"primaryKey"` + "`" + `
}

func (*_another) TableName() string { return "another_for_unit_test" }

func Test_Available(t *testing.T) {
	if !Use(_gen_test_db).Available() {
		t.Errorf("query.Available() == false")
	}
}

func Test_WithContext(t *testing.T) {
	query := Use(_gen_test_db)
	if !query.Available() {
		t.Errorf("query Use(_gen_test_db) fail: query.Available() == false")
	}

	type Content string
	var key, value Content = "gen_tag", "unit_test"
	qCtx := query.WithContext(context.WithValue(context.Background(), key, value))

	for _, ctx := range []context.Context{
		{{range $name,$d :=.Data -}}
		qCtx.{{$d.ModelStructName}}.UnderlyingDB().Statement.Context,
		{{end}}
	} {
		if v := ctx.Value(key); v != value {
			t.Errorf("get value from context fail, expect %q, got %q", value, v)
		}
	}
}

func Test_Transaction(t *testing.T) {
	query := Use(_gen_test_db)
	if !query.Available() {
		t.Errorf("query Use(_gen_test_db) fail: query.Available() == false")
	}

	err := query.Transaction({{if not .WithoutContext}}context.Background(), {{end}}func(tx *Query) error { return nil })
	if err != nil {
		t.Errorf("query.Transaction execute fail: %s", err)
	}

	tx := query.Begin({{if not .WithoutContext}}context.Background(){{end}})

	err = tx.SavePoint("point")
	if err != nil {
		t.Errorf("query tx SavePoint fail: %s", err)
	}
	err = tx.RollbackTo("point")
	if err != nil {
		t.Errorf("query tx RollbackTo fail: %s", err)
	}
	err = tx.Commit()
	if err != nil {
		t.Errorf("query tx Commit fail: %s", err)
	}

	err = query.Begin({{if not .WithoutContext}}context.Background(){{end}}).Rollback()
	if err != nil {
		t.Errorf("query tx Rollback fail: %s", err)
	}
}

{{if not .WithoutContext}}
func Test_TransactionContext(t *testing.T) {
    q := Use(_gen_test_db)
    type contextKey struct{}
    ctx := context.WithValue(context.Background(), contextKey{}, "outer")
    check := func(tx *Query, want string) error {
        if tx.db.Statement.Context.Value(contextKey{}) != want {
            return fmt.Errorf("transaction context not inherited")
        }
        for _, db := range []*gorm.DB{
            {{range $name,$d :=.Data -}}
            tx.{{$d.ModelStructName}}.{{$d.QueryStructName}}Do.UnderlyingDB(),
            {{end}}
        } {
            if db.Statement.Context.Value(contextKey{}) != want || db.Statement.Model == nil {
                return fmt.Errorf("DAO context or model lost")
            }
        }
        return nil
    }
    err := q.Transaction(ctx, func(tx *Query) error {
        if err := check(tx, "outer"); err != nil { return err }
        nested := context.WithValue(ctx, contextKey{}, "nested")
        rollback := fmt.Errorf("rollback nested transaction")
        record := &_another{}
        err := tx.Transaction(nested, func(inner *Query) error {
            if err := check(inner, "nested"); err != nil { return err }
            if inner.db.Statement.ConnPool != tx.db.Statement.ConnPool {
                return fmt.Errorf("nested transaction changed connection")
            }
            if err := inner.db.Create(record).Error; err != nil { return err }
            return rollback
        })
        if err != rollback { return fmt.Errorf("nested rollback: %v", err) }
        var count int64
        if err := tx.db.Model(&_another{}).Where("id = ?", record.ID).Count(&count).Error; err != nil { return err }
        if count != 0 { return fmt.Errorf("nested write survived rollback") }
        return check(tx, "outer")
    })
    if err != nil { t.Fatal(err) }
    if q.db.Statement.Context.Value(contextKey{}) != nil { t.Fatal("shared query context changed") }
    {{range $name,$d :=.Data -}}
    if q.{{$d.ModelStructName}}.{{$d.QueryStructName}}Do.UnderlyingDB().Statement.Context.Value(contextKey{}) != nil {
        t.Fatal("shared DAO context changed")
    }
    {{end}}
    tx := q.Begin(ctx)
    if tx.Error != nil { t.Fatal(tx.Error) }
    defer tx.Rollback()
    if err := check(tx.Query, "outer"); err != nil { t.Fatal(err) }
    cancelled, cancel := context.WithCancel(ctx)
    cancel()
    if err := tx.Transaction(cancelled, func(*Query) error { t.Error("cancelled callback ran"); return nil }); err == nil {
        t.Fatal("cancelled nested transaction succeeded")
    }
}

func Test_TransactionContextPoolWait(t *testing.T) {
    q := Use(_gen_test_db)
    pool, err := _gen_test_db.DB()
    if err != nil { t.Fatal(err) }
    maxOpen := pool.Stats().MaxOpenConnections
    pool.SetMaxOpenConns(1)
    defer pool.SetMaxOpenConns(maxOpen)
    held, err := pool.Conn(context.Background())
    if err != nil { t.Fatal(err) }
    defer held.Close()
    for _, begin := range []bool{false, true} {
        ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
        done := make(chan error, 1)
        go func() {
            if begin {
                tx := q.Begin(ctx)
                if tx.Error == nil { tx.Rollback() }
                done <- tx.Error
            } else {
                done <- q.Transaction(ctx, func(*Query) error { return fmt.Errorf("unexpected callback") })
            }
        }()
        select {
        case err := <-done:
            if err != context.DeadlineExceeded { t.Errorf("pool wait error = %v", err) }
        case <-time.After(2*time.Second):
            cancel()
            t.Fatal("transaction ignored deadline while waiting for connection")
        }
        cancel()
    }
}

{{end}}

`
