package entigo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBaseEntityInit(t *testing.T) {
	resetIDGenerator()

	e := &BaseEntity{}
	e.Init()

	assert.True(t, e.ID > 0, "Init should set a valid positive ID")
	assert.False(t, e.CreatedAt.IsZero(), "Init should set CreatedAt")
	assert.False(t, e.UpdatedAt.IsZero(), "Init should set UpdatedAt")
	assert.Equal(t, 0, e.Revision, "Init should set Revision to 0")

	// CreatedAt and UpdatedAt should be in UTC
	assert.Equal(t, time.UTC, e.CreatedAt.Location(), "CreatedAt should be UTC")
	assert.Equal(t, time.UTC, e.UpdatedAt.Location(), "UpdatedAt should be UTC")
}

func TestBaseEntityInitPreservesExistingID(t *testing.T) {
	resetIDGenerator()

	existingID := NewID()
	e := &BaseEntity{}
	e.SetID(existingID)
	e.Init()

	// Init should not overwrite a valid existing ID
	assert.Equal(t, existingID, e.GetID(), "Init should preserve an existing valid ID")
}

func TestBaseEntityGenerateID(t *testing.T) {
	resetIDGenerator()

	e := &BaseEntity{}
	id := e.GenerateID()

	assert.True(t, id > 0, "GenerateID should return a valid positive ID")

	// Generate another ID and verify uniqueness
	id2 := e.GenerateID()
	assert.NotEqual(t, id, id2, "GenerateID should return unique IDs")
}

func TestBaseEntitySoftDelete(t *testing.T) {
	e := &BaseEntity{}
	assert.False(t, e.IsDeleted(), "new entity should not be deleted")
	assert.Nil(t, e.GetDeletedAt(), "new entity should have nil DeletedAt")

	e.SoftDelete()

	assert.True(t, e.IsDeleted(), "entity should be marked as deleted after SoftDelete")
	assert.NotNil(t, e.GetDeletedAt(), "DeletedAt should be set after SoftDelete")
	assert.True(t, e.DeletedAt.Valid, "DeletedAt.Valid should be true after SoftDelete")
}

func TestBaseEntityRestore(t *testing.T) {
	e := &BaseEntity{}

	// First soft delete, then restore
	e.SoftDelete()
	assert.True(t, e.IsDeleted(), "entity should be deleted before restore")

	e.Restore()
	assert.False(t, e.IsDeleted(), "entity should not be deleted after Restore")
	assert.Nil(t, e.GetDeletedAt(), "DeletedAt should be nil after Restore")
	assert.False(t, e.DeletedAt.Valid, "DeletedAt.Valid should be false after Restore")
}

func TestBaseEntityRevision(t *testing.T) {
	e := &BaseEntity{}

	assert.Equal(t, 0, e.GetRevision(), "initial revision should be 0")

	e.IncrementRevision()
	assert.Equal(t, 1, e.GetRevision(), "revision should be 1 after first increment")

	e.IncrementRevision()
	assert.Equal(t, 2, e.GetRevision(), "revision should be 2 after second increment")
}

func TestBaseEntityAccessors(t *testing.T) {
	resetIDGenerator()

	e := &BaseEntity{}

	// Test SetID / GetID
	testID := NewID()
	e.SetID(testID)
	assert.Equal(t, testID, e.GetID())

	// Test SetCreatedAt / GetCreatedAt
	now := time.Now().UTC()
	e.SetCreatedAt(now)
	assert.Equal(t, now, e.GetCreatedAt())

	// Test SetUpdatedAt / GetUpdatedAt
	later := now.Add(time.Hour)
	e.SetUpdatedAt(later)
	assert.Equal(t, later, e.GetUpdatedAt())
}

func TestBaseEntityGetEntityName(t *testing.T) {
	e := &BaseEntity{}
	name := e.GetEntityName()
	assert.Contains(t, name, "BaseEntity", "entity name should contain 'BaseEntity'")
}

func TestBaseEntityNewInstance(t *testing.T) {
	e := &BaseEntity{}
	inst := e.NewInstance()

	assert.NotNil(t, inst, "NewInstance should return a non-nil Entity")
	_, ok := inst.(*BaseEntity)
	assert.True(t, ok, "NewInstance should return a *BaseEntity")
}

func TestBaseEntityValidate(t *testing.T) {
	e := &BaseEntity{}
	err := e.Validate()
	assert.NoError(t, err, "default Validate should return nil")
}

func TestCopy(t *testing.T) {
	type Simple struct {
		Name  string
		Value int
	}

	// Test successful copy
	t.Run("successful copy", func(t *testing.T) {
		src := &Simple{Name: "test", Value: 42}
		dst := &Simple{}

		err := Copy(src, dst)
		assert.NoError(t, err)
		assert.Equal(t, src.Name, dst.Name)
		assert.Equal(t, src.Value, dst.Value)
	})

	// Test copy with nil destination
	t.Run("copy to nil pointer", func(t *testing.T) {
		src := &Simple{Name: "test", Value: 42}
		var dst *Simple
		// copier handles nil destination gracefully
		err := Copy(src, dst)
		assert.Error(t, err)
	})
}

func TestIsNil(t *testing.T) {
	// Test with nil interface
	t.Run("nil interface", func(t *testing.T) {
		assert.True(t, IsNil(nil), "nil should be detected as nil")
	})

	// Test with valid pointer
	t.Run("valid pointer", func(t *testing.T) {
		s := "hello"
		assert.False(t, IsNil(&s), "non-nil pointer should not be nil")
	})

	// Test with nil pointer
	t.Run("nil pointer", func(t *testing.T) {
		var p *string
		assert.True(t, IsNil(p), "nil pointer should be detected as nil")
	})

	// Test with nil pointer passed as interface
	t.Run("nil pointer as interface", func(t *testing.T) {
		var e *BaseEntity
		assert.True(t, IsNil(e), "nil *BaseEntity should be detected as nil")
	})

	// Test with non-nil pointer passed as interface
	t.Run("non-nil pointer as interface", func(t *testing.T) {
		e := &BaseEntity{}
		assert.False(t, IsNil(e), "non-nil *BaseEntity should not be nil")
	})
}

func TestNewInstance(t *testing.T) {
	// Test with pointer type
	t.Run("pointer type", func(t *testing.T) {
		inst := NewInstance[*BaseEntity]()
		assert.NotNil(t, inst, "NewInstance should return a non-nil pointer")
		assert.Equal(t, ID(0), inst.ID, "new instance should have zero ID")
	})

	// Test with value type
	t.Run("value type", func(t *testing.T) {
		inst := NewInstance[BaseEntity]()
		assert.Equal(t, ID(0), inst.ID, "new instance should have zero ID")
		assert.True(t, inst.CreatedAt.IsZero(), "new instance should have zero CreatedAt")
	})

	// Test with simple type
	t.Run("simple struct type", func(t *testing.T) {
		type Simple struct {
			Name string
			Age  int
		}
		inst := NewInstance[Simple]()
		assert.Equal(t, "", inst.Name)
		assert.Equal(t, 0, inst.Age)
	})

	// Test with pointer to simple type
	t.Run("pointer to simple struct", func(t *testing.T) {
		type Simple struct {
			Name string
			Age  int
		}
		inst := NewInstance[*Simple]()
		assert.NotNil(t, inst)
		assert.Equal(t, "", inst.Name)
		assert.Equal(t, 0, inst.Age)
	})
}
