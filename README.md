<p align="center">
  <img src="docs/logo.png" alt="ember" width="400">
</p>

<!--
  Logo placeholder: [logo]: docs/logo.png
  Replace docs/logo.png with your project logo.
-->

<h1 align="center">ember</h1>
<p align="center"><strong>A Laravel Eloquent-inspired ORM for Go</strong></p>

<p align="center">
  <a href="https://github.com/NepMods/ember/releases"><img src="https://img.shields.io/badge/version-2.0.0-blue.svg" alt="Version"></a>
  <a href="https://go.dev/doc/devel/release"><img src="https://img.shields.io/github/go-mod/go-version/NepMods/ember" alt="Go Version"></a>
  <a href="https://github.com/NepMods/ember/actions"><img src="https://img.shields.io/github/actions/workflow/status/NepMods/ember/ci.yml?branch=main" alt="Build Status"></a>
  <a href="https://goreportcard.com/report/github.com/NepMods/ember"><img src="https://goreportcard.com/badge/github.com/NepMods/ember" alt="Go Report Card"></a>
  <a href="https://github.com/NepMods/ember/blob/main/LICENSE"><img src="https://img.shields.io/github/license/NepMods/ember" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/NepMods/ember"><img src="https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white" alt="go.dev"></a>
</p>

---

## Quick Start

```go
package main

import (
  "context"
  "fmt"
  "log"

  ember "github.com/NepMods/ember"
)

type User struct {
  ID         int64             `ember:"column:id;primaryKey;autoIncr"`
  Name       string            `ember:"column:name"`
  Email      string            `ember:"column:email"`
  Metadata   map[string]string `ember:"column:metadata;cast:json"`
  CreatedAt  string            `ember:"column:created_at"`
  UpdatedAt  string            `ember:"column:updated_at"`
}

func (User) TableName() string { return "users" }

func main() {
  ctx := context.Background()

  db, err := ember.Open(ember.Config{
    Driver: "sqlite3",
    Master: ":memory:",
  })
  if err != nil {
    log.Fatal(err)
  }
  defer db.Close()

  schema := ember.NewSchema(db)
  schema.Create("users", func(b *ember.Blueprint) {
    b.ID()
    b.String("name", 100)
    b.String("email", 255).Unique()
    b.Text("metadata").Nullable()
    b.Timestamps()
  })

  user := &User{Name: "Alice", Email: "alice@example.com"}
  if err := db.Model().Create(ctx, user); err != nil {
    log.Fatal(err)
  }
  fmt.Printf("Created user with ID %d\n", user.ID)

  var found User
  if err := db.Model().Find(ctx, &found, user.ID); err != nil {
    log.Fatal(err)
  }
  fmt.Printf("Found: %s <%s>\n", found.Name, found.Email)
}
```

---

## Features

| | Feature | Description |
|---|---|---|
| 🏗️ | **Fluent Query Builder** | Chainable SELECT/INSERT/UPDATE/DELETE/UPSERT builder with full WHERE, JOIN, GROUP BY, HAVING, ORDER BY, LIMIT/OFFSET support |
| 📦 | **Active Record Models** | `Create` / `Find` / `Save` / `Update` / `Delete` / `Restore` / `ForceDelete` with struct tag mapping |
| 🔗 | **Relationships** | HasMany, HasOne, BelongsTo, BelongsToMany — eager load with N+1 prevention |
| ⏩ | **Eager Loading** | `.With("Posts")`, nested `.With("Posts.Comments")`, ordering on relations |
| 🎯 | **Global Scopes** | Per-model automatic query filters (e.g. `activeOnlyScope`, `SoftDeleteScope`) |
| 🔌 | **Builder Macros** | Extend Builder with custom methods — per-instance or global |
| 🔔 | **Lifecycle Events** | 10 observer events: Creating/Created, Updating/Updated, Saving/Saved, Deleting/Deleted, Restoring/Restored |
| 🔄 | **Attribute Casting** | Tag-based cast to JSON, string, int, float, bool, date, datetime |
| 🗄️ | **Migration DSL** | `Schema.Create`/`Table`/`Drop` with `Blueprint` columns, indexes, foreign keys |
| 📄 | **Pagination** | `Paginate(page, perPage)` with total count, metadata, eager loading support |
| ⚡ | **Master/Replica** | Read-write splitting with round-robin replica load balancing and sticky-read context |
| 🧵 | **Transactions** | `Transaction()`, savepoints, nested transactions |
| 🔍 | **Raw SQL** | `Raw()` queries with `Scan`, `First`, `Pluck`, `Value`, `Exec` |
| 🏷️ | **Serialization** | `ModelToMap`, `ToJSON`, `CollectionToJSON` with configurable date/datetime formats |
| 🧩 | **Connection Manager** | Thread-safe multi-connection registry with named connections |
| 🗃️ | **Soft Deletes** | Auto `deleted_at` filtering, `Restore`, `ForceDelete`, `WithTrashed`/`OnlyTrashed` scopes |
| 📊 | **Aggregates** | `Count`, `Sum`, `Avg`, `Min`, `Max`, `Exists` |
| 🪣 | **Chunking** | `Chunk(size, fn)` for memory-efficient bulk processing |

---

## Installation

```bash
go get github.com/NepMods/ember
```

Then import:

```go
import ember "github.com/NepMods/ember"
```

Supported databases: **PostgreSQL**, **MySQL**, **SQLite3**.

---

## Core Concepts

### Package Structure

`ember` is a **flat, single-import package** — no sub-packages, no circular imports.

```go
import ember "github.com/NepMods/ember"
```

### Architecture Overview

```
DB ───┬── Table(name) → Builder (fluent SQL builder)
      ├── Model() → ModelDB (active record CRUD)
      ├── Raw(sql, ...) → RawQuery (raw SQL)
      ├── Begin(ctx, opts) → Tx (transactions)
      └── ScopeRegistry() → Add/Get scopes

Builder ──┬── Get/First/Find → struct/map scan
          ├── Insert/Update/Delete → exec
          ├── Paginate → Paginator
          ├── ToSQL → (string, args)
          └── Macro/Call → custom macros

ModelDB ──┬── Create/Find/First/All/Save/Update
          ├── Delete/ForceDelete/Restore
          ├── With("Relation") → eager loaded queries
          └── Paginate → Paginator
```

### Model Tags

```go
type Post struct {
  ID        int64     `ember:"column:id;primaryKey;autoIncr"`
  Title     string    `ember:"column:title"`
  Body      string    `ember:"column:body"`
  UserID    int64     `ember:"column:user_id"`
  Status    string    `ember:"column:status;default:draft"`
  Metadata  string    `ember:"column:metadata;cast:json"`
  CreatedAt time.Time `ember:"column:created_at"`
  UpdatedAt time.Time `ember:"column:updated_at"`
  DeletedAt *time.Time `ember:"column:deleted_at"`
}

// Custom table name (optional — falls back to snake_cased pluralized type name)
func (Post) TableName() string { return "posts" }
```

Tag directives: `column`, `primaryKey`, `autoIncr`, `nullable`, `unique`, `default`, `cast`.

---

## Complete CRUD

### Define a Model

```go
type Product struct {
  ID        int64     `ember:"column:id;primaryKey;autoIncr"`
  Name      string    `ember:"column:name"`
  Price     float64   `ember:"column:price"`
  Stock     int       `ember:"column:stock"`
  CreatedAt time.Time `ember:"column:created_at"`
  UpdatedAt time.Time `ember:"column:updated_at"`
}

func (Product) TableName() string { return "products" }
```

### Create

```go
p := &Product{Name: "Widget", Price: 9.99, Stock: 100}
err := db.Model().Create(ctx, p)
// p.ID is now set (auto-increment)
```

### Read

```go
// Find by primary key
var p Product
err := db.Model().Find(ctx, &p, 42)

// First with scoping
err := db.Model().First(ctx, &p, func(b *ember.Builder) {
  b.Where("name", "=", "Widget").Where("stock", ">", 0)
})

// All records
var products []Product
err := db.Model().All(ctx, &products)

// Where shorthand
err := db.Model().Where(ctx, &products, func(b *ember.Builder) {
  b.Where("price", ">", 5.00).OrderBy("name", "ASC")
})
```

### Update

```go
// Update a model by its primary key
p.Price = 7.99
err := db.Model().Update(ctx, p)           // all columns
err := db.Model().Update(ctx, p, "price")  // specific columns only

// Update via Builder
result, err := db.Table("products").
  Where("id", "=", 42).
  Update(ctx, map[string]interface{}{"price": 7.99, "stock": 50})

// Increment / Decrement
db.Table("products").Where("id", "=", 42).Increment(ctx, "stock", 5)
db.Table("products").Where("id", "=", 42).Decrement(ctx, "stock")
```

### Delete

```go
// Hard delete
err := db.Model().Delete(ctx, &p)

// Force delete (bypasses soft delete)
err := db.Model().ForceDelete(ctx, &p)

// Builder delete
affected, err := db.Table("products").Where("stock", "=", 0).Delete(ctx)
```

### Soft Delete

Add a `deleted_at` column to your model:

```go
type Post struct {
  // ...
  DeletedAt *time.Time `ember:"column:deleted_at"`
}
```

Soft deletes are automatically applied. Use the built-in scopes:

```go
// For models with `deleted_at` field, Delete() sets deleted_at instead of removing
err := db.Model().Delete(ctx, &post)

// Restore
err := db.Model().Restore(ctx, &post)

// Include soft-deleted records
err := db.Model().
  WithTrashed().
  All(ctx, &posts)

// Only soft-deleted records
err := db.Model().
  OnlyTrashed().
  All(ctx, &posts)
```

---

## Advanced Features

### Relationships

Define relations using struct tags:

```go
type User struct {
  ID    int64  `ember:"column:id;primaryKey;autoIncr"`
  Name  string `ember:"column:name"`
  Posts []Post `ember:"relation:hasMany;foreignKey:user_id;localKey:id"`
}

type Post struct {
  ID        int64  `ember:"column:id;primaryKey;autoIncr"`
  UserID    int64  `ember:"column:user_id"`
  Title     string `ember:"column:title"`
  Author    User   `ember:"relation:belongsTo;foreignKey:user_id;ownerKey:id"`
}

type Profile struct {
  ID     int64  `ember:"column:id;primaryKey;autoIncr"`
  UserID int64  `ember:"column:user_id"`
  User   User   `ember:"relation:hasOne;foreignKey:user_id;localKey:id"`
}

type Role struct {
  ID   int64  `ember:"column:id;primaryKey;autoIncr"`
  Name string `ember:"column:name"`
}

type UserRole struct {
  UserID int64 `ember:"column:user_id"`
  RoleID int64 `ember:"column:role_id"`
}

type UserWithRoles struct {
  ID    int64  `ember:"column:id;primaryKey;autoIncr"`
  Name  string `ember:"column:name"`
  Roles []Role `ember:"relation:belongsToMany;pivot:user_roles;pivotfk:user_id;pivotrk:role_id"`
}
```

Supported relation types: `hasMany`, `hasOne`, `belongsTo`, `belongsToMany`.

Tag keys: `foreignKey`, `localKey`, `ownerKey`, `pivot`, `pivotfk`, `pivotrk`.

### Eager Loading

```go
var users []User
err := db.Model().With("Posts").All(ctx, &users)
// All posts are fetched in a single WHERE user_id IN (...) query

// Nested loading
err := db.Model().With("Posts.Comments").All(ctx, &users)

// Multiple relations
err := db.Model().With("Posts", "Profile").All(ctx, &users)

// Order relation results
err := db.Model().With("Posts.OrderBy(created_at DESC)").All(ctx, &users)
```

The eager loader uses **N+1 prevention** via `WHERE fk IN (...)` — two queries instead of a complex JOIN.

### Scopes

```go
// Define a scope
func activeScope(b *ember.Builder) *ember.Builder {
  return b.Where("status", "=", "active")
}

// Use it on a query
err := db.Table("users").
  ApplyScopes(activeScope).
  All(ctx, &users)

// Or define a parameterized scope
func priceAbove(min float64) ember.Scope {
  return func(b *ember.Builder) *ember.Builder {
    return b.Where("price", ">", min)
  }
}

err := db.Table("products").
  ApplyScopes(priceAbove(10.0)).
  All(ctx, &products)
```

**Global Scopes** — automatically applied to every query for a model:

```go
db.ScopeRegistry().Add(&User{}, activeScope)

// Now every User query includes WHERE status = 'active' automatically
var users []User
db.Model().All(ctx, &users)
```

Built-in scopes: `ActiveScope`, `SoftDeleteScope`, `WithTrashedScope()`, `OnlyTrashedScope()`.

### Macros

Extend the Builder with custom chainable methods:

```go
// Per-instance macro
db.Table("users").
  Macro("whereActive", func(b *ember.Builder, args ...interface{}) interface{} {
    return b.Where("status", "=", "active").Where("deleted_at", "=", nil)
  }).
  Call("whereActive")

// Global macro
ember.AddBuilderMacro("whereActive", func(b *ember.Builder, args ...interface{}) interface{} {
  return b.Where("status", "=", "active")
})

db.Table("users").Call("whereActive").All(ctx, &users)
```

### Lifecycle Events

```go
// Define an observer
type UserObserver struct {
  ember.BaseObserver
}

func (o *UserObserver) Created(ctx context.Context, model interface{}) error {
  user := model.(*User)
  log.Printf("User %s created with ID %d", user.Name, user.ID)
  return nil
}

func (o *UserObserver) Saving(ctx context.Context, model interface{}) error {
  user := model.(*User)
  if user.Email == "" {
    return fmt.Errorf("email is required")
  }
  return nil
}

// Register the observer
dispatcher := ember.NewEventDispatcher()
dispatcher.Observe(&User{}, &UserObserver{})
db.SetEventDispatcher(dispatcher)

// Or use ObserverFuncs for inline handlers
dispatcher.ObserveAll(&ember.ObserverFuncs{
  CreatingFunc: func(ctx context.Context, model interface{}) error {
    log.Printf("About to create: %T", model)
    return nil
  },
})
```

10 lifecycle events: `Creating`, `Created`, `Updating`, `Updated`, `Saving`, `Saved`, `Deleting`, `Deleted`, `Restoring`, `Restored`.

### Attribute Casting

```go
type Article struct {
  ID        int64           `ember:"column:id;primaryKey;autoIncr"`
  Title     string          `ember:"column:title"`
  Tags      []string        `ember:"column:tags;cast:json"`
  Published bool            `ember:"column:published;cast:bool"`   // stored as 0/1
  Views     int             `ember:"column:views;cast:int"`       // stored as TEXT in SQLite
  Rating    float64         `ember:"column:rating;cast:float"`    // stored as TEXT
  Slug      string          `ember:"column:slug;cast:string"`     // stored as INT, read as string
  DateOnly  string          `ember:"column:date_only;cast:date"`
  CreatedAt time.Time       `ember:"column:created_at;cast:datetime"`
}
```

Casts are applied automatically on read and write:
- `json` — serialized/deserialized via `encoding/json`
- `string`, `int`, `float`, `bool` — converted between types safely
- `date` / `datetime` — `time.Time` ↔ formatted string (SQLite compatibility)

### Migrations

```go
// Define a migration
type CreateUsersTable struct{}

func (m CreateUsersTable) Version() string { return "2024_01_01_000001" }
func (m CreateUsersTable) Up(schema *ember.Schema) error {
  return schema.Create("users", func(b *ember.Blueprint) {
    b.ID()
    b.String("name", 100).Nullable()
    b.String("email", 255).Unique()
    b.Timestamps()
    b.SoftDeletes()
  })
}
func (m CreateUsersTable) Down(schema *ember.Schema) error {
  return schema.Drop("users")
}

// Run migrations
migrator := ember.NewMigrator(db, CreateUsersTable{})
err := migrator.Run(ctx)

// Rollback the latest migration
err := migrator.Rollback(ctx, 1)

// Fresh (drop all tables and re-run)
err := migrator.Fresh(ctx)

// Check status
statuses, _ := migrator.Status(ctx)
for _, s := range statuses {
  fmt.Printf("%s: ran=%v\n", s.Version, s.Ran)
}
```

### Blueprint DSL

```go
schema.Create("products", func(b *ember.Blueprint) {
  b.ID()
  b.String("sku", 50).Unique()
  b.String("name", 200)
  b.Text("description").Nullable()
  b.Integer("quantity").Default(0).Unsigned()
  b.Decimal("price", 10, 2)
  b.Boolean("is_active").Default(true)
  b.Date("available_from").Nullable()
  b.DateTime("last_restocked").Nullable()
  b.JSON("attributes").Nullable()
  b.Enum("status", []string{"draft", "published", "archived"}).Default("draft")
  b.Timestamps()
  b.SoftDeletes()

  b.Index("name", "sku")
  b.UniqueIndex("sku")
  b.ForeignKey("user_id").References("id").On("users").CascadeOnDelete()
})
```

### Pagination

```go
var users []User
paginator, err := db.Model().Paginate(ctx, &users, 2, 15) // page 2, 15 per page

fmt.Printf("Page %d of %d (total: %d)\n", paginator.CurrentPage, paginator.LastPage, paginator.Total)
fmt.Printf("Showing items %d–%d\n", paginator.From, paginator.To)

// Builder pagination
var products []Product
paginator, err := db.Table("products").
  Where("price", ">", 10.0).
  OrderBy("name", "ASC").
  Paginate(ctx, &products, 1, 20)

// Pagination with eager loading
var posts []Post
paginator, err := db.Model().With("Comments").Paginate(ctx, &posts, 1, 10)
```

The `Paginator` struct:

```go
type Paginator struct {
  CurrentPage int         // Current page number
  LastPage    int         // Total number of pages
  PerPage     int         // Items per page
  Total       int64       // Total matching records
  From        int         // Starting item number
  To          int         // Ending item number
  Items       interface{} // Pointer to the result slice
}
```

---

## Configuration

### Master/Replica Setup

```go
db, err := ember.Open(ember.Config{
  Driver: "postgres",
  Master: "postgres://user:pass@master:5432/db?sslmode=require",
  Replicas: []string{
    "postgres://user:pass@replica1:5432/db?sslmode=require",
    "postgres://user:pass@replica2:5432/db?sslmode=require",
  },
  Pool: ember.PoolConfig{
    MaxOpenConns:    25,
    MaxIdleConns:    10,
    ConnMaxLifetime: 5 * time.Minute,
    ConnMaxIdleTime: 30 * time.Second,
  },
})
```

- **Writes** always go to master (INSERT, UPDATE, DELETE, DDL).
- **Reads** use round-robin across replicas.
- **Sticky reads**: Force the master connection after a write:

```go
ctx := ember.WithStickyMaster(ctx)
result := db.Table("users").Where("id", "=", 1).Value(ctx, "name", &name) // reads from master
```

### Connection Manager

```go
mgr := ember.NewConnectionManager()
mgr.Add("primary", ember.Config{
  Driver: "postgres",
  Master: "postgres://user:pass@primary:5432/db",
})
mgr.Add("analytics", ember.Config{
  Driver: "mysql",
  Master: "user:pass@tcp(analytics:3306)/db",
})

primaryDB := mgr.Connection("primary")
analyticsDB := mgr.Connection("analytics")

// Set default
mgr.SetDefault("primary")
defaultDB := mgr.DB()

// List connections
for _, name := range mgr.Names() {
  fmt.Println(name)
}

// Cleanup
mgr.CloseAll()
```

### Transactions

```go
err := db.Transaction(ctx, func(tx *ember.Tx) error {
  tx.Table("accounts").Where("id", "=", 1).Decrement(ctx, "balance", 100)
  tx.Table("accounts").Where("id", "=", 2).Increment(ctx, "balance", 100)

  // With savepoints
  tx.Savepoint(ctx, "after_debit")
  tx.RollbackTo(ctx, "after_debit")
  tx.ReleaseSavepoint(ctx, "after_debit")

  return nil
})

// Manual transaction
tx, _ := db.Begin(ctx, nil)
tx.Table("orders").Insert(ctx, map[string]interface{}{"total": 49.99})
tx.Commit()
```

### Raw SQL

```go
// Single row scan
var user User
err := db.Raw("SELECT * FROM users WHERE id = ?", 1).First(ctx, &user)

// Multi-row scan
var users []User
err := db.Raw("SELECT * FROM users WHERE active = ?", true).Scan(ctx, &users)

// Pluck a column
var names []string
err := db.Raw("SELECT name FROM users").Pluck(ctx, &names)

// Single value
var count int64
err := db.Raw("SELECT COUNT(*) FROM users").Value(ctx, &count)

// Exec
result, err := db.Raw("UPDATE users SET status = ? WHERE id = ?", "inactive", 5).Exec(ctx)
affected, _ := result.RowsAffected()
```

### Serialization

```go
// Model to map
m, _ := ember.ModelToMap(user)
fmt.Println(m["name"])

// Model to JSON
data, _ := ember.ToJSON(user)
os.Stdout.Write(data)

// Custom date formats
cfg := &ember.SerializationConfig{
  DateFormat:        "2006-01-02",
  DateTimeFormat:    time.RFC1123,
  IncludeTimestamps: true,
  IncludeSoftDelete: false,
}
m, _ := ember.ModelToMapWithConfig(user, cfg)

// Collections
users := []User{...}
maps, _ := ember.CollectionToMap(users)
json, _ := ember.CollectionToJSON(users)
```

### Query Builder Deep Dive

```go
db.Table("orders").
  Select("orders.*", "users.name AS user_name").
  Join("users", "orders.user_id", "=", "users.id").
  LeftJoin("coupons", "orders.coupon_id", "=", "coupons.id").
  Where("orders.status", "=", "completed").
  OrWhere("orders.priority", ">", 5).
  WhereIn("orders.region", "US", "EU", "APAC").
  WhereBetween("orders.total", 50.0, 500.0).
  WhereNull("orders.deleted_at").
  WhereGroup(func(q *ember.Builder) {
    q.Where("orders.express", "=", true).
      OrWhere("orders.shipped", "=", true)
  }).
  GroupBy("orders.region").
  Having("COUNT(*)", ">", 10).
  OrderBy("orders.created_at", "DESC").
  Limit(100).
  Offset(20).
  LockForUpdate().
  ToSQL() // (string, []interface{})
```

Available WHERE methods: `Where`, `OrWhere`, `WhereIn`, `WhereNotIn`, `WhereNull`, `WhereNotNull`, `WhereBetween`, `WhereGroup`, `OrWhereGroup`, `WhereColumn`, `WhereRaw`, `OrWhereRaw`.

Aggregates: `Count`, `Sum`, `Avg`, `Min`, `Max`, `Exists`, `DoesntExist`.

Utility: `Chunk(size, fn)`, `ForPage(page, perPage)`, `Pluck`, `Value`, `ToSQL`.

---

## Testing

All tests use SQLite in-memory — zero setup required.

```bash
go test -v -count=1 ./...
```

Tests run in approximately 1–3 seconds with no external database daemon needed.

## Contributing

1. Fork the repository.
2. Create a feature branch (`git checkout -b feat/amazing`).
3. Write tests for your changes.
4. Ensure all tests pass (`go test -v -count=1 ./...`).
5. Open a Pull Request.

See the [full contributing guide](https://github.com/NepMods/ember/blob/main/CONTRIBUTING.md) for details.

---

## License

**ember** is open-source software released under the [MIT License](https://github.com/NepMods/ember/blob/main/LICENSE).

---

## Documentation

- [Full API Reference](https://pkg.go.dev/github.com/NepMods/ember)
- [GitHub Repository](https://github.com/NepMods/ember)
- [Issue Tracker](https://github.com/NepMods/ember/issues)
