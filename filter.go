package entigo

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// filterProcessor processes a single filter condition and returns the SQL
// condition string along with its arguments.
type filterProcessor struct {
	// Condition is the generated SQL WHERE clause fragment (e.g., "name LIKE ?").
	Condition string
	// Args holds the bind parameters for the condition.
	Args []any
}

// ignoredFilterParams defines common query parameters that should be excluded
// from filter processing (pagination, sorting, etc.).
var ignoredFilterParams = map[string]bool{
	"page":     true,
	"size":     true,
	"per_page": true,
	"limit":    true,
	"offset":   true,
	"order":    true,
	"sort":     true,
	"sort_by":  true,
	"sort_dir": true,
}

// processFilter handles a single filter field and value, returning the SQL
// condition and arguments. It supports the following operator prefixes for
// string values:
//
//   - like:     -- case-sensitive LIKE match
//   - ilike:    -- case-insensitive LIKE match
//   - gt:       -- greater than (numeric)
//   - ge:       -- greater than or equal (numeric)
//   - lt:       -- less than (numeric)
//   - le:       -- less than or equal (numeric)
//   - ne:       -- not equal (numeric)
//   - from:     -- greater than or equal (RFC3339 time)
//   - to:       -- less than or equal (RFC3339 time)
//   - between:  -- BETWEEN two RFC3339 timestamps separated by comma
//   - in:       -- IN clause with comma-separated values
//   - json:     -- JSONB query (exact match or LIKE)
//   - null:     -- IS NULL check
//   - not_null: -- IS NOT NULL check
//
// For non-string values, slices produce an IN clause, and all other types
// produce an equality check.
func processFilter(field string, value any) *filterProcessor {
	fp := &filterProcessor{}

	// Skip if field is a common query parameter, or null value
	if ignoredFilterParams[field] || value == nil {
		return fp
	}

	// Skip invalid column names to prevent SQL injection
	if !IsValidColumnName(field) {
		return fp
	}

	// Handle string type filters
	if strVal, ok := value.(string); ok {
		if strVal == "" {
			return fp
		}

		switch {
		case strings.HasPrefix(strVal, "ilike:"):
			pattern := strings.TrimPrefix(strVal, "ilike:")
			fp.Condition = fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", field)
			fp.Args = []any{"%" + pattern + "%"}

		case strings.HasPrefix(strVal, "like:"):
			pattern := strings.TrimPrefix(strVal, "like:")
			fp.Condition = field + " LIKE ?"
			fp.Args = []any{"%" + pattern + "%"}

		case strings.HasPrefix(strVal, "gt:"):
			numStr := strings.TrimPrefix(strVal, "gt:")
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				fp.Condition = field + " > ?"
				fp.Args = []any{num}
			}

		case strings.HasPrefix(strVal, "ge:"):
			numStr := strings.TrimPrefix(strVal, "ge:")
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				fp.Condition = field + " >= ?"
				fp.Args = []any{num}
			}

		case strings.HasPrefix(strVal, "lt:"):
			numStr := strings.TrimPrefix(strVal, "lt:")
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				fp.Condition = field + " < ?"
				fp.Args = []any{num}
			}

		case strings.HasPrefix(strVal, "le:"):
			numStr := strings.TrimPrefix(strVal, "le:")
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				fp.Condition = field + " <= ?"
				fp.Args = []any{num}
			}

		case strings.HasPrefix(strVal, "ne:"):
			numStr := strings.TrimPrefix(strVal, "ne:")
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				fp.Condition = field + " <> ?"
				fp.Args = []any{num}
			}

		case strings.HasPrefix(strVal, "from:"):
			timeStr := strings.TrimPrefix(strVal, "from:")
			if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
				fp.Condition = field + " >= ?"
				fp.Args = []any{t}
			}

		case strings.HasPrefix(strVal, "to:"):
			timeStr := strings.TrimPrefix(strVal, "to:")
			if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
				fp.Condition = field + " <= ?"
				fp.Args = []any{t}
			}

		case strings.HasPrefix(strVal, "between:"):
			// Split time range string
			parts := strings.Split(strings.TrimPrefix(strVal, "between:"), ",")
			if len(parts) != 2 {
				return fp
			}

			// Parse both times
			from, to := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			fromTime, err1 := time.Parse(time.RFC3339, from)
			toTime, err2 := time.Parse(time.RFC3339, to)

			// Check if both times are valid
			if err1 != nil || err2 != nil {
				return fp
			}

			// Ensure the time range is valid (from <= to)
			if fromTime.After(toTime) {
				fromTime, toTime = toTime, fromTime // swap to ensure valid range
			}

			fp.Condition = field + " BETWEEN ? AND ?"
			fp.Args = []any{fromTime, toTime}

		case strings.HasPrefix(strVal, "in:"):
			items := strings.Split(strings.TrimPrefix(strVal, "in:"), ",")
			validItems := make([]string, 0, len(items))
			for _, item := range items {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					validItems = append(validItems, trimmed)
				}
			}
			if len(validItems) > 0 {
				fp.Condition = field + " IN (?)"
				fp.Args = []any{validItems}
			}

		case strings.HasPrefix(strVal, "json:"):
			// JSONB query with support for LIKE operations.
			// Format:
			// - Like match:  json:key~value      -> field->>key ILIKE '%value%'
			// - Path like:   json:key->path~value -> field#>'{key,path}' ILIKE '%value%'
			// - Exact match: json:path=value      -> field @> '{"path":"value"}'
			// - Path match:  json:key->path=value -> field#>>'{key,path}' = value
			jsonCond := strings.TrimPrefix(strVal, "json:")

			if strings.Contains(jsonCond, "~") {
				parts := strings.Split(jsonCond, "~")
				if len(parts) != 2 {
					return fp
				}
				path := strings.TrimSpace(parts[0])
				matchValue := strings.TrimSpace(parts[1])

				if strings.Contains(path, "->") {
					// Path LIKE query: convert jsonb path result to text
					jsonPath := strings.Split(path, "->")
					fp.Condition = fmt.Sprintf("(%s#>?)::text ILIKE ?", field)
					fp.Args = []any{jsonPath, "%" + matchValue + "%"}
				} else {
					// Simple field LIKE query: use ->> to get text
					fp.Condition = fmt.Sprintf("%s->>? ILIKE ?", field)
					fp.Args = []any{path, "%" + matchValue + "%"}
				}
				return fp
			}

			// Handle exact match
			if strings.Contains(jsonCond, "=") {
				parts := strings.Split(jsonCond, "=")
				if len(parts) != 2 {
					return fp
				}
				path := strings.TrimSpace(parts[0])
				matchValue := strings.TrimSpace(parts[1])

				if strings.Contains(path, "->") {
					jsonPath := strings.Split(path, "->")
					fp.Condition = fmt.Sprintf("%s#>>? = ?", field)
					fp.Args = []any{jsonPath, matchValue}
				} else {
					fp.Condition = fmt.Sprintf("%s @> ?", field)
					fp.Args = []any{fmt.Sprintf(`{"%s":"%s"}`, path, matchValue)}
				}
				return fp
			}

		case strings.HasPrefix(strVal, "null:"):
			fp.Condition = field + " IS NULL"
			fp.Args = nil

		case strings.HasPrefix(strVal, "not_null:"):
			fp.Condition = field + " IS NOT NULL"
			fp.Args = nil

		default:
			fp.Condition = field + " = ?"
			fp.Args = []any{strVal}
		}
		return fp
	}

	// Handle slice types for IN queries
	switch v := value.(type) {
	case []string, []int, []int64, []any:
		fp.Condition = field + " IN (?)"
		fp.Args = []any{v}
	default:
		fp.Condition = field + " = ?"
		fp.Args = []any{value}
	}

	return fp
}
