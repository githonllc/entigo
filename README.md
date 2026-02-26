# entigo

[![Go Reference](https://pkg.go.dev/badge/github.com/githonllc/entigo.svg)](https://pkg.go.dev/github.com/githonllc/entigo)
[![Go Report Card](https://goreportcard.com/badge/github.com/githonllc/entigo)](https://goreportcard.com/report/github.com/githonllc/entigo)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A generic entity framework for Go built on GORM. Define your model once with struct tags, get automatic DTO generation, CRUD operations, optimistic locking, filtering, caching, and audit logging -- no code generation, no handwritten DTOs.

## Features

- **Single struct definition** -- one model drives create/update/patch/response DTOs via reflection
- **`ent` struct tags** -- declarative field scoping with attributes (required, readonly, min, max, default)
- **Generic `EntityService[T]`** -- type-safe CRUD with optimistic locking and transaction propagation
- **`Converter[T]`** -- automatic DTO generation at runtime, no codegen step
- **Filter system** -- query parameter operators (`like:`, `gt:`, `in:`, `between:`, etc.)
- **`ConditionBuilder`** -- fluent, composable SQL WHERE clause construction
- **`SQLBuilder`** -- CTE support, DISTINCT ON, paginated queries
- **Snowflake IDs** -- 64-bit IDs serialized as JSON strings for JavaScript safety
- **Pluggable caching** -- `CacheService` interface with built-in `InMemCache` and `DummyCache`
- **Audit logging** -- `AuditService` interface with functional option event construction
- **Tracing** -- `Tracer`/`Span` interface with `NoopTracer` default; bring your own (Sentry, OpenTelemetry)
- **Injectable context extraction** -- `ContextExtractor` interface to customize how actor info is resolved
- **Gin integration** -- `ginx` sub-package provides `BaseHandler[S,T]` for automatic REST endpoints

## Installation

```bash
go get github.com/githonllc/entigo
```

For the Gin integration sub-package:

```bash
go get github.com/githonllc/entigo/ginx
```

## Quick Start

### 1. Define a model

```go
package models

import "github.com/githonllc/entigo"

type Article struct {
    entigo.BaseEntity
    Title   string `json:"title"   ent:"scope=create,update,response,filter"`
    Content string `json:"content" ent:"scope=create,update,response"`
    Status  string `json:"status"  ent:"scope=response,filter"`
    Views   int    `json:"views"   ent:"scope=response,filter"`
}

func (a *Article) GenerateID() entigo.ID {
    return entigo.NewID()
}
```

### 2. Create a service

```go
opts := entigo.NewServiceOptions().
    With(entigo.OptionKeyDB, db).
    With(entigo.OptionKeyReplicaDB, replicaDB).
    With(entigo.OptionKeyCache, entigo.NewInMemCache()).
    With(entigo.OptionKeyTracer, entigo.NoopTracer{})

articleService := entigo.NewEntityService[*Article](opts)
```

### 3. CRUD operations

```go
ctx := context.Background()

// Create
article := &Article{Title: "Hello", Content: "World"}
err := articleService.Create(ctx, article)

// Read
found, err := articleService.GetByID(ctx, article.GetID())

// Update (with optimistic locking)
found.Title = "Updated"
err = articleService.Update(ctx, found)

// Query with filters and pagination
articles, err := articleService.List(ctx,
    entigo.WithFilter(map[string]any{"status": "published"}),
    entigo.WithOrder("created_at", true),
    entigo.WithPagination(1, 10),
)

// Delete (soft delete)
err = articleService.Delete(ctx, article.GetID())
```

## Core Concepts

### Entity & BaseEntity

All models embed `entigo.BaseEntity`, which provides:

| Field       | Type             | Description                                |
|-------------|------------------|--------------------------------------------|
| `ID`        | `entigo.ID`      | Snowflake-based primary key                |
| `CreatedAt` | `time.Time`      | Auto-set on creation                       |
| `UpdatedAt` | `time.Time`      | Auto-updated on save                       |
| `DeletedAt` | `gorm.DeletedAt` | Soft-delete timestamp (excluded from JSON) |
| `Revision`  | `int`            | Optimistic locking counter                 |

The `Entity` interface provides lifecycle hooks (`BeforeCreate`, `AfterCreate`, `BeforeUpdate`, etc.), `Validate()`, soft-delete helpers (`SoftDelete`, `Restore`, `IsDeleted`), and revision accessors.

Override `GenerateID()` in your model to use custom ID generation logic. The default implementation calls `entigo.NewID()`.

### ent Tag System

The `ent` struct tag controls which fields appear in which DTO. Fields are tagged with scopes that the `Converter` reads at runtime via reflection.

**Syntax:**

```
ent:"scope=<scope1>,<scope2>(<attr1>,<attr2>=<value>)"
```

**Scopes:**

| Scope      | Purpose                                      |
|------------|----------------------------------------------|
| `create`   | Included in create request DTO               |
| `update`   | Included in update request DTO               |
| `patch`    | Included in patch request DTO (pointer fields)|
| `response` | Included in response DTO                     |
| `filter`   | Field is filterable via query parameters      |

**Attributes (within parentheses):**

| Attribute  | Example               | Description                          |
|------------|-----------------------|--------------------------------------|
| `readonly` | `update(readonly)`    | Field is present but cannot be modified |
| `required` | `create(required)`    | Field must be provided               |
| `max`      | `update(max=100)`     | Maximum value/length                 |
| `min`      | `update(min=0)`       | Minimum value/length                 |
| `default`  | `create(default=active)` | Default value when absent         |

**Filter options:**

The `filter` scope supports `name` and `col` options to customize the query parameter name and database column name:

```go
SerialNo string `json:"serial_no" ent:"scope=filter(name=sn,col=serial_number)"`
// Query: ?sn=ABC123  ->  WHERE serial_number = 'ABC123'
```

**Example:**

```go
type User struct {
    entigo.BaseEntity
    Name   string `json:"name"   ent:"scope=create(required),update,response,filter"`
    Email  string `json:"email"  ent:"scope=create(required),update(readonly),response,filter"`
    Status string `json:"status" ent:"scope=response,filter"`
    Score  int    `json:"score"  ent:"scope=create,update(min=0,max=100),response"`
}
```

This single definition produces four distinct DTOs:
- **Create DTO**: `{name, email, score}`
- **Update DTO**: `{name, email (readonly), score}`
- **Patch DTO**: `{name *string, email *string, score *int}` (pointer fields)
- **Response DTO**: `{id, created_at, updated_at, name, email, status, score}`

### EntityService[T]

`EntityService[T]` is a generic interface providing full CRUD, querying, transactions, caching, and audit logging. Create one with `NewEntityService`:

```go
service := entigo.NewEntityService[*MyModel](options)
```

**Key methods:**

| Method            | Description                                      |
|-------------------|--------------------------------------------------|
| `Create`          | Insert with auto-ID, validation, cache, audit    |
| `GetByID`         | Fetch by primary key with association preloading  |
| `Update`          | Full update with optimistic locking               |
| `Patch`           | Partial update (map or struct with pointer fields)|
| `Delete`          | Soft delete                                       |
| `List`            | Query with `QueryOption` composable filters       |
| `Query`           | General query with options                        |
| `QueryFirst`      | Return first matching record                      |
| `Count`           | Count matching records                            |
| `GetOrCreate`     | Return existing or create new                     |
| `Upsert`          | Create or update by key column                    |
| `Exec`            | Execute raw SQL                                   |
| `WithTransaction` | Execute function within a database transaction    |

**Optimistic locking:** Every `Update` and `Patch` checks the `Revision` field. If another transaction modified the record, `ErrConcurrentModification` is returned. The revision is atomically incremented on success.

**Transactions:** `WithTransaction` stores the transaction in context. Nested service calls automatically participate in the same transaction:

```go
err := service.WithTransaction(ctx, func(txCtx context.Context, txDb *gorm.DB) error {
    if err := serviceA.Create(txCtx, entityA); err != nil {
        return err // triggers rollback
    }
    return serviceB.Update(txCtx, entityB)
})
```

**QueryOption composition:**

```go
results, err := service.List(ctx,
    entigo.WithWhere("status = ?", "active"),
    entigo.WithFilter(map[string]any{"name": "ilike:john"}),
    entigo.WithOrder("created_at", true),
    entigo.WithPagination(1, 25),
)
```

### Converter[T]

`Converter[T]` uses reflection to generate request/response DTOs from `ent` scope tags at runtime. Types are computed once and cached.

```go
converter := entigo.NewConverter[*Article](&Article{})

// Generate DTO instances
createDTO := converter.GenCreateRequest()   // only "create" scoped fields
updateDTO := converter.GenUpdateRequest()   // only "update" scoped fields
patchDTO  := converter.GenPatchRequest()    // "patch" scoped fields as pointers
respDTO   := converter.GenResponse()        // only "response" scoped fields

// Convert between model and DTO
model, err := converter.ToModel(createDTO)
resp, err  := converter.ToResponse(model)
items, err := converter.ToListResponse(models)
```

The flow: **ent tags -> Converter reads scopes via reflection -> generates struct types at startup -> creates DTO instances at request time -> copies fields via copier**.

No code generation. No separate DTO files. One struct definition drives everything.

### Filter System

Filters support operator prefixes on string values. Pass them via `WithFilter` or `SQLBuilder.ApplyFilter`:

| Prefix     | SQL                       | Example value                              |
|------------|---------------------------|--------------------------------------------|
| *(none)*   | `= ?`                    | `"active"`                                 |
| `like:`    | `LIKE ?`                 | `"like:john"`                              |
| `ilike:`   | `LOWER() LIKE LOWER()`   | `"ilike:john"`                             |
| `gt:`      | `> ?`                    | `"gt:100"`                                 |
| `ge:`      | `>= ?`                   | `"ge:100"`                                 |
| `lt:`      | `< ?`                    | `"lt:50"`                                  |
| `le:`      | `<= ?`                   | `"le:50"`                                  |
| `ne:`      | `<> ?`                   | `"ne:0"`                                   |
| `in:`      | `IN (?)`                 | `"in:a,b,c"`                               |
| `from:`    | `>= ?` (RFC3339 time)    | `"from:2024-01-01T00:00:00Z"`              |
| `to:`      | `<= ?` (RFC3339 time)    | `"to:2024-12-31T23:59:59Z"`                |
| `between:` | `BETWEEN ? AND ?`        | `"between:2024-01-01T00:00:00Z,2024-12-31T23:59:59Z"` |
| `json:`    | JSONB query              | `"json:key=value"` or `"json:key~partial"` |
| `null:`    | `IS NULL`                | `"null:"`                                  |
| `not_null:`| `IS NOT NULL`            | `"not_null:"`                              |

**Type-aware filtering from ent tags:**

```go
// Automatically parse filter scopes from struct tags and build conditions
filters := entigo.BuildFiltersForType[*Article](queryMap)
results, err := service.List(ctx, entigo.WithFilter(filters))
```

### ConditionBuilder

Fluent API for constructing complex WHERE clauses with AND/OR groups and conditional inclusion:

```go
cb := entigo.NewConditionBuilder().
    And("status = ?", "active").
    AndIf(entigo.NotEmpty(name), "name ILIKE ?", "%"+name+"%").
    OrGroupStart().
        And("role = ?", "admin").
        And("level > ?", 5).
    GroupEnd()

query, args := cb.Build()
// Use with service:
results, err := service.List(ctx, entigo.WithConditionBuilder(cb))
```

Helper check functions: `NotEmpty(string)`, `NotZero(int)`, `NotNil(any)`.

### SQLBuilder

For complex queries with CTEs, DISTINCT ON, and manual pagination:

```go
qb := entigo.NewSQLBuilder().
    With("recent", "SELECT * FROM articles WHERE created_at > ?", cutoff).
    Select("*").
    From("recent").
    Where("status = ?", "published").
    OrderBy("created_at DESC")

total, results, err := entigo.ExecutePaginatedQuery[Article](ctx, db, qb, offset, size)
```

Supports `DistinctOn` (PostgreSQL), `GroupBy`, `ApplyFilter`, and nested CTEs via `WithBuilder`.

### ContextExtractor

`ContextExtractor` is an interface that allows you to customize how entigo resolves actor identity from `context.Context`. This is useful when your application uses a different context layout than the built-in `CtxKey*` constants.

```go
type ContextExtractor interface {
    Extract(ctx context.Context) ActorInfo
}
```

`ActorInfo` contains:

| Field        | Type        | Description                     |
|--------------|-------------|---------------------------------|
| `ActorType`  | `ActorType` | User, API key, etc.             |
| `ActorID`    | `ID`        | The authenticated actor's ID    |
| `IdentityID` | `ID`        | Underlying account/identity ID  |
| `IsAdmin`    | `bool`      | Whether the actor is an admin   |
| `IPAddress`  | `string`    | Client IP address               |
| `UserAgent`  | `string`    | User-Agent header value         |

**Injecting a custom extractor:**

```go
opts := entigo.NewServiceOptions().
    With(entigo.OptionKeyDB, db).
    With(entigo.OptionKeyReplicaDB, replicaDB).
    With(entigo.OptionKeyContextExtractor, myCustomExtractor)
```

If no extractor is provided, a default implementation reads from the built-in `CtxKeyUserID`, `CtxKeyApiKeyID`, `CtxKeyIsAdmin`, etc. context keys.

### ID Generation

IDs are snowflake-based 64-bit integers (`entigo.ID`). They serialize to JSON as strings to avoid JavaScript precision loss.

```go
// Initialize with a node ID (0-1023). Call once at startup.
entigo.InitIDGenerator(1)

// Generate IDs
id := entigo.NewID()

// Parse from various types
id, err := entigo.ParseID("1234567890")
id, err := entigo.ParseID(int64(1234567890))

// Validate
if entigo.IsInvalidID(id) { ... }
```

If `InitIDGenerator` is not called, the generator auto-initializes with node 0 on first use.

### Caching

The `CacheService` interface provides string-based key-value caching with a 5-minute default TTL.

```go
type CacheService interface {
    Get(key string) (string, error)
    Set(key string, value string) error
    Delete(key string) error
}
```

**Built-in implementations:**

| Type         | Description                                            |
|--------------|--------------------------------------------------------|
| `InMemCache` | Thread-safe in-memory cache with lazy TTL expiration   |
| `DummyCache` | No-op cache (Get always returns `ErrCacheMiss`)        |

**Object caching helpers:**

```go
// Store and retrieve typed objects (JSON serialized)
entigo.SetObjectCache(cache, "user:123", user)
entigo.GetObjectCache(cache, "user:123", &user)

// With custom TTL (requires CacheServiceWithExp)
entigo.SetObjectCacheExp(cache, "key", value, 10*time.Minute)
```

### Audit Logging

The `AuditService` interface records entity operation events. Events are constructed with functional options:

```go
type AuditService interface {
    LogEvent(ctx context.Context, entry *AuditLogEvent)
}
```

```go
event := entigo.NewAuditLogEvent(
    entigo.WithAction("CREATE"),
    entigo.WithResourceType("article"),
    entigo.WithResourceID(id),
    entigo.WithActorType(entigo.ActorTypeUser),
    entigo.WithActorID(userID),
    entigo.WithDetails(map[string]any{"title": "New Article"}),
)
auditService.LogEvent(ctx, event)
```

`EntityService` automatically logs audit events for Create, Update, Patch, and Delete operations. Use `NewDummyAuditService()` to disable.

### Tracing

The `Tracer` interface creates spans for tracking operations:

```go
type Tracer interface {
    StartSpan(ctx context.Context, operation string) Span
}

type Span interface {
    Finish()
}
```

Use `NoopTracer{}` as the default. Implement the interface to integrate with Sentry, OpenTelemetry, Datadog, or any tracing backend. All `EntityService` and `BaseHandler` methods create spans automatically.

### Context Keys

`ContextKey` is a typed `string` for storing values in `context.Context`. Built-in keys:

| Key                    | Type   | Description                       |
|------------------------|--------|-----------------------------------|
| `CtxKeyUserID`         | `ID`   | Authenticated user ID             |
| `CtxKeyIdentityID`     | `ID`   | Underlying account/identity ID    |
| `CtxKeyApiKeyID`       | `ID`   | API key used for authentication   |
| `CtxKeyIsAdmin`        | `bool` | Whether the user is an admin      |
| `CtxKeyRealIP`         | `string`| Client IP from proxy headers     |
| `CtxKeyUserAgent`      | `string`| User-Agent header value          |
| `CtxKeyClientIP`       | `string`| Direct connection IP             |

Define custom keys for your domain:

```go
const CtxKeyTenantID entigo.ContextKey = "myapp.tenant_id"
```

## Gin Integration (ginx)

The `github.com/githonllc/entigo/ginx` sub-package provides Gin-specific HTTP handler support.

### BaseHandler[S,T]

`BaseHandler` provides ready-to-use HTTP handler methods for standard REST operations:

```go
handler := ginx.NewBaseHandler[ArticleService, *Article](articleService)

r := gin.Default()
r.GET("/articles",     handler.List)     // paginated list with filters
r.GET("/articles/:id", handler.Get)      // get by ID
r.POST("/articles",    handler.Create)   // create from request body
r.PUT("/articles/:id", handler.Update)   // full update
r.PATCH("/articles/:id", handler.Patch)  // partial update
r.DELETE("/articles/:id", handler.Delete)// soft delete
r.GET("/articles/export", handler.Export)// CSV export
```

`List` automatically parses `?page=`, `?size=`, `?order=`, and filter query parameters. Request bodies are automatically bound to the appropriate DTO (create, update, or patch) generated by the `Converter`.

### HandlerHooks

Customize lifecycle behavior without subclassing:

```go
handler := ginx.NewBaseHandler[ArticleService, *Article](articleService)
handler.Hooks = ginx.HandlerHooks[*Article]{
    BeforeCreate: func(c *gin.Context, entity *Article) error {
        entity.Status = "draft"
        return nil
    },
    AfterCreate: func(c *gin.Context, entity *Article) error {
        // send notification, etc.
        return nil
    },
    BeforeUpdate: func(c *gin.Context, entity *Article) error { ... },
    AfterUpdate:  func(c *gin.Context, entity *Article) error { ... },
    BeforePatch:  func(c *gin.Context, input any) error { ... },
    AfterPatch:   func(c *gin.Context, input any) error { ... },
}
```

### Error Handling

`CustomError` carries an HTTP status code, error code, and message:

```go
type CustomError struct {
    Message    string
    ErrorCode  int
    HTTPStatus int
}
```

**Predefined errors:** `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrTooManyRequests`, `ErrInternalServer`.

**Error constructors:** `NewBadRequestError(msg)`, `NewNotFoundError(msg)`, `NewConflictError(msg)`, etc.

**`HandleDBError`** translates GORM and PostgreSQL errors into `CustomError` values:

- `gorm.ErrRecordNotFound` -> 404
- `gorm.ErrDuplicatedKey` / PG code 23505 -> 400 (duplicate)
- PG code 23503 -> 400 (foreign key violation)
- `context.Canceled` / `context.DeadlineExceeded` -> 408

**`ResponseError`** sends the appropriate JSON error response based on the error type.

### Response Format

All responses use a standard JSON envelope:

```json
{
    "ok": true,
    "message": "optional message",
    "data": { ... },
    "count": 100
}
```

Error responses:

```json
{
    "ok": false,
    "message": "error description",
    "error_code": 40400
}
```

### CSV Export

The `Export` handler and `ToCSV` function convert response DTOs to CSV:

```go
data, err := ginx.ToCSV(entities, ginx.WithComma(ginx.SemicolonSeparator))
```

Options: `WithComma(rune)` for custom delimiters, `WithUseCRLF(bool)` for line endings.

### Helpers

- `RequireContext(c)` -- creates a `context.Context` populated with auth metadata from Gin context
- `QueryParamsToQueryMap(c)` -- converts Gin query params to `entigo.QueryMap`
- `GetIDFromContext(c, key)` -- retrieves an `entigo.ID` from Gin context
- `IsAdmin(c)`, `IsCurrentUser(c, id)`, `IsAdminOrCurrentUser(c, id)` -- auth checks

## Configuration

Wire up dependencies via `ServiceOptions`:

```go
opts := entigo.NewServiceOptions()

// Required: primary and replica database connections
opts.With(entigo.OptionKeyDB, primaryDB)
opts.With(entigo.OptionKeyReplicaDB, replicaDB)

// Optional: caching (defaults to DummyCache)
opts.With(entigo.OptionKeyCache, entigo.NewInMemCache())

// Optional: audit logging (defaults to DummyAuditService)
opts.With(entigo.OptionKeyAudit, myAuditService)

// Optional: distributed tracing (defaults to NoopTracer)
opts.With(entigo.OptionKeyTracer, myTracer)

// Optional: custom context extraction (defaults to built-in CtxKey* reader)
opts.With(entigo.OptionKeyContextExtractor, myExtractor)

// Debug flags
opts.DebugMode = true    // log all operations
opts.DebugSQLMode = true // log SQL queries

// Clone for a second service with different settings
opts2 := opts.Clone()
opts2.With(entigo.OptionKeyDB, anotherDB)
```

**Available option keys:**

| Key                         | Type               | Required | Description              |
|-----------------------------|--------------------|----------|--------------------------|
| `OptionKeyDB`               | `*gorm.DB`         | Yes      | Primary database         |
| `OptionKeyReplicaDB`        | `*gorm.DB`         | Yes      | Read replica database    |
| `OptionKeyCache`            | `CacheService`     | No       | Caching backend          |
| `OptionKeyAudit`            | `AuditService`     | No       | Audit log backend        |
| `OptionKeyTracer`           | `Tracer`           | No       | Distributed tracing      |
| `OptionKeyContextExtractor` | `ContextExtractor` | No       | Actor info extraction    |
| `OptionKeyServiceContext`   | `any`              | No       | Custom service context   |

## Dependencies

| Package                        | Purpose                                   |
|--------------------------------|-------------------------------------------|
| `gorm.io/gorm`                 | ORM for database operations               |
| `github.com/bwmarrin/snowflake`| Snowflake ID generation                   |
| `github.com/jinzhu/copier`     | Struct-to-struct field copying             |
| `github.com/gin-gonic/gin`     | HTTP framework (ginx sub-package)         |
| `github.com/jackc/pgx/v5`      | PostgreSQL driver (error handling)        |
| `github.com/gocarina/gocsv`    | CSV marshaling (ginx sub-package)         |

## Contributing

Contributions are welcome. Please open an issue to discuss proposed changes before submitting a pull request. All code, comments, and commit messages must be in English.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Commit your changes
4. Push to the branch and open a pull request

## License

MIT -- see [LICENSE](LICENSE) for details.
