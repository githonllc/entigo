// Package entigo provides a generic entity framework for Go built on GORM.
//
// It features dynamic DTO generation via reflection-based struct tags,
// generic CRUD operations with optimistic locking, flexible query filtering,
// caching, audit logging, tracing, and snowflake-based ID generation.
//
// # Key types
//
//   - [Entity] and [BaseEntity] -- domain model interface and base implementation
//   - [EntityService] -- generic CRUD service with optimistic locking and transactions
//   - [Converter] -- automatic request/response DTO generation from ent struct tags
//   - [ConditionBuilder] -- fluent, composable SQL WHERE clause construction
//   - [SQLBuilder] -- complex query builder with CTE and DISTINCT ON support
//   - [ContextExtractor] -- injectable interface for resolving actor info from context
//   - [CacheService] -- pluggable caching with [InMemCache] and [DummyCache]
//   - [AuditService] -- pluggable audit logging with functional option events
//   - [Tracer] and [Span] -- pluggable distributed tracing with [NoopTracer] default
//
// # Module path
//
//	github.com/githonllc/entigo
//
// The ginx sub-package (github.com/githonllc/entigo/ginx) provides Gin-specific
// HTTP handler support via [BaseHandler].
package entigo
