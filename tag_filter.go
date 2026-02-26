package entigo

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// FilterScope describes a filterable field extracted from struct tags.
type FilterScope struct {
	Field      string       // struct field name
	QueryName  string       // query parameter name (from tag or JSON name)
	ColumnName string       // database column name (from tag or JSON name)
	Type       reflect.Type // field type for value parsing
}

// Compiled regex patterns for parsing ent tag scope definitions.
//
// Example tag format:
//
//	ent:"scope=filter(name=sn,col=serial_no);sort(name=sn_sort)"
var (
	scopeRegex  = regexp.MustCompile(`(\w+)(?:\((.*?)\))?`)
	optionRegex = regexp.MustCompile(`(\w+)=([^,]+)`)
)

// parseFilterOptions parses filter options from an options string.
// Example: "name=user_id,col=id" => {"name": "user_id", "col": "id"}
func parseFilterOptions(optStr string) map[string]string {
	if optStr == "" {
		return make(map[string]string) // return empty map instead of nil for consistency
	}

	// Pre-allocate map with estimated size
	options := make(map[string]string, strings.Count(optStr, ",")+1)

	matches := optionRegex.FindAllStringSubmatch(optStr, -1)
	for _, match := range matches {
		if len(match) == 3 {
			options[strings.TrimSpace(match[1])] = strings.TrimSpace(match[2])
		}
	}
	return options
}

// ParseFilterScopes parses all filter scopes from a struct type, including
// nested/embedded structs. It inspects ent struct tags to find fields with
// the "filter" scope and extracts query and column name mappings.
func ParseFilterScopes(t reflect.Type) []FilterScope {
	// Pre-allocate slice with a reasonable initial capacity
	results := make([]FilterScope, 0, t.NumField())
	return parseStructFields(t, results)
}

// parseStructFields handles the actual parsing of struct fields, recursing
// into embedded structs.
func parseStructFields(t reflect.Type, results []FilterScope) []FilterScope {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded/nested structs
		if field.Anonymous {
			results = parseStructFields(field.Type, results)
			continue
		}

		// Parse filter scope if present
		if scope := parseFieldFilter(field); scope != nil {
			results = append(results, *scope)
		}
	}
	return results
}

// parseFieldFilter extracts a filter scope from a single struct field.
// Returns nil if the field does not have a filter scope defined.
func parseFieldFilter(field reflect.StructField) *FilterScope {
	entTag := field.Tag.Get("ent")
	if entTag == "" {
		return nil
	}

	// Parse scope definitions
	for _, part := range strings.Split(entTag, ";") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "scope=") {
			continue
		}

		scopeDef := strings.TrimPrefix(part, "scope=")
		matches := scopeRegex.FindAllStringSubmatch(scopeDef, -1)
		for _, match := range matches {
			if len(match) > 1 && match[1] == "filter" {
				var filterOpts map[string]string
				if len(match) > 2 {
					filterOpts = parseFilterOptions(match[2])
				}

				defaultName := GetJSONName(field)
				return &FilterScope{
					Field:      field.Name,
					Type:       field.Type,
					QueryName:  GetOrDefault(filterOpts["name"], defaultName),
					ColumnName: GetOrDefault(filterOpts["col"], defaultName),
				}
			}
		}
	}
	return nil
}

// BuildFilterConditions builds query conditions from query parameters based on
// filter scopes. It returns a ConditionBuilder that can be used with
// WithConditionBuilder to apply the conditions to a GORM query.
//
// For multi-value parameters, an IN clause is generated.
// For single string values, wildcards (* and ?) are converted to SQL LIKE patterns.
// Boolean, integer, unsigned integer, and float types are parsed and matched exactly.
func BuildFilterConditions(params QueryMap, scopes []FilterScope) *ConditionBuilder {
	qb := NewConditionBuilder()

	for _, scope := range scopes {
		// Defense-in-depth: validate column name even though it comes from struct tags
		if !IsValidColumnName(scope.ColumnName) {
			continue
		}

		values, exists := params[scope.QueryName]
		if !exists || len(values) == 0 {
			continue
		}

		// Handle multi-value case
		if len(values) > 1 {
			qb.And(fmt.Sprintf("%s IN (?)", scope.ColumnName), values)
			continue
		}

		// Handle single value case
		value := values[0]
		if value == "" {
			continue
		}

		switch scope.Type.Kind() {
		case reflect.Bool:
			if value == "true" {
				qb.And(fmt.Sprintf("%s = ?", scope.ColumnName), true)
			} else if value == "false" {
				qb.And(fmt.Sprintf("%s = ?", scope.ColumnName), false)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				qb.And(fmt.Sprintf("%s = ?", scope.ColumnName), v)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if v, err := strconv.ParseUint(value, 10, 64); err == nil {
				qb.And(fmt.Sprintf("%s = ?", scope.ColumnName), v)
			}
		case reflect.Float32, reflect.Float64:
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				qb.And(fmt.Sprintf("%s = ?", scope.ColumnName), v)
			}
		case reflect.String:
			// Split by comma for potential multiple values
			splitValues := strings.Split(value, ",")
			if len(splitValues) > 1 {
				// Filter non-empty values
				nonEmptyValues := make([]string, 0, len(splitValues))
				for _, v := range splitValues {
					if trimmed := strings.TrimSpace(v); trimmed != "" {
						nonEmptyValues = append(nonEmptyValues, trimmed)
					}
				}

				if len(nonEmptyValues) > 0 {
					// Create multiple ? placeholders
					placeholders := make([]string, len(nonEmptyValues))
					args := make([]any, len(nonEmptyValues)+1)
					for i := range nonEmptyValues {
						placeholders[i] = "?"
						args[i+1] = nonEmptyValues[i]
					}
					// First arg is the query string with multiple ?
					args[0] = fmt.Sprintf("%s IN (%s)", scope.ColumnName, strings.Join(placeholders, ","))
					qb.And(args...)
					continue
				}
			}

			// Handle single value with wildcards
			if strings.Contains(value, "*") || strings.Contains(value, "?") {
				value = strings.ReplaceAll(value, "*", "%")
				value = strings.ReplaceAll(value, "?", "_")
				qb.And(fmt.Sprintf("%s LIKE ?", scope.ColumnName), value)
			} else {
				qb.And(fmt.Sprintf("%s = ?", scope.ColumnName), value)
			}
		default:
			// Unsupported types are silently skipped
		}
	}

	return qb
}

// ParseAndBuildFilters combines parsing filter scopes from a type and building
// conditions from query parameters in a single call.
func ParseAndBuildFilters[T any](params QueryMap) *ConditionBuilder {
	var model T
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	scopes := ParseFilterScopes(t)
	return BuildFilterConditions(params, scopes)
}

// BuildFilters converts a QueryMap to a filter map based on FilterScopes.
// The returned map uses column names as keys and can be used with WithFilter
// or SQLBuilder.ApplyFilter.
func BuildFilters(queryMap QueryMap, scopes []FilterScope) map[string]any {
	// Create a lookup map for faster scope searching
	scopeMap := make(map[string]FilterScope, len(scopes))
	for _, scope := range scopes {
		scopeMap[scope.QueryName] = scope
	}

	filters := make(map[string]any)
	for paramName, values := range queryMap {
		// Skip empty values
		if len(values) == 0 {
			continue
		}

		// Find corresponding scope
		scope, exists := scopeMap[paramName]
		if !exists {
			continue
		}

		// Handle multi-values
		if len(values) > 1 {
			filters[scope.ColumnName] = "in:" + strings.Join(values, ",")
			continue
		}

		// Single value
		filters[scope.ColumnName] = values[0]
	}

	return filters
}

// BuildFiltersForType builds a filter map for a given struct type using its
// ent tag filter scopes and the provided query parameters.
func BuildFiltersForType[T any](params QueryMap) map[string]any {
	var model T
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	scopes := ParseFilterScopes(t)
	return BuildFilters(params, scopes)
}
