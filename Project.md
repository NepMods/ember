# ember — Laravel-Inspired ORM for Go

**Version 2.0.0** — Full-featured ORM, query builder, migration DSL, and schema system for Go.

---

## Overview

`ember` is a flat-package Go library that provides a Laravel Eloquent-inspired ORM experience: a fluent query builder,
an active-record-style model layer with hooks and events, a cross-dialect migration DSL, eager loading, pagination,
serialization, attribute casting, global scopes, builder macros, and a connection manager supporting
master/replica read-write splitting — all in a single importable package (`import ember "github.com/NepMods/ember"`).

Supported databases: PostgreSQL, MySQL, SQLite3.

---

## Package Architecture

### File Listing

| File | Purpose |
|---|---|
| `orm.go` | Package doc, version constant (`2.0.0`), quick-start example |
| `db.go` | Core `DB` struct — master + replica `*sql.DB` handles, `Open()`, routing, transactions, `Table()`/`Raw()` entry points |
| `builder.go` | Fluent SQL builder — SELECT/INSERT/UPDATE/DELETE/UPSERT, JOIN, WHERE, GROUP BY, HAVING, ORDER BY, LIMIT/OFFSET, aggregates, `ToSQL()`, chunking |
| `model.go` | `ModelDB` — active-record CRUD (Create/Find/Save/Update/Delete/Restore/ForceDelete), hook interfaces (`BeforeCreate`, `AfterCreate`, etc.), `FillFromMap`, scan helpers |
| `schema.go` | Struct introspection — `ParseSchema()`, `FieldSchema`, `ModelSchema`, tag parsing (`ember:"column:...;primaryKey"`), auto-table naming, pluralization, relation tag parsing, schema cache |
| `dialect.go` | `Dialect` interface + implementations for PostgreSQL, MySQL, SQLite3 — quoting, placeholders, `RETURNING`, `UPSERT` clauses |
| `blueprint.go` | Migration column/table DSL — `ColumnDef`, `IndexDef`, `ForeignKeyDef`, `Blueprint`, `ToCreateSQL()`, `ToAlterSQL()`, `ToIndexSQL()`, type resolution per dialect |
| `migration.go` | Migration framework — `Migration` interface, `Schema` (Create/Drop/Table/Raw/HasTable/HasColumn), `Migrator` (Run/Rollback/Fresh/Status) |
| `raw.go` | `RawQuery` — execute raw SQL with `Scan()`, `First()`, `Exec()`, `Pluck()`, `Value()` |
| `tx.go` | `Tx` — transaction wrapper with `Commit()`, `Rollback()`, `Savepoint()`, `RollbackTo()`, `ReleaseSavepoint()`, `Table()`, `Raw()` |
| `errors.go` | Sentinel errors (`ErrNotFound`, `ErrNoMaster`, `ErrInvalidModel`, `ErrMissingPrimaryKey`, `ErrDuplicateConnection`), `QueryError` |
| `config.go` | `Config` and `PoolConfig` structs — driver, DSNs, pool settings |
| `manager.go` | `ConnectionManager` — named connection registry with `Add()`, `AddDB()`, `Connection()`, `Remove()`, `CloseAll()`, thread-safe |
| `relations.go` | Eager loading engine — `HasMany`/`HasOne`/`BelongsTo`/`BelongsToMany` load methods, `EagerLoadSlice`/`EagerLoadSingle`, N+1 prevention via `IN (...)` queries, `safeKey()` string conversion |
| `pagination.go` | `Paginator` — `Builder.Paginate()` and `ModelDB.Paginate()` with total count + LIMIT/OFFSET |
| `serialization.go` | `ModelToMap`, `ToJSON`, `CollectionToMap`, `CollectionToJSON`, `SelectColumns` — configurable date/datetime format |
| `events.go` | Observer-pattern event system — `EventDispatcher`, `Observer` interface, `BaseObserver`, `ObserverFuncs`, 10 lifecycle events |
| `macros.go` | Builder macros — per-instance (`Macro()`) and global (`AddBuilderMacro()`), `Call()` dispatch |
| `scopes.go` | `ScopeRegistry` — per-model-type global scope storage, `Scope` type alias |
| `casting.go` | Attribute casting — `CastJSON`, `CastDate`, `CastDatetime`, `CastString`, `CastInt`, `CastFloat`, `CastBool`; applied on read/write |
| `orm_test.go` | Integration tests — SQLite in-memory, all builder operations, model CRUD, relationships, pagination, serialization, events, scopes, macros, casting |

### Package Dependency Graph

Because `ember` is a single flat Go package, **every file shares all types and functions without import edges**. The logical dependency graph is conceptual rather than explicit:

```
errors.go ────────────────── (sentinel errors used everywhere)
config.go ────────────────── (Config struct used by db.go)
dialect.go ───────────────── (used by db.go, builder.go, blueprint.go, migration.go, schema.go)

db.go ────────────────────── (DB struct — root object)
  ├── builder.go ─────────── (Builder — returned by DB.Table())
  │     ├── raw.go ───────── (RawQuery — returned by DB.Raw())
  │     └── pagination.go ── (Paginator — uses Builder.Count())
  ├── model.go ───────────── (ModelDB — returned by DB.Model())
  │     ├── schema.go ────── (ParseSchema — struct introspection)
  │     ├── relations.go ─── (eager loading — used by ModelDB.With())
  │     ├── events.go ────── (Observer lifecycle events)
  │     ├── scopes.go ────── (ScopeRegistry — global scopes)
  │     └── casting.go ───── (attribute cast on read/write)
  ├── tx.go ──────────────── (Tx — returned by DB.Begin())
  ├── manager.go ─────────── (ConnectionManager — registry of *DB)
  └── migration.go ───────── (Schema + Migrator — DDL operations)
        └── blueprint.go ─── (Column/Index/FK definitions)

macros.go ────────────────── (BuilderMacro — attached to Builder)
serialization.go ─────────── (ModelToMap/ToJSON — standalone functions)

orm.go ───────────────────── (package doc + Version constant)
orm_test.go ──────────────── (tests, imports ember as external package)
```

---

## Key Design Decisions

### 1. Flat Package (No Sub-Packages)

`ember` lives entirely in a single Go package. This avoids circular import problems (common in layered ORMs),
simplifies the user's import path to one line, and allows internal types (`whereClause`, `joinClause`, etc.)
to remain unexported while still being shared across all subsystems. The tradeoff is a slightly larger API surface
but significantly easier maintenance and zero import-cycle headaches.

### 2. Master/Replica Pattern Instead of Connection Pooling Alone

Each `Config` defines one master DSN and N replica DSNs. Writes always go to master; SELECTs use round-robin
load balancing across replicas. A sticky-read context key (`WithStickyMaster`) forces reads to master after a write.
This is a deliberate architectural choice over simple connection pooling — it mirrors production deployment patterns
(one writer, many read replicas) and avoids the complexity of middleware-level read/write splitting.

### 3. `interface{}` Map Keys Replaced with `safeKey()` String Conversion

Eager loading groups child records by foreign key values. Using raw `interface{}` values as map keys is fragile
(incomparable types panic at runtime). Instead, `safeKey()` converts any value to a string (`string`, `[]byte` → direct;
everything else → `fmt.Sprintf("%v")`), providing a stable, comparable key for grouping maps.

### 4. Interface-Based Dialect System

The `Dialect` interface abstracts quoting (`"` vs `` ` ``), placeholders (`$N` vs `?`), `RETURNING` support,
and `UPSERT` syntax. New databases are added by implementing this interface and registering in `GetDialect()`.
This avoids dialect-switching conditionals throughout the codebase and supports user-defined dialects.

### 5. Eager Loading Uses N+1 Prevention via `IN (...)` Queries Instead of Joins

When loading `HasMany`/`BelongsTo`/`BelongsToMany`, `ember` first queries the parent records, collects their
keys, then executes a single `WHERE fk IN (...)` for all children. Results are grouped in memory by foreign key
and assigned to parent struct fields via reflection. This produces two simple queries instead of one complex
join, is easier to debug, works correctly with LIMIT, and avoids row duplication from `JOIN` fan-out.

### 6. Pagination: Separate `COUNT` + `SELECT` Instead of `SQL_CALC_FOUND_ROWS`

`Paginate()` runs `COUNT(*)` first (with the same WHERE clauses), then a `SELECT ... LIMIT/OFFSET`.
This approach is database-agnostic, avoids MySQL-specific `SQL_CALC_FOUND_ROWS`/`FOUND_ROWS()` which is
deprecated in MySQL 8.0.17+, and works identically across all three supported dialects.

### 7. Casting Is Tag-Based (`ember:"cast:json"`)

Attribute casts are declared on the struct field tag rather than via a method (e.g., Laravel's `$casts` property).
This keeps the type system as the source of truth, avoids runtime method lookup, and makes casts visible at a
glance. Supported: `json`, `string`, `int`, `float`, `bool`, `date`, `datetime`.

### 8. Events Use Observer Pattern Instead of Go Channels

The `Observer` interface + `EventDispatcher` with `Fire()` broadcasting follows the classical Observer pattern.
Go channels would require each subscriber to have a dedicated goroutine and introduce back-pressure complexity.
The Observer approach is synchronous (the model operation waits for all observers), deterministic, and simpler
to debug. `BaseObserver` provides no-op defaults so users only override the methods they need.

### 9. Macros Use `interface{}` Return Type

`BuilderMacro` is `func(*Builder, ...interface{}) interface{}`. The `interface{}` return trades compile-time
type safety for maximum flexibility — a macro may return a modified `*Builder` (for chaining), a computed value,
or an error. This matches the dynamic nature of macros and avoids forcing every macro into a `*Builder` return.

### 10. Global Scopes Are Registry-Based Rather Than Method-Based

Instead of a `boot()` method on the model, scopes are registered imperatively:
`db.ScopeRegistry().Add(&MyModel{}, activeOnlyScope)`. This decouples scope definition from model definition,
allows runtime scope registration, and avoids requiring users to implement a specific interface on every model.

### 11. `Builder` Is NOT Safe for Concurrent Use

The `Builder` struct holds mutable slices (`wheres`, `joins`, `orderBys`, etc.) and internal state
(`placeholderIdx`). It is designed for single-goroutine fluent chaining. Making it thread-safe would require
locking every mutator method, which would harm performance for the common single-threaded case. Users who need
concurrent query building should clone the builder (shallow copy) for each goroutine.

### 12. Schema Caching Uses `sync.RWMutex`

`ParseSchema()` caches parsed schemas by `reflect.Type` in a global map guarded by `sync.RWMutex`.
Reads are the critical path (every model operation calls `ParseSchema`), so the read lock (`RLock`) is used
for the fast path. The write lock is only acquired when a new type is parsed for the first time. This matches
the read-heavy workload of schema introspection.

### 13. SQLite `TEXT(N)` Was Removed

SQLite does not support `VARCHAR(N)` with length limits — it treats `VARCHAR(N)` as `TEXT` and ignores the N.
The original code generated `TEXT(255)` which is invalid SQL. The fix maps all `varchar`, `text`, `json`, `jsonb`,
and `enum` types to plain `TEXT` in the SQLite dialect resolver.

### 14. `Default()` Is Type-Aware

`ColumnDef.Default(v interface{})` inspects the Go type of the value: strings are auto-quoted (with SQL injection
prevention via single-quote escaping), bools map to `1`/`0`, nil maps to `NULL`, and all other types use
`fmt.Sprintf("%v")`. This prevents callers from having to remember to quote string defaults manually.

### 15. Savepoint Names Are Validated with Regex

`Tx.Savepoint()`, `RollbackTo()`, and `ReleaseSavepoint()` validate the savepoint name against
`^[a-zA-Z_][a-zA-Z0-9_]*$` before executing the SQL. This prevents SQL injection through savepoint names
(which are interpolated directly into SQL strings rather than bound as parameters).

### 16. Scan Uses Temp Strings for `time.Time` Fields (SQLite Compatibility)

SQLite stores timestamps as TEXT. When scanning into a `time.Time` struct field, `database/sql` cannot
directly convert a TEXT column. The `scanPointersByCol()` / `scanPointersForRow()` functions detect
date/datetime-cast fields and allocate temporary `*string` pointers, scan into those, then parse them
back into `time.Time` using `scanTimeStrings()`. This keeps SQLite working without requiring users to
change their model types.

### 17. `scanPointersByCol` Approach Chosen Over `sql.Row` for `First()`

`ModelDB.scanFirst()` and `scanRow()` in the builder both use a two-step approach: execute a query,
get `*sql.Rows`, read column metadata, then build scan pointers by column name using `scanPointersByCol()`.
This is preferred over `*sql.Row.Scan()` because it gives access to column names (needed for mapping
DB columns to struct fields) and handles the variable-column-order problem that arises with `SELECT *`.

### 18. Version Is 2.0.0

This version represents a major refactor from the initial prototype. Key breaking changes include fixing `With()`
(was a no-op), making `LoadBelongsTo` respect `OwnerKey`, type-aware `Default()`, removing the dead `Builder.bindings`
field, replacing the custom `sortStrings` with `sort.Strings`, and fixing SQLite `TEXT(N)`.

---

## API Reference Summary

### Core Types

| Type | Key Methods |
|---|---|
| `DB` | `Open(Config)`, `Close()`, `Ping()`, `Table(string)`, `Raw(string, ...)`, `Model()`, `Begin(ctx, *TxOptions)`, `Transaction(ctx, fn)`, `ScopeRegistry()`, `SetEventDispatcher()` |
| `Builder` | `Select(...)`, `Where(col, op, val)`, `OrWhere()`, `WhereIn()`, `WhereNull()`, `WhereBetween()`, `WhereGroup()`, `Join()`, `LeftJoin()`, `OrderBy()`, `GroupBy()`, `Having()`, `Limit()`, `Offset()`, `LockForUpdate()`, `ToSQL()`, `Get()`, `First()`, `Find()`, `Pluck()`, `Value()`, `Count()`, `Sum()`, `Avg()`, `Min()`, `Max()`, `Exists()`, `Chunk()`, `Insert()`, `InsertBatch()`, `InsertGetId()`, `Upsert()`, `Update()`, `Increment()`, `Decrement()`, `Delete()`, `Truncate()`, `Paginate()`, `Macro()`, `Call()` |
| `ModelDB` | `Create()`, `Find()`, `First()`, `All()`, `Save()`, `Update()`, `Delete()`, `ForceDelete()`, `Restore()`, `Where()`, `Paginate()`, `With()`, `LoadHasMany()`, `LoadHasOne()`, `LoadBelongsTo()`, `LoadBelongsToMany()` |
| `RawQuery` | `Scan()`, `First()`, `Get()`, `Exec()`, `ExecAffected()`, `Pluck()`, `Value()`, `ToSQL()` |
| `Tx` | `Commit()`, `Rollback()`, `Savepoint()`, `RollbackTo()`, `ReleaseSavepoint()`, `Table()`, `Raw()`, `Model()` |
| `Schema` | `Create()`, `Table()`, `Drop()`, `DropIfExists()`, `Raw()`, `HasTable()`, `HasColumn()` |
| `Migrator` | `NewMigrator()`, `Run()`, `Rollback()`, `Fresh()`, `Status()`, `Pending()`, `Add()`, `SetTable()` |
| `Blueprint` | `ID()`, `String()`, `Integer()`, `BigInteger()`, `Text()`, `Boolean()`, `Date()`, `DateTime()`, `Timestamp()`, `JSON()`, `JSONB()`, `Enum()`, `Decimal()`, `Float()`, `Timestamps()`, `SoftDeletes()`, `Foreign()`, `Index()`, `UniqueIndex()`, `Primary()`, `Nullable()`, `Default()`, `Unique()`, `Unsigned()` |
| `ConnectionManager` | `NewConnectionManager()`, `Add()`, `AddDB()`, `DB()`, `Connection()`, `ConnectionSafe()`, `Remove()`, `CloseAll()`, `Names()`, `SetDefault()` |
| `Paginator` | `CurrentPage`, `LastPage`, `PerPage`, `Total`, `From`, `To`, `Items` |
| `EventDispatcher` | `NewEventDispatcher()`, `Observe()`, `ObserveAll()`, `Fire()` |
| `ScopeRegistry` | `NewScopeRegistry()`, `Add()`, `Get()` |

### Key Interfaces

| Interface | Methods |
|---|---|
| `Dialect` | `Name()`, `QuoteIdentifier()`, `Placeholder()`, `SupportsReturning()`, `UpsertClause()` |
| `Migration` | `Version()`, `Up(schema)`, `Down(schema)` |
| `Observer` | `Creating`, `Created`, `Updating`, `Updated`, `Saving`, `Saved`, `Deleting`, `Deleted`, `Restoring`, `Restored` |
| `BeforeCreator`, `AfterCreator`, `BeforeSaver`, `AfterSaver`, `BeforeUpdater`, `AfterUpdater`, `BeforeDeleter`, `AfterDeleter` | Single hook method each |
| `Tabler` | `TableName() string` |
| `GlobalScoper` | `GlobalScopes() []Scope` |

### Standalone Functions

| Function | Purpose |
|---|---|
| `ParseSchema(model)` | Introspect a struct into `*ModelSchema` |
| `FillFromMap(model, data)` | Populate struct from column→value map |
| `ModelToMap(model)` / `ModelToMapWithConfig` | Serialize model to `map[string]interface{}` |
| `ToJSON(model)` / `ToJSONWithConfig` | Serialize model to JSON bytes |
| `CollectionToMap(models)` / `CollectionToJSON` | Serialize slice of models |
| `SelectColumns(schema, dialect)` | Produce quoted column list |
| `ApplyScopes(b, scopes...)` | Apply scope functions to builder |
| `AddBuilderMacro(name, fn)` | Register a global builder macro |
| `GetDialect(driver)` | Factory for dialect by driver name |
| `NewSchema(db)`, `NewMigrator(db, ...)` | Schema and migrator constructors |
| `WithStickyMaster(ctx)` | Context wrapper forcing master reads |

---

## Testing Strategy

### Approach

- **SQLite in-memory** (`:memory:`) for all integration tests — zero setup, fast, deterministic.
- **Dialect comparison** via `ToSQL()` — SQL output is checked for correct structure, argument count, and placeholder style.
- **All three dialects** are tested in blueprint schema generation (`TestBlueprint_CreateSQL`).
- Tests use `ember_test` package (external test package) to ensure the public API is exercised as a real consumer would.

### Coverage Areas

| Area | Test Functions |
|---|---|
| Dialect placeholders/quoting | `TestDialects` |
| Builder SQL compilation | `TestBuilderToSQL_Select`, `WhereIn`, `WhereBetween`, `WhereNull`, `WhereGroup`, `Join`, `GroupBy` |
| INSERT / batch INSERT | `TestBuilderToSQL_Insert`, `TestBuilderToSQL_InsertBatch` |
| UPDATE / DELETE | `TestBuilderToSQL_Update`, `TestBuilderToSQL_Delete` |
| Get maps | `TestBuilderGet_Maps` |
| Exists / Count | `TestBuilderExists` |
| Increment | `TestBuilderIncrement` |
| Chunk | `TestBuilderChunk` |
| Transactions | `TestTransaction_Commit`, `TestTransaction_Rollback` |
| Raw queries | `TestRawQuery` |
| Schema parsing | `TestParseSchema` |
| Blueprint SQL generation | `TestBlueprint_CreateSQL`, `TestBlueprint_ForeignKey` |
| Migration run/rollback | `TestMigrator_RunAndRollback` |
| Model CRUD | `TestModel_CreateAndFind`, `TestModel_Update`, `TestModel_SoftDelete` |
| Connection manager | `TestConnectionManager` |
| Scopes | `TestScopes`, `TestGlobalScopes` |
| Relation parsing | `TestParseRelations` |
| Eager loading | `TestEagerLoadHasMany`, `TestWithHasMany`, `TestWithBelongsTo` |
| Pagination | `TestBuilderPaginate` |
| Serialization | `TestSerialization` |
| Events | `TestModelEvents` |
| Macros | `TestBuilderMacros` |
| Casting | `TestCasting` |
| FillFromMap | `TestFillFromMap` |

### Running Tests

```bash
cd /home/arjun/ember
go test -v -count=1 ./...
```

Tests run in approximately 1–3 seconds with only SQLite (no Postgres/MySQL daemon required).

---

## Security Considerations

### SQL Injection Prevention

- **All user values** are passed as bind parameters (`?` or `$N`), never interpolated into SQL strings.
- **Column/table names** are quoted via `Dialect.QuoteIdentifier()` — they come from struct tags or trusted code, not user input. The builder does not allow dynamic column names from untrusted sources without explicit quoting.
- **Operator whitelist**: `Builder.Where()` validates the operator against `validOperators` map (`=`, `!=`, `<>`, `<`, `>`, `LIKE`, `REGEXP`, etc.). Invalid operators silently default to `=`.
- **Raw SQL methods** (`WhereRaw`, `HavingRaw`, `OrderByRaw`, `RawQuery`) accept raw SQL strings — the caller is responsible for safe SQL. These are documented as such.

### Input Validation

- **Savepoint names** are validated with `^[a-zA-Z_][a-zA-Z0-9_]*$` before being interpolated into `SAVEPOINT name` SQL.
- **`Default()` quoting**: String values are escaped (`'` → `''`) and wrapped in single quotes. Bool, nil, and numeric types are handled safely.
- **Batch INSERT validation**: All rows must have the same column set; mismatched columns return an error before executing SQL.

### Thread Safety Guarantees

| Component | Safe? | Notes |
|---|---|---|
| `DB` (master/replica handles) | Yes | `*sql.DB` is safe; replica index uses `atomic.AddUint64` |
| `Builder` | **No** | Single-goroutine only |
| `ConnectionManager` | Yes | `sync.RWMutex` on all operations |
| `EventDispatcher` | Yes | `sync.RWMutex` on observe/fire |
| `ScopeRegistry` | Yes | `sync.RWMutex` on add/get |
| Schema cache | Yes | `sync.RWMutex` — read-locked on fast path |
| `ModelDB` | No (inherits Builder) | Depends on Builder and reflect, not safe |
| Global macros | Yes | `sync.RWMutex` on add/call |

---

## Known Limitations

- **Pluralization is English-only** — the `pluralize()` function handles common irregulars and standard English rules but does not support other languages.
- **No model factory support** — Unlike Laravel's `Factory` for test data generation, `ember` has no equivalent yet.
- **No polymorphic relationship test coverage** — `MorphOne`, `MorphMany`, `MorphTo` relation types are parsed but not exercised in tests or eager loading.
- **No Postgres/MySQL integration tests** — All tests use SQLite in-memory. Dialect-specific features (`SERIAL`, `RETURNING`, `AUTO_INCREMENT`, etc.) are tested via `ToSQL()` output only.
- **Concurrent builder use is not safe** — As documented above, sharing a `Builder` across goroutines will cause data races.
- **Embedded/anonymous struct fields are skipped** — `ParseSchema` skips anonymous (embedded) fields. Flattening is not supported.
- **No migration file scanning** — Migrations must be registered explicitly with `NewMigrator(db, migrations...)`; there is no filesystem auto-discovery.

---

## Migration Guide (v1 → v2)

### Breaking Changes

1. **`With()` now actually works** — In v1, `With("Relation")` was parsed but eager loading was never executed. v2 implements full eager loading for `HasMany`, `HasOne`, `BelongsTo`, and `BelongsToMany`.

2. **`LoadBelongsTo` now respects `OwnerKey`** — v1 ignored the `OwnerKey` parameter and always used `"id"`. v2 resolves `OwnerKey` from the relation definition or falls back to the parent's primary key.

3. **`Default()` is type-aware** — v1 always treated the default value as a raw SQL string. v2 inspects the Go type:
   - `string` → auto-quoted with single-quote escaping
   - `bool` → `1` / `0`
   - `nil` → `NULL`
   - other → `fmt.Sprintf("%v")`

4. **`Builder.bindings` field removed** — v1 had a `bindings []interface{}` field on `Builder` that was never populated (dead code). Removed in v2.

5. **`sortStrings` replaced with `sort.Strings`** — v1 had a custom `sortStrings` helper. v2 uses the standard library `sort.Strings()`.

6. **SQLite `TEXT(N)` → `TEXT`** — v1 generated `TEXT(255)` for varchar columns in SQLite, which is invalid SQL. v2 maps all text-like types to plain `TEXT`.

7. **`ConnectionManager.Add` no longer holds lock during `Open()`** — v1 held the write lock across the entire `Open()` call (which may be slow). v2 releases the lock before `Open()` and re-acquires it for a double-check.

8. **`ModelToMap` and `ToJSON` signatures unchanged but enhanced** — Signatures remain identical but now properly apply casting transforms (JSON marshal, date formatting).

9. **`SelectColumns` moved from `model.go` to `serialization.go`** — No functional change, just file reorganization for clarity.
