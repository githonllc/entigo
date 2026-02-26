package entigo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConditionBuilderSimple(t *testing.T) {
	cb := NewConditionBuilder().
		And("name = ?", "John")

	query, args := cb.Build()

	assert.Contains(t, query, "name = ?")
	assert.Equal(t, []any{"John"}, args)
}

func TestConditionBuilderMultiple(t *testing.T) {
	cb := NewConditionBuilder().
		And("name = ?", "John").
		And("age > ?", 18)

	query, args := cb.Build()

	assert.Contains(t, query, "name = ?")
	assert.Contains(t, query, "AND")
	assert.Contains(t, query, "age > ?")
	assert.Equal(t, []any{"John", 18}, args)
}

func TestConditionBuilderOr(t *testing.T) {
	cb := NewConditionBuilder().
		And("status = ?", "active").
		Or("status = ?", "pending")

	query, args := cb.Build()

	assert.Contains(t, query, "status = ?")
	assert.Contains(t, query, "OR")
	assert.Equal(t, []any{"active", "pending"}, args)
}

func TestConditionBuilderGroup(t *testing.T) {
	cb := NewConditionBuilder().
		And("active = ?", true).
		GroupStart().
		And("name = ?", "John").
		Or("name = ?", "Jane").
		GroupEnd()

	query, args := cb.Build()

	assert.Contains(t, query, "active = ?")
	assert.Contains(t, query, "AND")
	assert.Contains(t, query, "name = ?")
	assert.Contains(t, query, "OR")
	assert.Equal(t, []any{true, "John", "Jane"}, args)
}

func TestConditionBuilderNested(t *testing.T) {
	cb := NewConditionBuilder().
		GroupStart().
		And("a = ?", 1).
		And("b = ?", 2).
		GroupEnd().
		GroupStart().
		And("c = ?", 3).
		Or("d = ?", 4).
		GroupEnd()

	query, args := cb.Build()

	assert.NotEmpty(t, query)
	assert.Equal(t, []any{1, 2, 3, 4}, args)
}

func TestConditionBuilderEmpty(t *testing.T) {
	cb := NewConditionBuilder()

	query, args := cb.Build()

	assert.Empty(t, query, "empty builder should return empty string")
	assert.Nil(t, args, "empty builder should return nil args")
}

func TestConditionBuilderConditional(t *testing.T) {
	t.Run("condition added when check returns true", func(t *testing.T) {
		cb := NewConditionBuilder().
			ConditionIf(func() bool { return true }, "name = ?", "John")

		query, args := cb.Build()
		assert.Contains(t, query, "name = ?")
		assert.Equal(t, []any{"John"}, args)
	})

	t.Run("condition skipped when check returns false", func(t *testing.T) {
		cb := NewConditionBuilder().
			ConditionIf(func() bool { return false }, "name = ?", "John")

		query, args := cb.Build()
		assert.Empty(t, query)
		assert.Nil(t, args)
	})
}

func TestConditionBuilderAndIf(t *testing.T) {
	t.Run("added when true", func(t *testing.T) {
		cb := NewConditionBuilder().
			And("id > ?", 0).
			AndIf(func() bool { return true }, "status = ?", "active")

		query, args := cb.Build()
		assert.Contains(t, query, "status = ?")
		assert.Equal(t, []any{0, "active"}, args)
	})

	t.Run("skipped when false", func(t *testing.T) {
		cb := NewConditionBuilder().
			And("id > ?", 0).
			AndIf(func() bool { return false }, "status = ?", "active")

		query, args := cb.Build()
		assert.NotContains(t, query, "status = ?")
		assert.Equal(t, []any{0}, args)
	})
}

func TestConditionBuilderOrIf(t *testing.T) {
	t.Run("added when true", func(t *testing.T) {
		cb := NewConditionBuilder().
			And("status = ?", "active").
			OrIf(func() bool { return true }, "status = ?", "pending")

		query, args := cb.Build()
		assert.Contains(t, query, "OR")
		assert.Equal(t, []any{"active", "pending"}, args)
	})

	t.Run("skipped when false", func(t *testing.T) {
		cb := NewConditionBuilder().
			And("status = ?", "active").
			OrIf(func() bool { return false }, "status = ?", "pending")

		query, args := cb.Build()
		assert.NotContains(t, query, "OR")
		assert.Equal(t, []any{"active"}, args)
	})
}

func TestConditionBuilderOrGroupStart(t *testing.T) {
	cb := NewConditionBuilder().
		And("active = ?", true).
		OrGroupStart().
		And("role = ?", "admin").
		And("role = ?", "superadmin").
		GroupEnd()

	query, args := cb.Build()
	assert.Contains(t, query, "active = ?")
	assert.Contains(t, query, "OR")
	assert.Contains(t, query, "role = ?")
	assert.Equal(t, []any{true, "admin", "superadmin"}, args)
}

func TestConditionBuilderHasConditions(t *testing.T) {
	t.Run("empty builder", func(t *testing.T) {
		cb := NewConditionBuilder()
		assert.False(t, cb.HasConditions())
		assert.True(t, cb.IsEmpty())
	})

	t.Run("builder with conditions", func(t *testing.T) {
		cb := NewConditionBuilder().And("x = ?", 1)
		assert.True(t, cb.HasConditions())
		assert.False(t, cb.IsEmpty())
	})
}

func TestConditionBuilderConditionMultipleArgs(t *testing.T) {
	// Condition processes args in pairs: extractQueryAndArgs takes the first
	// arg as the query and ALL remaining args as bind params. Then it advances
	// by 2 positions. This means the first condition receives extra trailing
	// args that get collected by extractQueryAndArgs.
	cb := NewConditionBuilder().
		Condition("a = ?", 1, "b = ?", 2)

	query, args := cb.Build()
	assert.Contains(t, query, "a = ?")
	assert.Contains(t, query, "b = ?")
	// First condition: query="a = ?", args=[1, "b = ?", 2]
	// Second condition: query="b = ?", args=[2]
	assert.Equal(t, []any{1, "b = ?", 2, 2}, args)
}

func TestConditionBuilderEmptyArgs(t *testing.T) {
	// Calling And/Or/Condition with no args should be a no-op
	cb := NewConditionBuilder().
		And().
		Or().
		Condition()

	query, args := cb.Build()
	assert.Empty(t, query)
	assert.Nil(t, args)
}

func TestConditionBuilderGroupEndAtRoot(t *testing.T) {
	// GroupEnd at root level should not panic
	cb := NewConditionBuilder().
		And("x = ?", 1).
		GroupEnd() // Should be a no-op since we are already at root

	query, args := cb.Build()
	assert.Contains(t, query, "x = ?")
	assert.Equal(t, []any{1}, args)
}

func TestNotEmptyHelper(t *testing.T) {
	assert.True(t, NotEmpty("hello")())
	assert.False(t, NotEmpty("")())
}

func TestNotZeroHelper(t *testing.T) {
	assert.True(t, NotZero(42)())
	assert.False(t, NotZero(0)())
}

func TestNotNilHelper(t *testing.T) {
	s := "hello"
	assert.True(t, NotNil(&s)())
	assert.False(t, NotNil(nil)())
}
