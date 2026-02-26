package entigo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProcessFilterLike(t *testing.T) {
	fp := processFilter("name", "like:john")

	assert.Equal(t, "name LIKE ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, "%john%", fp.Args[0])
}

func TestProcessFilterILike(t *testing.T) {
	fp := processFilter("name", "ilike:John")

	assert.Equal(t, "LOWER(name) LIKE LOWER(?)", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, "%John%", fp.Args[0])
}

func TestProcessFilterGt(t *testing.T) {
	fp := processFilter("age", "gt:18")

	assert.Equal(t, "age > ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, float64(18), fp.Args[0])
}

func TestProcessFilterGe(t *testing.T) {
	fp := processFilter("score", "ge:90")

	assert.Equal(t, "score >= ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, float64(90), fp.Args[0])
}

func TestProcessFilterLt(t *testing.T) {
	fp := processFilter("price", "lt:100")

	assert.Equal(t, "price < ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, float64(100), fp.Args[0])
}

func TestProcessFilterLe(t *testing.T) {
	fp := processFilter("quantity", "le:50")

	assert.Equal(t, "quantity <= ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, float64(50), fp.Args[0])
}

func TestProcessFilterNe(t *testing.T) {
	fp := processFilter("status", "ne:0")

	assert.Equal(t, "status <> ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, float64(0), fp.Args[0])
}

func TestProcessFilterIn(t *testing.T) {
	fp := processFilter("status", "in:active,inactive,pending")

	assert.Equal(t, "status IN (?)", fp.Condition)
	assert.Len(t, fp.Args, 1)
	// The args should be a slice of valid items
	items, ok := fp.Args[0].([]string)
	assert.True(t, ok, "IN clause args should be a []string")
	assert.Equal(t, []string{"active", "inactive", "pending"}, items)
}

func TestProcessFilterInWithSpaces(t *testing.T) {
	fp := processFilter("category", "in: a , b , c ")

	assert.Equal(t, "category IN (?)", fp.Condition)
	items, ok := fp.Args[0].([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"a", "b", "c"}, items)
}

func TestProcessFilterBetween(t *testing.T) {
	from := "2024-01-01T00:00:00Z"
	to := "2024-12-31T23:59:59Z"
	fp := processFilter("created_at", "between:"+from+","+to)

	assert.Equal(t, "created_at BETWEEN ? AND ?", fp.Condition)
	assert.Len(t, fp.Args, 2)

	fromTime, _ := time.Parse(time.RFC3339, from)
	toTime, _ := time.Parse(time.RFC3339, to)
	assert.Equal(t, fromTime, fp.Args[0])
	assert.Equal(t, toTime, fp.Args[1])
}

func TestProcessFilterBetweenSwapsInvalidRange(t *testing.T) {
	// When from > to, the processor should swap them
	from := "2024-12-31T23:59:59Z"
	to := "2024-01-01T00:00:00Z"
	fp := processFilter("created_at", "between:"+from+","+to)

	assert.Equal(t, "created_at BETWEEN ? AND ?", fp.Condition)
	assert.Len(t, fp.Args, 2)

	// After swap, the earlier time should be first
	fromTime, _ := time.Parse(time.RFC3339, to)
	toTime, _ := time.Parse(time.RFC3339, from)
	assert.Equal(t, fromTime, fp.Args[0])
	assert.Equal(t, toTime, fp.Args[1])
}

func TestProcessFilterBetweenInvalid(t *testing.T) {
	// Invalid number of parts
	fp := processFilter("created_at", "between:2024-01-01T00:00:00Z")
	assert.Empty(t, fp.Condition)
	assert.Nil(t, fp.Args)

	// Invalid time format
	fp = processFilter("created_at", "between:invalid,also_invalid")
	assert.Empty(t, fp.Condition)
	assert.Nil(t, fp.Args)
}

func TestProcessFilterFrom(t *testing.T) {
	fp := processFilter("created_at", "from:2024-01-01T00:00:00Z")

	assert.Equal(t, "created_at >= ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	expectedTime, _ := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
	assert.Equal(t, expectedTime, fp.Args[0])
}

func TestProcessFilterTo(t *testing.T) {
	fp := processFilter("created_at", "to:2024-12-31T23:59:59Z")

	assert.Equal(t, "created_at <= ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	expectedTime, _ := time.Parse(time.RFC3339, "2024-12-31T23:59:59Z")
	assert.Equal(t, expectedTime, fp.Args[0])
}

func TestProcessFilterNull(t *testing.T) {
	fp := processFilter("deleted_at", "null:")

	assert.Equal(t, "deleted_at IS NULL", fp.Condition)
	assert.Nil(t, fp.Args)
}

func TestProcessFilterNotNull(t *testing.T) {
	fp := processFilter("email", "not_null:")

	assert.Equal(t, "email IS NOT NULL", fp.Condition)
	assert.Nil(t, fp.Args)
}

func TestProcessFilterExact(t *testing.T) {
	fp := processFilter("status", "active")

	assert.Equal(t, "status = ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, "active", fp.Args[0])
}

func TestProcessFilterSlice(t *testing.T) {
	t.Run("string slice", func(t *testing.T) {
		fp := processFilter("status", []string{"active", "inactive"})
		assert.Equal(t, "status IN (?)", fp.Condition)
		assert.Len(t, fp.Args, 1)
		assert.Equal(t, []string{"active", "inactive"}, fp.Args[0])
	})

	t.Run("int slice", func(t *testing.T) {
		fp := processFilter("level", []int{1, 2, 3})
		assert.Equal(t, "level IN (?)", fp.Condition)
		assert.Len(t, fp.Args, 1)
		assert.Equal(t, []int{1, 2, 3}, fp.Args[0])
	})

	t.Run("int64 slice", func(t *testing.T) {
		fp := processFilter("id", []int64{100, 200, 300})
		assert.Equal(t, "id IN (?)", fp.Condition)
		assert.Len(t, fp.Args, 1)
		assert.Equal(t, []int64{100, 200, 300}, fp.Args[0])
	})

	t.Run("any slice", func(t *testing.T) {
		fp := processFilter("mixed", []any{"a", 1})
		assert.Equal(t, "mixed IN (?)", fp.Condition)
		assert.Len(t, fp.Args, 1)
	})
}

func TestProcessFilterNonStringNonSlice(t *testing.T) {
	// Non-string, non-slice types produce equality check
	fp := processFilter("age", 25)
	assert.Equal(t, "age = ?", fp.Condition)
	assert.Len(t, fp.Args, 1)
	assert.Equal(t, 25, fp.Args[0])
}

func TestProcessFilterSkipsIgnoredParams(t *testing.T) {
	for param := range ignoredFilterParams {
		fp := processFilter(param, "some_value")
		assert.Empty(t, fp.Condition, "ignored param %q should produce empty condition", param)
	}
}

func TestProcessFilterSkipsNilValue(t *testing.T) {
	fp := processFilter("name", nil)
	assert.Empty(t, fp.Condition)
	assert.Nil(t, fp.Args)
}

func TestProcessFilterSkipsEmptyString(t *testing.T) {
	fp := processFilter("name", "")
	assert.Empty(t, fp.Condition)
	assert.Nil(t, fp.Args)
}

func TestProcessFilterInvalidNumeric(t *testing.T) {
	// Invalid numeric value for gt: should produce empty condition
	fp := processFilter("age", "gt:not_a_number")
	assert.Empty(t, fp.Condition)
	assert.Nil(t, fp.Args)
}
