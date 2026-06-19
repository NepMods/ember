package ember_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	ember "github.com/NepMods/ember"
	_ "github.com/mattn/go-sqlite3"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func openTestDB(t *testing.T) *ember.DB {
	t.Helper()
	db, err := ember.Open(ember.Config{
		Driver: "sqlite3",
		Master: ":memory:",
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createUsersTable(t *testing.T, db *ember.DB) {
	t.Helper()
	s := testSchema(db)
	if err := s.Create("users", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("name", 100)
		bp.String("email", 255).Unique()
		bp.String("status", 50).Default("active")
		bp.Timestamp("created_at").Nullable()
		bp.Timestamp("updated_at").Nullable()
		bp.Timestamp("deleted_at").Nullable()
	}); err != nil {
		t.Fatalf("createUsersTable: %v", err)
	}
}

func testSchema(db *ember.DB) *ember.Schema {
	// Use exported constructor (we add it in migration.go)
	return ember.NewSchema(db)
}

// ─── Models ───────────────────────────────────────────────────────────────────

type User struct {
	ID        int64      `ember:"column:id;primaryKey;autoIncr"`
	Name      string     `ember:"column:name"`
	Email     string     `ember:"column:email"`
	Status    string     `ember:"column:status"`
	CreatedAt time.Time  `ember:"column:created_at"`
	UpdatedAt time.Time  `ember:"column:updated_at"`
	DeletedAt *time.Time `ember:"column:deleted_at"`
}

// ─── Dialect tests ────────────────────────────────────────────────────────────

func TestDialects(t *testing.T) {
	pg, _ := ember.GetDialect("postgres")
	my, _ := ember.GetDialect("mysql")
	sq, _ := ember.GetDialect("sqlite3")

	if pg.Name() != "postgres" {
		t.Errorf("expected postgres, got %s", pg.Name())
	}
	if my.Name() != "mysql" {
		t.Errorf("expected mysql, got %s", my.Name())
	}
	if sq.Name() != "sqlite3" {
		t.Errorf("expected sqlite3, got %s", sq.Name())
	}

	// Placeholder
	if pg.Placeholder(1) != "$1" {
		t.Errorf("postgres placeholder: expected $1, got %s", pg.Placeholder(1))
	}
	if my.Placeholder(5) != "?" {
		t.Errorf("mysql placeholder: expected ?, got %s", my.Placeholder(5))
	}
	if sq.Placeholder(3) != "?" {
		t.Errorf("sqlite placeholder: expected ?, got %s", sq.Placeholder(3))
	}

	// QuoteIdentifier
	if pg.QuoteIdentifier("users") != `"users"` {
		t.Errorf("pg quote: %s", pg.QuoteIdentifier("users"))
	}
	if my.QuoteIdentifier("users") != "`users`" {
		t.Errorf("mysql quote: %s", my.QuoteIdentifier("users"))
	}
}

// ─── Builder SQL compilation tests ───────────────────────────────────────────

func TestBuilderToSQL_Select(t *testing.T) {
	db := openTestDB(t)
	sql, args := db.Table("users").
		Select("id", "name", "email").
		Where("id", "=", 42).
		Where("status", "=", "active").
		OrderBy("name", "ASC").
		Limit(10).
		Offset(20).
		ToSQL()

	t.Logf("SQL: %s | Args: %v", sql, args)
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuilderToSQL_WhereIn(t *testing.T) {
	db := openTestDB(t)
	sql, args := db.Table("users").
		WhereIn("id", 1, 2, 3).
		ToSQL()
	t.Logf("WhereIn SQL: %s | Args: %v", sql, args)
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestBuilderToSQL_WhereBetween(t *testing.T) {
	db := openTestDB(t)
	sql, args := db.Table("orders").WhereBetween("amount", 100, 500).ToSQL()
	t.Logf("Between SQL: %s | Args: %v", sql, args)
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuilderToSQL_WhereNull(t *testing.T) {
	db := openTestDB(t)
	sql, _ := db.Table("users").WhereNull("deleted_at").ToSQL()
	if sql == "" {
		t.Error("WhereNull returned empty SQL")
	}
}

func TestBuilderToSQL_WhereGroup(t *testing.T) {
	db := openTestDB(t)
	sql, args := db.Table("users").
		Where("active", "=", true).
		WhereGroup(func(b *ember.Builder) {
			b.Where("role", "=", "admin").OrWhere("role", "=", "mod")
		}).
		ToSQL()
	t.Logf("WhereGroup SQL: %s | Args: %v", sql, args)
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestBuilderToSQL_Join(t *testing.T) {
	db := openTestDB(t)
	sql, _ := db.Table("users").
		Select("users.id", "orders.total").
		Join("orders", "users.id", "=", "orders.user_id").
		LeftJoin("profiles", "users.id", "=", "profiles.user_id").
		ToSQL()
	if sql == "" {
		t.Error("Join returned empty SQL")
	}
}

func TestBuilderToSQL_GroupBy(t *testing.T) {
	db := openTestDB(t)
	sql, args := db.Table("orders").
		Select("status").
		SelectRaw("COUNT(*) AS cnt").
		GroupBy("status").
		Having("cnt", ">", 5).
		ToSQL()
	if sql == "" || len(args) == 0 {
		t.Errorf("GroupBy produced empty SQL or args: %q %v", sql, args)
	}
}

func TestBuilderToSQL_Insert(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	id, err := db.Table("users").Insert(ctx, map[string]interface{}{
		"name":   "Alice",
		"email":  "alice@example.com",
		"status": "active",
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	t.Logf("Inserted ID: %d", id)
}

func TestBuilderToSQL_InsertBatch(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	rows := []map[string]interface{}{
		{"name": "Bob", "email": "bob@example.com", "status": "active"},
		{"name": "Charlie", "email": "charlie@example.com", "status": "inactive"},
	}
	_, err := db.Table("users").InsertBatch(ctx, rows)
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}

	count, err := db.Table("users").Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}
}

func TestBuilderToSQL_Update(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	db.Table("users").Insert(ctx, map[string]interface{}{
		"name": "Dave", "email": "dave@example.com", "status": "active",
	})
	affected, err := db.Table("users").Where("email", "=", "dave@example.com").
		Update(ctx, map[string]interface{}{"status": "inactive"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 affected row, got %d", affected)
	}
}

func TestBuilderToSQL_Delete(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	db.Table("users").Insert(ctx, map[string]interface{}{
		"name": "Eve", "email": "eve@example.com", "status": "active",
	})
	affected, err := db.Table("users").Where("email", "=", "eve@example.com").Delete(ctx)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 affected row, got %d", affected)
	}
}

func TestBuilderGet_Maps(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		db.Table("users").Insert(ctx, map[string]interface{}{
			"name":   fmt.Sprintf("User%d", i),
			"email":  fmt.Sprintf("user%d@example.com", i),
			"status": "active",
		})
	}

	var rows []map[string]interface{}
	err := db.Table("users").OrderBy("id", "ASC").Get(ctx, &rows)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 rows, got %d", len(rows))
	}
}

func TestBuilderExists(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	exists, err := db.Table("users").Where("email", "=", "nobody@example.com").Exists(ctx)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("expected no rows")
	}
}

func TestBuilderIncrement(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	db.Raw("CREATE TABLE counters (id INTEGER PRIMARY KEY, hits INTEGER NOT NULL DEFAULT 0)").Exec(ctx)
	db.Raw("INSERT INTO counters (id, hits) VALUES (1, 10)").Exec(ctx)

	affected, err := db.Table("counters").Where("id", "=", 1).Increment(ctx, "hits")
	if err != nil {
		t.Fatalf("Increment: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 affected row, got %d", affected)
	}

	var hits int64
	err = db.Raw("SELECT hits FROM counters WHERE id = 1").Value(ctx, &hits)
	if err != nil {
		t.Fatalf("read after increment: %v", err)
	}
	if hits != 11 {
		t.Errorf("expected hits=11 after increment, got %d", hits)
	}
}

func TestBuilderChunk(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		db.Table("users").Insert(ctx, map[string]interface{}{
			"name":  fmt.Sprintf("U%d", i),
			"email": fmt.Sprintf("u%d@x.com", i),
		})
	}

	total := 0
	err := db.Table("users").Chunk(ctx, 3, func(rows []map[string]interface{}) bool {
		total += len(rows)
		return true // continue
	})
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}
	if total != 10 {
		t.Errorf("expected 10 rows chunked, got %d", total)
	}
}

// ─── Transaction tests ────────────────────────────────────────────────────────

func TestTransaction_Commit(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	err := db.Transaction(ctx, func(tx *ember.Tx) error {
		_, err := tx.Table("users").Insert(ctx, map[string]interface{}{
			"name": "TxUser", "email": "tx@example.com", "status": "active",
		})
		return err
	})
	if err != nil {
		t.Fatalf("transaction commit: %v", err)
	}

	count, _ := db.Table("users").Count(ctx)
	if count != 1 {
		t.Errorf("expected 1 committed row, got %d", count)
	}
}

func TestTransaction_Rollback(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	err := db.Transaction(ctx, func(tx *ember.Tx) error {
		if _, err := tx.Table("users").Insert(ctx, map[string]interface{}{
			"name": "RollbackUser", "email": "rb@example.com",
		}); err != nil {
			return err
		}
		return fmt.Errorf("forced rollback")
	})
	if err == nil {
		t.Fatal("expected error from rollback transaction")
	}

	count, _ := db.Table("users").Count(ctx)
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

// ─── Raw SQL tests ────────────────────────────────────────────────────────────

func TestRawQuery(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	db.Table("users").Insert(ctx, map[string]interface{}{
		"name": "RawUser", "email": "raw@example.com", "status": "active",
	})

	var rows []map[string]interface{}
	err := db.Raw("SELECT id, name, email FROM users WHERE status = ?", "active").Scan(ctx, &rows)
	if err != nil {
		t.Fatalf("Raw scan: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(rows))
	}
}

// ─── Schema parsing tests ─────────────────────────────────────────────────────

func TestParseSchema(t *testing.T) {
	type Post struct {
		ID        int64  `ember:"column:id;primaryKey;autoIncr"`
		Title     string `ember:"column:title"`
		AuthorID  int64  `ember:"column:author_id"`
		Published bool   `ember:"column:published"`
	}

	s, err := ember.ParseSchema(&Post{})
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if s.TableName != "posts" {
		t.Errorf("expected table name 'posts', got '%s'", s.TableName)
	}
	if s.PrimaryKey == nil || s.PrimaryKey.ColumnName != "id" {
		t.Errorf("expected primary key 'id'")
	}
	if len(s.Fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(s.Fields))
	}
}

// ─── Blueprint tests ──────────────────────────────────────────────────────────

func TestBlueprint_CreateSQL(t *testing.T) {
	pgDialect, _ := ember.GetDialect("postgres")
	myDialect, _ := ember.GetDialect("mysql")
	sqDialect, _ := ember.GetDialect("sqlite3")
	dialects := []ember.Dialect{pgDialect, myDialect, sqDialect}
	for _, d := range dialects {
		bp := ember.NewBlueprintForTest("users")
		bp.ID()
		bp.String("name", 100)
		bp.String("email", 255).Unique()
		bp.Timestamps()
		bp.SoftDeletes()

		sql := bp.ToCreateSQL(d)
		t.Logf("[%s] CREATE SQL:\n%s\n", d.Name(), sql)
		if sql == "" {
			t.Errorf("[%s] empty CREATE SQL", d.Name())
		}
	}
}

func TestBlueprint_ForeignKey(t *testing.T) {
	d, _ := ember.GetDialect("postgres")
	bp := ember.NewBlueprintForTest("posts")
	bp.ID()
	bp.UnsignedBigInteger("user_id")
	bp.Foreign("user_id").References("id").On("users").CascadeOnDelete()

	sql := bp.ToCreateSQL(d)
	if sql == "" {
		t.Error("ForeignKey SQL is empty")
	}
}

// ─── Migration tests ──────────────────────────────────────────────────────────

type CreateUsersTableMigration struct{}

func (m *CreateUsersTableMigration) Version() string { return "2024_01_01_create_users_table" }
func (m *CreateUsersTableMigration) Up(s *ember.Schema) error {
	return s.Create("mig_users", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("name", 100)
		bp.Timestamps()
	})
}
func (m *CreateUsersTableMigration) Down(s *ember.Schema) error {
	return s.DropIfExists("mig_users")
}

func TestMigrator_RunAndRollback(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	migrator := ember.NewMigrator(db, &CreateUsersTableMigration{})

	if err := migrator.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	statuses, err := migrator.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 || !statuses[0].Ran {
		t.Error("expected migration to be marked as ran")
	}

	if err := migrator.Rollback(ctx, 1); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	statuses, _ = migrator.Status(ctx)
	if statuses[0].Ran {
		t.Error("expected migration to be rolled back")
	}
}

// ─── Model layer tests ────────────────────────────────────────────────────────

func TestModel_CreateAndFind(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	u := &User{Name: "Model User", Email: "model@example.com", Status: "active"}
	if err := db.Model().Create(ctx, u); err != nil {
		t.Fatalf("Model.Create: %v", err)
	}
	if u.ID == 0 {
		t.Error("expected ID to be set after Create")
	}

	found := &User{}
	if err := db.Model().Find(ctx, found, u.ID); err != nil {
		t.Fatalf("Model.Find: %v", err)
	}
	if found.Name != "Model User" {
		t.Errorf("expected name 'Model User', got '%s'", found.Name)
	}
}

func TestModel_Update(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	u := &User{Name: "Before", Email: "before@example.com", Status: "active"}
	db.Model().Create(ctx, u)

	u.Name = "After"
	if err := db.Model().Update(ctx, u); err != nil {
		t.Fatalf("Model.Update: %v", err)
	}

	found := &User{}
	if err := db.Model().Find(ctx, found, u.ID); err != nil {
		t.Fatalf("Find after update: %v", err)
	}
	if found.Name != "After" {
		t.Errorf("expected 'After', got '%s'", found.Name)
	}
}

func TestModel_SoftDelete(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	u := &User{Name: "Soft", Email: "soft@example.com", Status: "active"}
	if err := db.Model().Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := db.Model().Delete(ctx, u); err != nil {
		t.Fatalf("Model.Delete: %v", err)
	}

	// Verify deleted_at is set
	if u.DeletedAt == nil || u.DeletedAt.IsZero() {
		t.Error("expected deleted_at to be set after soft delete")
	}

	// Should not find soft-deleted record
	found := &User{}
	err := db.Model().Find(ctx, found, u.ID)
	if err == nil {
		t.Error("expected ErrNotFound for soft-deleted record")
	}
}

// ─── Connection Manager tests ─────────────────────────────────────────────────

func TestConnectionManager(t *testing.T) {
	mgr := ember.NewConnectionManager()

	db1, _ := ember.Open(ember.Config{Driver: "sqlite3", Master: ":memory:"})
	db2, _ := ember.Open(ember.Config{Driver: "sqlite3", Master: ":memory:"})
	t.Cleanup(func() { db1.Close(); db2.Close() })

	mgr.AddDB("default", db1)
	mgr.AddDB("reporting", db2)

	if len(mgr.Names()) != 2 {
		t.Errorf("expected 2 connections, got %d", len(mgr.Names()))
	}

	// Duplicate registration should fail
	err := mgr.AddDB("default", db1)
	if err == nil {
		t.Error("expected error for duplicate connection name")
	}

	// Remove and close
	mgr.Remove("reporting")
	if len(mgr.Names()) != 1 {
		t.Errorf("expected 1 connection after remove, got %d", len(mgr.Names()))
	}
}

// ─── Scope tests ──────────────────────────────────────────────────────────────

func TestScopes(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	db.Table("users").Insert(ctx, map[string]interface{}{"name": "A", "email": "a@x.com", "status": "active"})
	db.Table("users").Insert(ctx, map[string]interface{}{"name": "B", "email": "b@x.com", "status": "inactive"})

	b := db.Table("users")
	b = ember.ApplyScopes(b, ember.ActiveScope)
	var rows []map[string]interface{}
	b.Get(ctx, &rows)
	if len(rows) != 1 {
		t.Errorf("expected 1 active user, got %d", len(rows))
	}
}

// ─── Relationship parsing tests ──────────────────────────────────────────────

type RelationPost struct {
	ID     int64
	Title  string
	UserID int64
	User   *RelationUserTag `ember:"relation:belongsTo;foreignKey:user_id"`
}

type RelationUserTag struct {
	ID      int64
	Name    string
	Posts   []RelationPost    `ember:"relation:hasMany;foreignKey:user_id"`
	Profile *RelationProfile  `ember:"relation:hasOne;foreignKey:user_id"`
	Roles   []RelationRoleTag `ember:"relation:belongsToMany;pivot:role_user_tag;pivotFK:user_tag_id;pivotRK:role_tag_id"`
}

type RelationRoleTag struct {
	ID   int64
	Name string
}

type RelationProfile struct {
	ID     int64
	UserID int64
	Bio    string
}

func TestParseRelations(t *testing.T) {
	s, err := ember.ParseSchema(&RelationUserTag{})
	if err != nil {
		t.Fatalf("ParseSchema(RelationUserTag): %v", err)
	}

	// hasMany: Posts
	rel, ok := s.Relations["Posts"]
	if !ok {
		t.Fatal("expected Posts relation")
	}
	if rel.Type != ember.HasManyRelation {
		t.Errorf("expected HasMany for Posts, got %v", rel.Type)
	}
	if !rel.IsSlice {
		t.Error("expected IsSlice=true for hasMany")
	}
	if rel.ForeignKey != "user_id" {
		t.Errorf("expected foreignKey 'user_id', got '%s'", rel.ForeignKey)
	}
	if rel.RelatedType.Name() != "RelationPost" {
		t.Errorf("expected RelatedType 'RelationPost', got '%s'", rel.RelatedType.Name())
	}

	// hasOne: Profile
	rel, ok = s.Relations["Profile"]
	if !ok {
		t.Fatal("expected Profile relation")
	}
	if rel.Type != ember.HasOneRelation {
		t.Errorf("expected HasOne for Profile, got %v", rel.Type)
	}
	if rel.IsSlice {
		t.Error("expected IsSlice=false for hasOne")
	}
	if rel.ForeignKey != "user_id" {
		t.Errorf("expected foreignKey 'user_id', got '%s'", rel.ForeignKey)
	}

	// belongsToMany: Roles
	rel, ok = s.Relations["Roles"]
	if !ok {
		t.Fatal("expected Roles relation")
	}
	if rel.Type != ember.BelongsToManyRelation {
		t.Errorf("expected BelongsToMany for Roles, got %v", rel.Type)
	}
	if !rel.IsSlice {
		t.Error("expected IsSlice=true for belongsToMany")
	}
	if rel.PivotTable != "role_user_tag" {
		t.Errorf("expected pivot 'role_user_tag', got '%s'", rel.PivotTable)
	}
	if rel.PivotFK != "user_tag_id" {
		t.Errorf("expected pivotFK 'user_tag_id', got '%s'", rel.PivotFK)
	}
	if rel.PivotRK != "role_tag_id" {
		t.Errorf("expected pivotRK 'role_tag_id', got '%s'", rel.PivotRK)
	}

	// Parse RelationPost which has belongsTo
	s2, err := ember.ParseSchema(&RelationPost{})
	if err != nil {
		t.Fatalf("ParseSchema(RelationPost): %v", err)
	}

	rel, ok = s2.Relations["User"]
	if !ok {
		t.Fatal("expected User relation")
	}
	if rel.Type != ember.BelongsToRelation {
		t.Errorf("expected BelongsTo for User, got %v", rel.Type)
	}
	if rel.IsSlice {
		t.Error("expected IsSlice=false for belongsTo")
	}
	if rel.ForeignKey != "user_id" {
		t.Errorf("expected foreignKey 'user_id', got '%s'", rel.ForeignKey)
	}
}

// ─── Eager loading tests ─────────────────────────────────────────────────────

func TestEagerLoadHasMany(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	testSchema(db).Create("eager_users2", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("name", 100)
	})

	testSchema(db).Create("eager_posts2", func(bp *ember.Blueprint) {
		bp.ID()
		bp.UnsignedBigInteger("eager_user_id")
		bp.String("title", 100)
	})

	db.Table("eager_users2").Insert(ctx, map[string]interface{}{"name": "Alice"})
	db.Table("eager_users2").Insert(ctx, map[string]interface{}{"name": "Bob"})
	db.Table("eager_posts2").Insert(ctx, map[string]interface{}{"eager_user_id": 1, "title": "Post1"})
	db.Table("eager_posts2").Insert(ctx, map[string]interface{}{"eager_user_id": 1, "title": "Post2"})
	db.Table("eager_posts2").Insert(ctx, map[string]interface{}{"eager_user_id": 2, "title": "Post3"})

	var users []EagerUser2
	err := db.Model().With("Posts").All(ctx, &users, func(b *ember.Builder) {
		b.OrderBy("id", "ASC")
	})
	if err != nil {
		t.Fatalf("EagerLoadHasMany: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if len(users[0].Posts) != 2 {
		t.Errorf("expected Alice to have 2 posts, got %d", len(users[0].Posts))
	}
	if len(users[1].Posts) != 1 {
		t.Errorf("expected Bob to have 1 post, got %d", len(users[1].Posts))
	}
}

// ─── Pagination tests ────────────────────────────────────────────────────────

func TestBuilderPaginate(t *testing.T) {
	db := openTestDB(t)
	createUsersTable(t, db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		db.Table("users").Insert(ctx, map[string]interface{}{
			"name":   fmt.Sprintf("User%d", i),
			"email":  fmt.Sprintf("user%d@test.com", i),
			"status": "active",
		})
	}

	paginator, err := db.Table("users").Paginate(ctx, &[]map[string]interface{}{}, 2, 3)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}

	if paginator.Total != 10 {
		t.Errorf("expected total 10, got %d", paginator.Total)
	}
	if paginator.CurrentPage != 2 {
		t.Errorf("expected page 2, got %d", paginator.CurrentPage)
	}
	if paginator.PerPage != 3 {
		t.Errorf("expected perPage 3, got %d", paginator.PerPage)
	}
	if paginator.LastPage != 4 {
		t.Errorf("expected lastPage 4, got %d", paginator.LastPage)
	}
	if paginator.From != 4 {
		t.Errorf("expected from 4, got %d", paginator.From)
	}
	if paginator.To != 6 {
		t.Errorf("expected to 6, got %d", paginator.To)
	}
}

// ─── Serialization tests ─────────────────────────────────────────────────────

type SerializeModel struct {
	ID    int64  `ember:"column:id;primaryKey;autoIncr"`
	Name  string `ember:"column:name"`
	Email string `ember:"column:email"`
}

func (m *SerializeModel) TableName() string { return "serial_users" }

func TestSerialization(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	testSchema(db).Create("serial_users", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("name", 100)
		bp.String("email", 255)
	})

	m := &SerializeModel{Name: "Test", Email: "test@test.com"}
	db.Model().Create(ctx, m)

	data, err := ember.ModelToMap(m)
	if err != nil {
		t.Fatalf("ModelToMap: %v", err)
	}
	if data["name"] != "Test" {
		t.Errorf("expected name 'Test', got %v", data["name"])
	}

	jsonBytes, err := ember.ToJSON(m)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Error("expected non-empty JSON")
	}
}

// ─── Event/Observer tests ────────────────────────────────────────────────────

type observedModel struct {
	ID   int64  `ember:"column:id;primaryKey;autoIncr"`
	Name string `ember:"column:name"`
}

func (m *observedModel) TableName() string { return "observed" }

type testObserver struct {
	ember.BaseObserver
	creatingCalled bool
	createdCalled  bool
}

func (o *testObserver) Creating(ctx context.Context, m interface{}) error {
	o.creatingCalled = true
	return nil
}

func (o *testObserver) Created(ctx context.Context, m interface{}) error {
	o.createdCalled = true
	return nil
}

func TestModelEvents(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	testSchema(db).Create("observed", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("name", 100)
	})

	observer := &testObserver{}
	ed := ember.NewEventDispatcher()
	ed.Observe(&observedModel{}, observer)
	db.SetEventDispatcher(ed)

	m := &observedModel{Name: "EventTest"}
	if err := db.Model().Create(ctx, m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !observer.creatingCalled {
		t.Error("expected Creating event to fire")
	}
	if !observer.createdCalled {
		t.Error("expected Created event to fire")
	}
}

// ─── Global Scope tests ──────────────────────────────────────────────────────

type scopedModel struct {
	ID     int64  `ember:"column:id;primaryKey;autoIncr"`
	Name   string `ember:"column:name"`
	Status string `ember:"column:status"`
}

func (m *scopedModel) TableName() string { return "scoped" }

func TestGlobalScopes(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	testSchema(db).Create("scoped", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("name", 100)
		bp.String("status", 50).Default("active")
	})

	db.Table("scoped").Insert(ctx, map[string]interface{}{"name": "A", "status": "active"})
	db.Table("scoped").Insert(ctx, map[string]interface{}{"name": "B", "status": "inactive"})

	db.ScopeRegistry().Add(&scopedModel{}, func(b *ember.Builder) *ember.Builder {
		return b.Where("status", "=", "active")
	})

	var items []scopedModel
	err := db.Model().All(ctx, &items)
	if err != nil {
		t.Fatalf("All with global scope: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item with global scope, got %d", len(items))
	}
}

// ─── Builder Macro tests ─────────────────────────────────────────────────────

func TestBuilderMacros(t *testing.T) {
	db := openTestDB(t)

	b := db.Table("users")
	b.Macro("whereActive", func(b *ember.Builder, args ...interface{}) interface{} {
		return b.Where("status", "=", "active")
	})
	result, err := b.Call("whereActive")
	if err != nil {
		t.Fatalf("Call whereActive: %v", err)
	}
	if _, ok := result.(*ember.Builder); !ok {
		t.Error("expected builder from macro call")
	}

	ember.AddBuilderMacro("whereInactive", func(b *ember.Builder, args ...interface{}) interface{} {
		return b.Where("status", "=", "inactive")
	})
	b2 := db.Table("users")
	result2, err := b2.Call("whereInactive")
	if err != nil {
		t.Fatalf("Call whereInactive: %v", err)
	}
	if _, ok := result2.(*ember.Builder); !ok {
		t.Error("expected builder from global macro call")
	}
}

// ─── Casting tests ───────────────────────────────────────────────────────────

type castModel struct {
	ID   int64  `ember:"column:id;primaryKey;autoIncr"`
	Meta string `ember:"column:meta;cast:json"`
}

func (m *castModel) TableName() string { return "cast_table" }

func TestCasting(t *testing.T) {
	db := openTestDB(t)

	testSchema(db).Create("cast_table", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("meta", 500)
	})

	schema, err := ember.ParseSchema(&castModel{})
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	metaField, ok := schema.FieldByCol["meta"]
	if !ok {
		t.Fatal("expected meta field")
	}
	if metaField.CastType != ember.CastJSON {
		t.Errorf("expected CastJSON, got %v", metaField.CastType)
	}
}

// ─── With() eager loading integration tests ────────────────────────────────────

type WithUser struct {
	ID    int64      `ember:"column:id;primaryKey;autoIncr"`
	Name  string     `ember:"column:name"`
	Posts []WithPost `ember:"relation:hasMany;foreignKey:with_user_id"`
}

func (m *WithUser) TableName() string { return "with_users" }

type WithPost struct {
	ID         int64     `ember:"column:id;primaryKey;autoIncr"`
	Title      string    `ember:"column:title"`
	WithUserID int64     `ember:"column:with_user_id"`
	User       *WithUser `ember:"relation:belongsTo;foreignKey:with_user_id"`
}

func (m *WithPost) TableName() string { return "with_posts" }

type EagerUser2 struct {
	ID    int64        `ember:"column:id;primaryKey;autoIncr"`
	Name  string       `ember:"column:name"`
	Posts []EagerPost2 `ember:"relation:hasMany;foreignKey:eager_user_id"`
}

func (m *EagerUser2) TableName() string { return "eager_users2" }

type EagerPost2 struct {
	ID          int64  `ember:"column:id;primaryKey;autoIncr"`
	Title       string `ember:"column:title"`
	EagerUserID int64  `ember:"column:eager_user_id"`
}

func (m *EagerPost2) TableName() string { return "eager_posts2" }

func TestWithHasMany(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	s := testSchema(db)
	s.DropIfExists("with_posts")
	s.DropIfExists("with_users")
	s.Create("with_users", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("name", 100)
	})
	s.Create("with_posts", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("title", 100)
		bp.UnsignedBigInteger("with_user_id")
	})

	db.Table("with_users").Insert(ctx, map[string]interface{}{"name": "Alice"})
	db.Table("with_users").Insert(ctx, map[string]interface{}{"name": "Bob"})
	db.Table("with_posts").Insert(ctx, map[string]interface{}{"title": "Post1", "with_user_id": 1})
	db.Table("with_posts").Insert(ctx, map[string]interface{}{"title": "Post2", "with_user_id": 1})
	db.Table("with_posts").Insert(ctx, map[string]interface{}{"title": "Post3", "with_user_id": 2})

	var users []WithUser
	err := db.Model().With("Posts").All(ctx, &users, func(b *ember.Builder) {
		b.OrderBy("id", "ASC")
	})
	if err != nil {
		t.Fatalf("With hasMany: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if len(users[0].Posts) != 2 {
		t.Errorf("expected Alice to have 2 posts, got %d", len(users[0].Posts))
	}
	if users[0].Posts[0].Title != "Post1" {
		t.Errorf("expected Post1, got %s", users[0].Posts[0].Title)
	}
	if users[0].Posts[1].Title != "Post2" {
		t.Errorf("expected Post2, got %s", users[0].Posts[1].Title)
	}
	if len(users[1].Posts) != 1 {
		t.Errorf("expected Bob to have 1 post, got %d", len(users[1].Posts))
	}
	if users[1].Posts[0].Title != "Post3" {
		t.Errorf("expected Post3, got %s", users[1].Posts[0].Title)
	}
}

func TestWithBelongsTo(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	s := testSchema(db)
	s.DropIfExists("with_posts")
	s.DropIfExists("with_users")
	s.Create("with_users", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("name", 100)
	})
	s.Create("with_posts", func(bp *ember.Blueprint) {
		bp.ID()
		bp.String("title", 100)
		bp.UnsignedBigInteger("with_user_id")
	})

	db.Table("with_users").Insert(ctx, map[string]interface{}{"name": "Alice"})
	db.Table("with_users").Insert(ctx, map[string]interface{}{"name": "Bob"})
	db.Table("with_posts").Insert(ctx, map[string]interface{}{"title": "Post1", "with_user_id": 1})
	db.Table("with_posts").Insert(ctx, map[string]interface{}{"title": "Post2", "with_user_id": 1})

	var posts []WithPost
	err := db.Model().With("User").All(ctx, &posts, func(b *ember.Builder) {
		b.OrderBy("id", "ASC")
	})
	if err != nil {
		t.Fatalf("With belongsTo: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].User == nil {
		t.Fatal("expected Post1 User to be loaded")
	}
	if posts[0].User.Name != "Alice" {
		t.Errorf("expected Alice, got %s", posts[0].User.Name)
	}
	if posts[1].User == nil {
		t.Fatal("expected Post2 User to be loaded")
	}
	if posts[1].User.Name != "Alice" {
		t.Errorf("expected Alice, got %s", posts[1].User.Name)
	}
}

// ─── FillFromMap tests ───────────────────────────────────────────────────────

type fillModel struct {
	ID    int64  `ember:"column:id;primaryKey;autoIncr"`
	Name  string `ember:"column:name"`
	Email string `ember:"column:email"`
}

func TestFillFromMap(t *testing.T) {
	m := &fillModel{}
	err := ember.FillFromMap(m, map[string]interface{}{
		"name":        "Filled",
		"email":       "filled@test.com",
		"nonexistent": "ignored",
	})
	if err != nil {
		t.Fatalf("FillFromMap: %v", err)
	}
	if m.Name != "Filled" {
		t.Errorf("expected Name 'Filled', got '%s'", m.Name)
	}
	if m.Email != "filled@test.com" {
		t.Errorf("expected Email 'filled@test.com', got '%s'", m.Email)
	}
}
