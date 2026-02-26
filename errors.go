package entigo

import "errors"

var (
	// ErrCacheMiss indicates the requested key was not found in the cache.
	ErrCacheMiss = errors.New("cache miss")

	// ErrEntityNil indicates that a nil entity was passed where a non-nil value is required.
	ErrEntityNil = errors.New("entity is nil")

	// ErrEntityAlreadyDeleted indicates an attempt to delete an entity that has already been soft-deleted.
	ErrEntityAlreadyDeleted = errors.New("entity already deleted")

	// ErrNoRowsDeleted indicates that a delete operation affected zero rows.
	ErrNoRowsDeleted = errors.New("no rows deleted")

	// ErrConcurrentModification indicates an optimistic locking conflict
	// where the entity was modified by another transaction.
	ErrConcurrentModification = errors.New("concurrent modification detected")

	// ErrPermissionDenied indicates that the caller lacks permission for the requested operation.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrFieldNotFound indicates that the specified field does not exist on the entity.
	ErrFieldNotFound = errors.New("field not found")
)
