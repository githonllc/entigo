package entigo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// txContextKey is a typed key for storing GORM transactions in context.Context.
// Using a distinct unexported type prevents collisions with other packages.
type txContextKey struct{}

// ctxKeyTx is the context key for transaction propagation.
var ctxKeyTx = txContextKey{}

// EntityService defines the generic service interface for CRUD operations on entities.
//
// Usage:
//
//	service := NewEntityService[*YourEntityType](options)
//
//	// Create
//	entity := &YourEntityType{...}
//	err := service.Create(ctx, entity)
//
//	// Read
//	entity, err := service.GetByID(ctx, id)
//
//	// Update
//	entity.SomeField = newValue
//	err := service.Update(ctx, entity)
//
//	// Delete
//	err := service.Delete(ctx, id)
//
//	// Complex query
//	entities, err := service.Query(ctx,
//	    WithWhere("field = ?", value),
//	    WithOrder("created_at", true),
//	    WithPagination(1, 10),
//	)
//
//	// Transaction
//	err := service.WithTransaction(ctx, func(txCtx context.Context, txDb *gorm.DB) error {
//	    // ... multiple operations ...
//	    return nil
//	})
type EntityService[T Entity] interface {
	GeneralService
	GetZeroValue() T
	NewModelInstance() T
	GetEntityName() string
	GetDB(ctx context.Context) *gorm.DB
	GetReplicaDB(ctx context.Context) *gorm.DB
	GetSchema() (*schema.Schema, error)
	Create(ctx context.Context, entity T) error
	GetOrCreate(ctx context.Context, entity T) (T, error)
	Upsert(ctx context.Context, entity T, keyColumn string, keyValue any) error
	GetByID(ctx context.Context, id ID) (T, error)
	Update(ctx context.Context, entity T, fieldsToUpdate ...string) error
	Patch(ctx context.Context, id ID, data any) error
	Delete(ctx context.Context, id ID) error
	List(ctx context.Context, opts ...QueryOption) ([]T, error)
	Exec(ctx context.Context, sql string, values ...any) error
	ExecWithDB(db *gorm.DB, sql string, values ...any) error
	Query(ctx context.Context, opts ...QueryOption) ([]T, error)
	QueryFirst(ctx context.Context, opts ...QueryOption) (T, error)
	QueryWithDB(db *gorm.DB, opts ...QueryOption) ([]T, error)
	QueryFirstWithDB(db *gorm.DB, opts ...QueryOption) (T, error)
	Count(ctx context.Context, opts ...QueryOption) (int64, error)
	WithTransaction(ctx context.Context, fn func(txCtx context.Context, txDb *gorm.DB) error) error
	IsZeroValue(entity T) bool
	InvalidCache(ctx context.Context, id ID)
	UpdateCache(ctx context.Context, entity T)
	ValidateAccessibleAsUser(ctx context.Context, userID ID) error
	LogAuditEvent(ctx context.Context, entry *AuditLogEvent)
	NewAuditLogEvent(ctx context.Context, action string, id ID, data map[string]any) *AuditLogEvent
	NewAuditLogEventExtra(ctx context.Context, opts ...AuditLogEventOption) *AuditLogEvent
}

// entityServiceImpl is the concrete implementation of EntityService[T].
type entityServiceImpl[T Entity] struct {
	GeneralServiceImpl
	zeroValue T
	options   *ServiceOptions
	db        *gorm.DB
	replicaDb *gorm.DB
}

// NewEntityService creates a new EntityService for the given entity type.
// The options context map must contain OptionKeyDB and OptionKeyReplicaDB
// with *gorm.DB values.
func NewEntityService[T Entity](options *ServiceOptions) EntityService[T] {
	dbRaw := options.Get(OptionKeyDB)
	if dbRaw == nil {
		panic("entigo: OptionKeyDB is required but not set in ServiceOptions")
	}
	replicaRaw := options.Get(OptionKeyReplicaDB)
	if replicaRaw == nil {
		panic("entigo: OptionKeyReplicaDB is required but not set in ServiceOptions")
	}

	db, ok := dbRaw.(*gorm.DB)
	if !ok {
		panic("entigo: OptionKeyDB must be *gorm.DB")
	}
	replicaDb, ok := replicaRaw.(*gorm.DB)
	if !ok {
		panic("entigo: OptionKeyReplicaDB must be *gorm.DB")
	}

	service := &entityServiceImpl[T]{
		GeneralServiceImpl: *NewGeneralService(options),
		options:            options,
		db:                 db,
		replicaDb:          replicaDb,
	}

	slog.Info("New entity service", "name", service.GetServiceName(), "entity", service.GetEntityName())
	return service
}

// ValidateAccessibleAsUser checks that the current user (from context) is
// either an admin or matches the given userID. Returns ErrPermissionDenied
// if the check fails. Actor info is resolved via the configured ContextExtractor.
func (s *entityServiceImpl[T]) ValidateAccessibleAsUser(ctx context.Context, userID ID) error {
	actor := s.ctxExtr.Extract(ctx)
	if !actor.IsAdmin && actor.ActorID != userID {
		return ErrPermissionDenied
	}
	return nil
}

// GetZeroValue returns the zero value of the entity type T.
func (s *entityServiceImpl[T]) GetZeroValue() T {
	return s.zeroValue
}

// NewModelInstance creates a new instance of the entity type T.
// If T is a pointer type, it allocates the underlying struct.
func (s *entityServiceImpl[T]) NewModelInstance() T {
	return NewInstance[T]()
}

// GetEntityName returns the struct name of the entity type.
func (s *entityServiceImpl[T]) GetEntityName() string {
	entityType := reflect.TypeOf(s.zeroValue)
	if entityType.Kind() == reflect.Ptr {
		entityType = entityType.Elem()
	}
	return entityType.Name()
}

// GetServiceName returns the service name.
func (s *entityServiceImpl[T]) GetServiceName() string {
	return "EntityService"
}

// GetDB returns a GORM session scoped to the entity model.
// If the context carries a transaction (set by WithTransaction), the returned
// session participates in that transaction.
func (s *entityServiceImpl[T]) GetDB(ctx context.Context) *gorm.DB {
	var modelType T
	// Check if there is an ongoing transaction in context
	if tx, ok := ctx.Value(ctxKeyTx).(*gorm.DB); ok {
		// Create a new session to ensure clean state while maintaining
		// the transaction connection
		session := tx.Model(modelType).Session(&gorm.Session{
			NewDB: true,
		})

		if s.options.DebugMode {
			session = session.Debug()
		}
		return session
	}

	// For non-transaction calls, create a new session from the main DB
	session := s.db.WithContext(ctx).Model(modelType).Session(&gorm.Session{
		NewDB: true,
	})

	if s.options.DebugMode {
		session = session.Debug()
	}
	return session
}

// GetReplicaDB returns a GORM session for the read replica database.
func (s *entityServiceImpl[T]) GetReplicaDB(ctx context.Context) *gorm.DB {
	db := s.replicaDb.WithContext(ctx)
	if s.options.DebugMode {
		db = db.Debug()
	}
	return db
}

// GetSchema parses and returns the GORM schema for the entity model.
func (s *entityServiceImpl[T]) GetSchema() (*schema.Schema, error) {
	var model T
	err := s.db.Statement.Parse(&model)
	return s.db.Statement.Schema, err
}

// getCacheKey builds a cache key from the entity type and ID.
func (s *entityServiceImpl[T]) getCacheKey(id ID) string {
	var entity T
	return fmt.Sprintf("%T:%d", entity, id)
}

// Create inserts a new entity into the database.
// It generates an ID if none is set, validates the entity, updates the cache,
// and logs an audit event.
func (s *entityServiceImpl[T]) Create(ctx context.Context, entity T) error {
	span := s.tracer.StartSpan(ctx, "EntityService.Create")
	defer span.Finish()

	if s.IsZeroValue(entity) || IsNil(entity) {
		return ErrEntityNil
	}

	if IsInvalidID(entity.GetID()) {
		entity.SetID(entity.GenerateID())
	}

	if err := entity.Validate(); err != nil {
		return err
	}

	if err := s.GetDB(ctx).Create(entity).Error; err != nil {
		return err
	}

	s.UpdateCache(ctx, entity)

	s.audit.LogEvent(ctx, s.NewAuditLogEvent(ctx, "CREATE", entity.GetID(), M{
		"entity": entity,
	}))
	return nil
}

// GetOrCreate returns the existing entity if found by ID, otherwise creates it.
func (s *entityServiceImpl[T]) GetOrCreate(ctx context.Context, entity T) (T, error) {
	span := s.tracer.StartSpan(ctx, "EntityService.GetOrCreate")
	defer span.Finish()

	if res, err := s.GetByID(ctx, entity.GetID()); err == nil {
		return res, nil
	}

	err := s.Create(ctx, entity)
	return entity, err
}

// GetByID retrieves an entity by its primary key, preloading all associations.
func (s *entityServiceImpl[T]) GetByID(ctx context.Context, id ID) (T, error) {
	span := s.tracer.StartSpan(ctx, "EntityService.GetByID")
	defer span.Finish()

	var entity T
	if err := s.GetDB(ctx).Preload(clause.Associations).First(&entity, id).Error; err != nil {
		return entity, err
	}

	s.UpdateCache(ctx, entity)

	return entity, nil
}

// Upsert creates the entity if no record matches the key column, otherwise updates it.
func (s *entityServiceImpl[T]) Upsert(ctx context.Context, entity T, keyColumn string, keyValue any) error {
	span := s.tracer.StartSpan(ctx, "EntityService.Upsert")
	defer span.Finish()

	if _, err := s.QueryFirst(ctx, WithWhere(keyColumn+" = ?", keyValue)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.Create(ctx, entity)
		}
		return err
	}

	return s.Update(ctx, entity)
}

// Update performs an optimistic-locking update on the entity.
// If fieldsToUpdate is empty, all database fields are updated.
// Returns an error if the entity has been concurrently modified.
func (s *entityServiceImpl[T]) Update(ctx context.Context, entity T, fieldsToUpdate ...string) error {
	span := s.tracer.StartSpan(ctx, "EntityService.Update")
	defer span.Finish()

	if s.IsZeroValue(entity) {
		return ErrEntityNil
	}

	if err := entity.Validate(); err != nil {
		return err
	}

	tx := s.GetDB(ctx)

	// Get updatable fields if not specified
	if len(fieldsToUpdate) == 0 {
		fieldsToUpdate = GetDbFields(entity)
	}

	// Verify the entity exists and check revision for optimistic locking
	dbEntity, err := s.GetByID(ctx, entity.GetID())
	if err != nil {
		return fmt.Errorf("entity not found: %v", err)
	}

	if dbEntity.GetRevision() != entity.GetRevision() {
		s.Logger.Warn(
			fmt.Sprintf("concurrent modification detected: id=%d, in_db_revision=%d, hold_revision=%d",
				entity.GetID(), dbEntity.GetRevision(), entity.GetRevision()),
			"hold_entity", entity,
			"entity_in_db", dbEntity,
		)
		return fmt.Errorf("%w: id=%d, in_db_revision=%d, hold_revision=%d",
			ErrConcurrentModification, entity.GetID(), dbEntity.GetRevision(), entity.GetRevision())
	}

	// Optimistic locking: increment revision before update
	currentRevision := entity.GetRevision()
	entity.IncrementRevision()

	result := tx.Model(entity).
		Select(append(fieldsToUpdate, "revision")).
		Where("id = ? AND revision = ?", entity.GetID(), currentRevision).
		Updates(entity)

	if result.Error != nil {
		s.Logger.Error("failed to update data (update data)",
			"error", result.Error,
			"entity", entity,
		)
		return fmt.Errorf("failed to update entity: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		latest, _ := s.GetByID(ctx, entity.GetID())
		s.Logger.Error("concurrent modification detected (zero rows affected)",
			"id", latest.GetID(),
			"entity", entity,
			"latestEntity", latest,
		)
		return fmt.Errorf("%w: id=%d, current_revision=%d, latest_revision=%d",
			ErrConcurrentModification, entity.GetID(), currentRevision, latest.GetRevision())
	}

	// Update cache and audit log
	s.UpdateCache(ctx, entity)
	s.audit.LogEvent(ctx, s.NewAuditLogEvent(ctx, "UPDATE", entity.GetID(), M{"entity": entity}))

	return nil
}

// Patch applies a partial update to the entity identified by id.
// The data parameter can be a map[string]any or a struct (with pointer fields
// indicating which fields to update).
func (s *entityServiceImpl[T]) Patch(ctx context.Context, id ID, data any) error {
	span := s.tracer.StartSpan(ctx, "EntityService.Patch")
	defer span.Finish()

	// Get existing record
	existingEntity, err := s.GetByID(ctx, id)
	if err != nil {
		s.Logger.Error("failed to patch data (get existing entity)",
			"error", err,
			"id", id,
		)
		return err
	}

	// Get model schema
	entitySchema, err := s.GetSchema()
	if err != nil {
		s.Logger.Error("failed to patch data (get entitySchema)",
			"error", err,
			"id", id,
		)
		return err
	}

	// Prepare updates map
	updates := make(map[string]any)

	// Handle different input types
	v := reflect.ValueOf(data)
	switch {
	case v.Kind() == reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			value := iter.Value().Interface()
			if value == nil {
				continue
			}
			// Convert field name if needed
			if dbField, exists := entitySchema.FieldsByName[key]; exists {
				updates[dbField.DBName] = value
			} else if dbField, exists := entitySchema.FieldsByDBName[key]; exists {
				updates[dbField.DBName] = value
			} else {
				return fmt.Errorf("%w: %s", ErrFieldNotFound, key)
			}
		}

	case v.Kind() == reflect.Ptr || v.Kind() == reflect.Struct:
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return errors.New("patch data pointer cannot be nil")
			}
			v = v.Elem()
			if v.Kind() != reflect.Struct {
				return fmt.Errorf("unsupported patch data type: %T", data)
			}
		}

		modelValue := reflect.ValueOf(existingEntity).Elem()

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldName := v.Type().Field(i).Name

			// Check if field is pointer and not nil
			if field.Kind() == reflect.Ptr && !field.IsNil() {
				fieldValue := field.Elem()

				if modelField := modelValue.FieldByName(fieldName); modelField.IsValid() {
					if modelField.CanSet() {
						modelField.Set(fieldValue)

						if dbField, exists := entitySchema.FieldsByName[fieldName]; exists {
							updates[dbField.DBName] = fieldValue.Interface()
						}
					}
				}
			}
		}
	}

	// Perform patch with optimistic locking
	tx := s.GetDB(ctx)
	currentRevision := existingEntity.GetRevision()
	existingEntity.IncrementRevision()
	updates["revision"] = existingEntity.GetRevision()
	result := tx.Model(existingEntity).
		Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id = ? AND revision = ?", id, currentRevision).
		Updates(updates)

	if result.Error != nil {
		s.Logger.Error("failed to patch data (update data)",
			"error", result.Error,
			"id", id,
			"data", data,
			"existingEntity", existingEntity,
		)
		return result.Error
	}

	if result.RowsAffected == 0 {
		latestEntity, _ := s.GetByID(ctx, existingEntity.GetID())
		s.Logger.Error("no rows updated (patch), possibly due to concurrent modification",
			"id", latestEntity.GetID(),
			"data", data,
			"entity", existingEntity,
			"entity.Revision", existingEntity.GetRevision(),
			"latestEntity", latestEntity,
			"latestEntity.Revision", latestEntity.GetRevision(),
		)
		return fmt.Errorf("%w: no rows updated (patch)", ErrConcurrentModification)
	}

	s.InvalidCache(ctx, id)

	s.audit.LogEvent(ctx, s.NewAuditLogEvent(ctx, "PATCH", id, M{
		"fields": updates,
		"entity": existingEntity,
	}))

	return nil
}

// Delete soft-deletes the entity identified by id.
func (s *entityServiceImpl[T]) Delete(ctx context.Context, id ID) error {
	span := s.tracer.StartSpan(ctx, "EntityService.Delete")
	defer span.Finish()

	entity, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if entity.IsDeleted() {
		return ErrEntityAlreadyDeleted
	}

	result := s.GetDB(ctx).Delete(entity)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrNoRowsDeleted
	}

	s.InvalidCache(ctx, id)
	s.audit.LogEvent(ctx, s.NewAuditLogEvent(ctx, "DELETE", id, nil))
	return nil
}

// List retrieves entities matching the query options, ordered by id descending.
func (s *entityServiceImpl[T]) List(ctx context.Context, opts ...QueryOption) ([]T, error) {
	span := s.tracer.StartSpan(ctx, "EntityService.List")
	defer span.Finish()

	var entities []T
	query := s.GetDB(ctx).Preload(clause.Associations)
	for _, opt := range opts {
		query = opt(query)
	}
	query.Order("id desc")
	err := query.Find(&entities).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entities, nil
		}
		return entities, err
	}
	return entities, nil
}

// Exec executes raw SQL on the primary database.
func (s *entityServiceImpl[T]) Exec(ctx context.Context, sql string, values ...any) error {
	span := s.tracer.StartSpan(ctx, "EntityService.Exec")
	defer span.Finish()

	return s.ExecWithDB(s.GetDB(ctx), sql, values...)
}

// ExecWithDB executes raw SQL on the provided database session.
func (s *entityServiceImpl[T]) ExecWithDB(db *gorm.DB, sql string, values ...any) error {
	span := s.tracer.StartSpan(context.Background(), "EntityService.ExecWithDB")
	defer span.Finish()

	result := db.Exec(sql, values...)
	return result.Error
}

// Query retrieves entities matching the query options from the primary database.
func (s *entityServiceImpl[T]) Query(ctx context.Context, opts ...QueryOption) ([]T, error) {
	span := s.tracer.StartSpan(ctx, "EntityService.Query")
	defer span.Finish()

	return s.QueryWithDB(s.GetDB(ctx), opts...)
}

// QueryFirst retrieves the first entity matching the query options.
func (s *entityServiceImpl[T]) QueryFirst(ctx context.Context, opts ...QueryOption) (T, error) {
	span := s.tracer.StartSpan(ctx, "EntityService.QueryFirst")
	defer span.Finish()

	return s.QueryFirstWithDB(s.GetDB(ctx), opts...)
}

// QueryWithDB retrieves entities matching the query options from the given DB session.
func (s *entityServiceImpl[T]) QueryWithDB(db *gorm.DB, opts ...QueryOption) ([]T, error) {
	span := s.tracer.StartSpan(context.Background(), "EntityService.QueryWithDB")
	defer span.Finish()

	var entities []T
	query := db.Preload(clause.Associations)
	for _, opt := range opts {
		query = opt(query)
	}
	err := query.Find(&entities).Error
	if err != nil {
		return entities, err
	}
	return entities, nil
}

// QueryFirstWithDB retrieves the first entity matching the query options from
// the given DB session.
func (s *entityServiceImpl[T]) QueryFirstWithDB(db *gorm.DB, opts ...QueryOption) (T, error) {
	span := s.tracer.StartSpan(context.Background(), "EntityService.QueryFirstWithDB")
	defer span.Finish()

	var entity T
	query := db.Preload(clause.Associations)
	for _, opt := range opts {
		query = opt(query)
	}
	err := query.First(&entity).Error
	if err != nil {
		return s.zeroValue, err
	}
	return entity, nil
}

// Count returns the number of entities matching the query options.
func (s *entityServiceImpl[T]) Count(ctx context.Context, opts ...QueryOption) (int64, error) {
	span := s.tracer.StartSpan(ctx, "EntityService.Count")
	defer span.Finish()

	var entity T
	var count int64
	query := s.GetDB(ctx)
	for _, opt := range opts {
		query = opt(query)
	}
	err := query.Model(&entity).Count(&count).Error
	return count, err
}

// IsZeroValue reports whether the given entity is nil, a nil pointer,
// or equal to the zero value of its type.
func (s *entityServiceImpl[T]) IsZeroValue(entity T) bool {
	v := reflect.ValueOf(entity)
	if !v.IsValid() {
		return true
	}
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return true
	}
	return reflect.DeepEqual(entity, *new(T))
}

// InvalidCache removes the entity's cached value by ID.
func (s *entityServiceImpl[T]) InvalidCache(ctx context.Context, id ID) {
	cacheKey := s.getCacheKey(id)
	if err := s.cache.Delete(cacheKey); err != nil {
		s.Logger.Error("Failed to remove cache", "err", err)
	}
}

// UpdateCache is a no-op placeholder. The default EntityService does not cache
// entities automatically. Consumers that need caching should either use the
// InvalidCache/cache hooks, embed entityServiceImpl and override this method,
// or implement their own caching strategy in service-layer wrappers.
func (s *entityServiceImpl[T]) UpdateCache(ctx context.Context, entity T) {
}

// WithTransaction executes fn within a database transaction.
// The transaction is stored in the context so that nested service calls
// participate in the same transaction. Nested calls to WithTransaction
// reuse the existing transaction and log a warning.
func (s *entityServiceImpl[T]) WithTransaction(ctx context.Context, fn func(txCtx context.Context, txDb *gorm.DB) error) error {
	if ctx.Value(ctxKeyTx) != nil {
		s.Logger.Warn("nested transaction")
		return fn(ctx, s.GetDB(ctx))
	}
	return s.GetDB(ctx).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, ctxKeyTx, tx)
		return fn(txCtx, tx)
	})
}

// NewAuditLogEventExtra creates an AuditLogEvent populated with actor
// metadata resolved via the configured ContextExtractor, plus the given options.
func (s *entityServiceImpl[T]) NewAuditLogEventExtra(ctx context.Context, opts ...AuditLogEventOption) *AuditLogEvent {
	actor := s.ctxExtr.Extract(ctx)

	auditLog := &AuditLogEvent{
		ActorType:    actor.ActorType,
		ActorID:      actor.ActorID,
		IdentityID:   actor.IdentityID,
		ResourceType: s.GetEntityName(),
		IPAddress:    actor.IPAddress,
		UserAgent:    actor.UserAgent,
	}

	for _, opt := range opts {
		opt(auditLog)
	}
	return auditLog
}

// NewAuditLogEvent creates an AuditLogEvent for a standard CRUD action.
func (s *entityServiceImpl[T]) NewAuditLogEvent(ctx context.Context, action string, resID ID, data map[string]any) *AuditLogEvent {
	return s.NewAuditLogEventExtra(ctx, WithAction(action), WithResourceID(resID), WithDetails(data))
}

// LogAuditEvent logs the audit event to both the structured logger and the
// audit service.
func (s *entityServiceImpl[T]) LogAuditEvent(ctx context.Context, auditLog *AuditLogEvent) {
	s.Logger.Info("Audit log",
		"ActorType", auditLog.ActorType,
		"ActorID", auditLog.ActorID,
		"IdentityID", auditLog.IdentityID,
		"ResourceType", auditLog.ResourceType,
		"ResourceID", auditLog.ResourceID,
		"Action", auditLog.Action,
		"Status", auditLog.Status,
		"Details", auditLog.Details,
		"IPAddress", auditLog.IPAddress,
		"UserAgent", auditLog.UserAgent,
		"ErrorMessage", auditLog.ErrorMessage,
	)
	s.audit.LogEvent(ctx, auditLog)
}

