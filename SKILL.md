---
name: entigo
description: Generic entity framework for Go + GORM. Use when defining GORM models with automatic CRUD, building REST APIs with Gin, implementing query filtering/pagination/sorting, or generating DTOs from struct tags. Covers entigo.BaseEntity, EntityService, Converter, BaseHandler, ent scope tags, filter operators, ConditionBuilder, and SQLBuilder.
user-invocable: false
---

# entigo — Generic Entity Framework for Go + GORM

entigo provides generic CRUD, dynamic DTO generation, query filtering, and Gin HTTP handlers — all driven by `ent` struct tags. One struct definition replaces hand-written CreateRequest, UpdateRequest, PatchRequest, and Response types.

Module: `github.com/githonllc/entigo`

## Core Pattern: Define Model → Get Everything

```go
import "github.com/githonllc/entigo"

type User struct {
    entigo.BaseEntity
    Username string `json:"username" gorm:"uniqueIndex;not null" ent:"scope=create,response,filter"`
    Email    string `json:"email" gorm:"uniqueIndex;not null" ent:"scope=create,update,response,filter"`
    Password string `json:"password" gorm:"not null" ent:"scope=create"`
    Bio      string `json:"bio" ent:"scope=create,update,patch,response"`
    Role     string `json:"role" gorm:"default:user" ent:"scope=response,filter"`
}

func (u *User) GenerateID() entigo.ID { return entigo.NewID() }
```

This single struct produces:
- **CreateRequest**: `{username, email, password, bio}` — scope=create fields
- **UpdateRequest**: `{email, bio}` — scope=update fields
- **PatchRequest**: `{bio*}` — scope=patch fields as pointers
- **Response**: `{id, username, email, bio, role, created_at, updated_at}` — scope=response fields
- **Filterable**: username, email, role — scope=filter fields

Password is `scope=create` only — never appears in responses, updates, or patches.

## ent Tag Syntax

```
ent:"scope=<name>[(<attrs>)],<name2>,..."
```

| Scope | Purpose | Notes |
|-------|---------|-------|
| `create` | POST request DTO | |
| `update` | PUT request DTO | |
| `patch` | PATCH request DTO | Fields become `*T` pointer types |
| `response` | GET response DTO | |
| `filter` | Filterable via query params | Supports operator prefixes |

Attributes (in parentheses): `readonly`, `required`, `max=N`, `min=N`, `default=V`, `name=X` (query param), `col=X` (DB column).

## Service Setup

```go
entigo.InitIDGenerator(1) // snowflake node ID, call once at startup

opts := entigo.NewServiceOptions()
opts.With(entigo.OptionKeyDB, db)                // *gorm.DB — REQUIRED
opts.With(entigo.OptionKeyReplicaDB, replicaDB)  // *gorm.DB — REQUIRED
opts.With(entigo.OptionKeyCache, entigo.NewInMemCache())        // optional
opts.With(entigo.OptionKeyAudit, entigo.NewDummyAuditService()) // optional
opts.With(entigo.OptionKeyTracer, myTracer)                     // optional
opts.With(entigo.OptionKeyContextExtractor, myExtractor)        // optional

service := entigo.NewEntityService[*User](opts)
```

## EntityService[T] Key Methods

```go
service.Create(ctx, entity)
service.GetByID(ctx, id)
service.Update(ctx, entity, fieldsToUpdate...)
service.Patch(ctx, id, map[string]any{"Bio": "new bio"})
service.Delete(ctx, id)  // soft delete

service.List(ctx,
    entigo.WithFilter(filters),
    entigo.WithOrder("created_at", true),
    entigo.WithPagination(1, 25),
)
service.Query(ctx, entigo.WithWhere("status = ?", "active"))
service.QueryFirst(ctx, entigo.WithWhere("email = ?", email))
service.Count(ctx, entigo.WithFilter(filters))

service.WithTransaction(ctx, func(txCtx context.Context, txDb *gorm.DB) error {
    return nil // all service calls with txCtx share this transaction
})
```

## Filter Operators (query parameter prefixes)

```
?name=ilike:john       → LOWER(name) LIKE LOWER('%john%')
?age=gt:18             → age > 18
?age=ge:18 / le:65     → age >= 18 / age <= 65
?status=ne:deleted     → status != 'deleted'
?role=in:admin,mod     → role IN ('admin','mod')
?created_at=from:2024-01-01T00:00:00Z   → >=
?created_at=between:START,END           → BETWEEN
?deleted_at=null:      → IS NULL
?email=not_null:       → IS NOT NULL
```

## ConditionBuilder

```go
cb := entigo.NewConditionBuilder().
    And("status = ?", "active").
    GroupStart().
        Or("role = ?", "admin").
        Or("role = ?", "moderator").
    GroupEnd().
    AndIf(entigo.NotEmpty(search), "name ILIKE ?", "%"+search+"%")

query, args := cb.Build()
```

## SQLBuilder (with CTEs)

```go
qb := entigo.NewSQLBuilder().
    With("active", "SELECT * FROM users WHERE status = ?", "active").
    Select("region, COUNT(*)").From("active").
    Where("age > ?", 18).GroupBy("region").OrderBy("cnt DESC")

total, results, err := entigo.ExecutePaginatedQuery[Result](ctx, db, qb, 0, 10)
```

## Gin Integration (ginx sub-package)

```go
import "github.com/githonllc/entigo/ginx"

handler := ginx.NewBaseHandler[MyService, *MyEntity](service)

g := r.Group("/users")
g.GET("", handler.List)        // pagination + filters + sorting
g.GET("/:id", handler.Get)
g.POST("", handler.Create)
g.PUT("/:id", handler.Update)
g.PATCH("/:id", handler.Patch)
g.DELETE("/:id", handler.Delete)
g.GET("/export", handler.Export) // CSV
```

### Handler Hooks

```go
handler.Hooks.BeforeCreate = func(c *gin.Context, u *User) error {
    u.Password = hash(u.Password)
    return nil
}
```

### Response/Error Helpers

```go
handler.Success(c, "ok", entity)
handler.Error(c, err)
ginx.SendOK(c, "ok", data, "count", total)
ginx.WriteError(c, err)
ginx.HandleDBError(err) // GORM/PostgreSQL → CustomError
ginx.NewBadRequestError("msg") / NewNotFoundError / NewForbiddenError / ...
```

### Context Helpers

```go
ctx := ginx.RequireContext(c)       // populate entigo context keys from Gin
ginx.IsAdmin(c)
ginx.IsAdminOrCurrentUser(c, id)
ginx.QueryParamsToQueryMap(c)
```

## ContextExtractor — Injectable Actor Resolution

```go
type MyExtractor struct{}
func (e MyExtractor) Extract(ctx context.Context) entigo.ActorInfo {
    return entigo.ActorInfo{
        ActorType: entigo.ActorTypeUser,
        ActorID:   entigo.ID(ctx.Value("uid").(int64)),
        IsAdmin:   ctx.Value("role") == "admin",
    }
}
opts.With(entigo.OptionKeyContextExtractor, MyExtractor{})
```

## Extending EntityService

```go
type UserService interface {
    entigo.EntityService[*User]
    GetByEmail(ctx context.Context, email string) (*User, error)
}

type userServiceImpl struct {
    entigo.EntityService[*User]
}

func (s *userServiceImpl) GetByEmail(ctx context.Context, email string) (*User, error) {
    return s.QueryFirst(ctx, entigo.WithWhere("email = ?", email))
}
```

## Key Design Rules

1. **One struct = all DTOs** — never write separate request/response types
2. **Password pattern** — `scope=create` only, never in response
3. **Immutable fields** — include only in `create`, not `update`/`patch`
4. **Admin-only fields** — `scope=response,filter` only, set via custom endpoints
5. **Patch uses pointers** — `*T` fields distinguish "not sent" from "zero value"
6. **GenerateID() required** — every model must implement it
7. **Validate() auto-called** — by Create and Update if implemented
8. **Optimistic locking** — Revision auto-increments, concurrent updates error
9. **Soft delete** — Delete sets DeletedAt, records hidden not removed
10. **Transactions propagate** — nested service calls share tx via context
