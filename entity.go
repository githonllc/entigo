package entigo

import (
	"fmt"
	"reflect"
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

// Entity defines the interface for all database entities.
// It provides lifecycle hooks, soft delete support, optimistic locking,
// and standard accessor methods for common fields.
type Entity interface {
	NewInstance() Entity
	GenerateID() ID

	GetEntityName() string

	GetID() ID
	SetID(id ID)
	GetCreatedAt() time.Time
	SetCreatedAt(t time.Time)
	GetUpdatedAt() time.Time
	SetUpdatedAt(t time.Time)
	GetDeletedAt() *time.Time

	IsDeleted() bool
	SoftDelete()
	Restore()

	Validate() error

	BeforeCreate(tx *gorm.DB) error
	AfterCreate(tx *gorm.DB) error
	BeforeUpdate(tx *gorm.DB) error
	AfterUpdate(tx *gorm.DB) error
	BeforeDelete(tx *gorm.DB) error
	AfterDelete(tx *gorm.DB) error
	AfterFind(tx *gorm.DB) error

	GetRevision() int
	IncrementRevision()
}

// BaseEntity provides a default implementation of the Entity interface.
// Embed this struct in domain models to get standard fields and behavior.
type BaseEntity struct {
	ID        ID             `json:"id" gorm:"primaryKey;<-:create" ent:"scope=response,filter"`
	CreatedAt time.Time      `json:"created_at" gorm:"<-:create" ent:"scope=response,filter"`
	UpdatedAt time.Time      `json:"updated_at" ent:"scope=response,filter"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index;<-:update"`
	Revision  int            `json:"-" gorm:"default:0"`
}

// NewInstance creates a new BaseEntity instance.
func (e *BaseEntity) NewInstance() Entity {
	return &BaseEntity{}
}

// GenerateID generates a new unique ID using the default ID generator.
func (e *BaseEntity) GenerateID() ID {
	return NewID()
}

// GetEntityName returns the fully-qualified type name of the entity.
func (e *BaseEntity) GetEntityName() string {
	return fmt.Sprintf("%T", e)
}

// GetID returns the entity ID.
func (e *BaseEntity) GetID() ID {
	return e.ID
}

// SetID sets the entity ID.
func (e *BaseEntity) SetID(id ID) {
	e.ID = id
}

// GetCreatedAt returns the creation timestamp.
func (e *BaseEntity) GetCreatedAt() time.Time {
	return e.CreatedAt
}

// SetCreatedAt sets the creation timestamp.
func (e *BaseEntity) SetCreatedAt(t time.Time) {
	e.CreatedAt = t
}

// GetUpdatedAt returns the last update timestamp.
func (e *BaseEntity) GetUpdatedAt() time.Time {
	return e.UpdatedAt
}

// SetUpdatedAt sets the last update timestamp.
func (e *BaseEntity) SetUpdatedAt(t time.Time) {
	e.UpdatedAt = t
}

// GetDeletedAt returns the soft-delete timestamp, or nil if not deleted.
func (e *BaseEntity) GetDeletedAt() *time.Time {
	if e.DeletedAt.Valid {
		return &e.DeletedAt.Time
	}
	return nil
}

// IsDeleted returns true if the entity has been soft-deleted.
func (e *BaseEntity) IsDeleted() bool {
	return e.DeletedAt.Valid
}

// SoftDelete marks the entity as soft-deleted with the current UTC time.
func (e *BaseEntity) SoftDelete() {
	e.DeletedAt = gorm.DeletedAt{Time: time.Now().UTC(), Valid: true}
}

// Restore clears the soft-delete marker, restoring the entity.
func (e *BaseEntity) Restore() {
	e.DeletedAt = gorm.DeletedAt{}
}

// Validate performs entity validation. Override in embedding structs for custom logic.
func (e *BaseEntity) Validate() error {
	return nil
}

// Init initializes the entity with a new ID and timestamps.
// Called automatically by BeforeCreate if no valid ID is set.
func (e *BaseEntity) Init() {
	if IsInvalidID(e.GetID()) {
		e.SetID(NewID())
	}
	now := time.Now().UTC()
	e.CreatedAt = now
	e.UpdatedAt = now
	e.Revision = 0
}

// BeforeCreate is a GORM hook called before inserting a new record.
// It initializes the entity if no valid ID is set and runs validation.
func (e *BaseEntity) BeforeCreate(tx *gorm.DB) error {
	if IsInvalidID(e.ID) {
		e.Init()
	}
	if err := e.Validate(); err != nil {
		return err
	}
	return nil
}

// AfterCreate is a GORM hook called after inserting a new record.
func (e *BaseEntity) AfterCreate(tx *gorm.DB) error {
	return nil
}

// BeforeUpdate is a GORM hook called before updating a record.
// It updates the timestamp and runs validation.
func (e *BaseEntity) BeforeUpdate(tx *gorm.DB) error {
	e.UpdatedAt = time.Now().UTC()
	if err := e.Validate(); err != nil {
		return err
	}
	return nil
}

// AfterUpdate is a GORM hook called after updating a record.
func (e *BaseEntity) AfterUpdate(tx *gorm.DB) error {
	return nil
}

// BeforeDelete is a GORM hook called before deleting a record.
func (e *BaseEntity) BeforeDelete(tx *gorm.DB) error {
	return nil
}

// AfterDelete is a GORM hook called after deleting a record.
func (e *BaseEntity) AfterDelete(tx *gorm.DB) error {
	return nil
}

// AfterFind is a GORM hook called after querying a record.
func (e *BaseEntity) AfterFind(tx *gorm.DB) error {
	return nil
}

// GetRevision returns the current revision number for optimistic locking.
func (e *BaseEntity) GetRevision() int {
	return e.Revision
}

// IncrementRevision increments the revision number.
func (e *BaseEntity) IncrementRevision() {
	e.Revision++
}

// Copy copies fields from src to dst using the copier library.
// Returns an error instead of panicking on failure.
func Copy(from, to any) error {
	return copier.Copy(to, from)
}

// IsNil checks whether the given value is nil or a nil-valued nillable type
// (pointer, map, slice, channel, function, or interface).
func IsNil(i any) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return v.IsNil()
	}
	return false
}

// NewInstance creates a new instance of the generic type T.
// If T is a pointer type, it allocates and returns a pointer to a new zero value.
// If T is a value type, it returns a new zero value.
func NewInstance[T any]() T {
	var instance T

	// Get the type of T
	tType := reflect.TypeOf(instance)

	// Determine whether T is a pointer type
	if tType.Kind() == reflect.Ptr {
		// Get the element type pointed to by the pointer
		elemType := tType.Elem()

		// Create an instance of this type
		newValue := reflect.New(elemType)

		// Convert reflect.Value to T
		return newValue.Interface().(T)
	}

	// If T is not a pointer type, create an instance directly
	newValue := reflect.New(tType).Elem()

	return newValue.Interface().(T)
}
