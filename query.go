package entigo

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// QueryOption is a function that modifies a GORM query. It is the primary
// building block for composing query behavior (pagination, filtering,
// ordering, etc.).
type QueryOption func(*gorm.DB) *gorm.DB

// ---------------------------------------------------------------------------
// Pagination helpers
// ---------------------------------------------------------------------------

// WithPagination creates a QueryOption that applies page-based pagination.
// Pages are 1-indexed: page=1 returns the first `size` records.
func WithPagination(page, size int) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		offset := (page - 1) * size
		return db.Offset(offset).Limit(size)
	}
}

// WithOffsetLimit creates a QueryOption that applies raw offset/limit pagination.
func WithOffsetLimit(offset, limit int) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(offset).Limit(limit)
	}
}

// WithPaginationFrom creates a pagination QueryOption from query parameters.
// Supports "page" and "size" parameters (e.g., ?page=1&size=10).
// Defaults to page=1, size=DefaultPageSize. Size is capped at MaxPageSize.
func WithPaginationFrom(queryMap QueryMap) QueryOption {
	// Get pagination params
	var page, size int
	if pages := queryMap["page"]; len(pages) > 0 {
		page = ParseIntOrDefault(pages[0], 1)
	} else {
		page = 1
	}

	if sizes := queryMap["size"]; len(sizes) > 0 {
		size = ParseIntOrDefault(sizes[0], DefaultPageSize)
	} else {
		size = DefaultPageSize
	}

	// Ensure valid values
	if page < 1 {
		page = 1
	}
	size = min(size, MaxPageSize)

	// Calculate offset
	offset := (page - 1) * size

	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(offset).Limit(size)
	}
}

// ---------------------------------------------------------------------------
// WHERE clause helpers
// ---------------------------------------------------------------------------

// WithWhere creates a QueryOption that adds a WHERE condition with explicit
// condition string and arguments.
func WithWhere(condition string, args ...any) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(condition, args...)
	}
}

// WithWhereArgs creates a QueryOption that adds a WHERE condition using
// variadic arguments. The first argument is the condition (string or struct),
// and the rest are bind parameters.
func WithWhereArgs(args ...any) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		switch len(args) {
		case 0:
			return db
		case 1:
			return db.Where(args[0])
		default:
			return db.Where(args[0], args[1:]...)
		}
	}
}

// WithConditionBuilder creates a QueryOption from a ConditionBuilder.
// If the builder has no conditions, the query is returned unchanged.
func WithConditionBuilder(qb *ConditionBuilder) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		query, args := qb.Build()
		if query != "" {
			return db.Where(query, args...)
		}
		return db
	}
}

// ---------------------------------------------------------------------------
// Filter helpers
// ---------------------------------------------------------------------------

// WithFilter creates a QueryOption from a filter map. Each key is a column
// name, and the value is processed through processFilter to support operator
// prefixes (like:, gt:, in:, etc.). Common pagination and sorting parameters
// are automatically ignored.
func WithFilter(filters map[string]any) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		for field, value := range filters {
			fp := processFilter(field, value)
			if fp.Condition != "" {
				db = db.Where(fp.Condition, fp.Args...)
			}
		}
		return db
	}
}

// WithFilterFrom creates a QueryOption by parsing filter scopes from the type
// parameter T and building filters from the provided query map.
func WithFilterFrom[T any](queryMap QueryMap) QueryOption {
	return WithFilter(BuildFiltersForType[T](queryMap))
}

// ---------------------------------------------------------------------------
// Order helpers
// ---------------------------------------------------------------------------

// WithOrder creates a QueryOption that orders by a single field.
// When desc is true, " DESC" is appended to the field name.
func WithOrder(field string, desc bool) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		if !IsValidColumnName(field) {
			return db
		}
		if desc {
			return db.Order(field + " DESC")
		}
		return db.Order(field)
	}
}

// WithOrderFrom creates a QueryOption for ordering based on query parameters.
// Supports multiple order params with format: field:desc or field:asc.
// Example: ?order=created_at:desc,id:desc
// Falls back to "id DESC" when no order parameters are provided.
func WithOrderFrom(queryMap QueryMap) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		// Get order params, if empty use default order
		orders := queryMap["order"]
		if len(orders) == 0 {
			orders = queryMap["sort"]
		}
		if len(orders) == 0 {
			return WithOrder("id", true)(db)
		}

		// Process each order param
		for _, order := range orders {
			// Split multiple fields
			for _, param := range strings.Split(order, ",") {
				if param = strings.TrimSpace(param); param == "" {
					continue
				}

				// Parse field and direction
				parts := strings.Split(param, ":")
				field := strings.TrimSpace(parts[0])
				if field == "" || !IsValidColumnName(field) {
					continue
				}

				// Set direction, default to ASC
				desc := len(parts) > 1 && strings.ToLower(strings.TrimSpace(parts[1])) == "desc"

				// Apply order
				if desc {
					db = db.Order(field + " DESC")
				} else {
					db = db.Order(field)
				}
			}
		}

		return db
	}
}

// ---------------------------------------------------------------------------
// QueryMap type and methods
// ---------------------------------------------------------------------------

// QueryMap is a multi-valued map typically populated from URL query parameters.
// Keys are parameter names and values are slices of strings supporting
// multi-value parameters.
type QueryMap map[string][]string

// Filter removes all keys from the QueryMap that are not present in the
// provided columns slice. This is useful for restricting filter parameters
// to a known set of allowed columns.
func (m QueryMap) Filter(columns []string) QueryMap {
	for key := range m {
		if !slices.Contains(columns, key) {
			delete(m, key)
		}
	}
	return m
}

// ToMap converts QueryMap to map[string]any.
// Single values are kept as strings.
// Multiple values are converted to "in:value1,value2" format.
func (m QueryMap) ToMap() map[string]any {
	result := make(map[string]any, len(m))

	for key, values := range m {
		if len(values) == 0 {
			continue
		}

		if len(values) == 1 {
			result[key] = values[0]
		} else {
			result[key] = "in:" + strings.Join(values, ",")
		}
	}

	return result
}

// Merge merges params into the QueryMap. If a param value is an array/slice,
// all values are appended. Otherwise, the value is converted to string and
// appended as a single value.
func (m *QueryMap) Merge(params map[string]any) QueryMap {
	if *m == nil {
		*m = make(QueryMap)
	}

	for key, value := range params {
		if value == nil {
			continue
		}

		switch v := value.(type) {
		case []string:
			(*m)[key] = append((*m)[key], v...)

		case []int:
			vals := make([]string, len(v))
			for i, num := range v {
				vals[i] = strconv.Itoa(num)
			}
			(*m)[key] = append((*m)[key], vals...)

		case []any:
			vals := make([]string, len(v))
			for i, item := range v {
				vals[i] = fmt.Sprint(item)
			}
			(*m)[key] = append((*m)[key], vals...)

		default:
			(*m)[key] = append((*m)[key], fmt.Sprint(value))
		}
	}

	return *m
}

