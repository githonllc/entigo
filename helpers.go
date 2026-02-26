package entigo

import (
	"log/slog"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// validColumnNameRegex matches standard SQL identifiers: starts with letter or underscore,
// followed by letters, digits, or underscores.
var validColumnNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// IsValidColumnName checks whether a string is a safe SQL column/field name.
// Prevents SQL injection via field name interpolation.
func IsValidColumnName(name string) bool {
	return name != "" && validColumnNameRegex.MatchString(name)
}

// M is a convenient alias for map[string]any, commonly used for
// passing dynamic key-value data such as GORM update maps or JSON payloads.
type M = map[string]any

// ToSnakeCase converts PascalCase to snake_case, handling acronyms correctly.
// e.g. "APIKey" -> "api_key", "ProofOfDelivery" -> "proof_of_delivery".
func ToSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := rune(s[i-1])
				if prev >= 'a' && prev <= 'z' {
					// Transition from lowercase to uppercase: "Of" -> "_o"
					result.WriteByte('_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
					// End of acronym before lowercase: "IK" in "APIKey" -> "i_k"
					result.WriteByte('_')
				}
			}
			result.WriteRune(r + ('a' - 'A'))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// GetJSONName returns the JSON field name for a struct field.
// It reads the "json" struct tag; if absent or set to "-", it falls back to
// converting the Go field name to snake_case.
func GetJSONName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" {
		if comma := strings.Index(jsonTag, ","); comma != -1 {
			jsonTag = jsonTag[:comma]
		}
		if jsonTag != "-" {
			return jsonTag
		}
	}
	return ToSnakeCase(field.Name)
}

// ParseIntOrDefault converts a string to int, returning defaultVal if parsing fails.
func ParseIntOrDefault(s string, defaultVal int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return defaultVal
}

// GetOrDefault returns value if it is not the zero value for its type;
// otherwise it returns defaultValue.
func GetOrDefault[T comparable](value, defaultValue T) T {
	var zero T
	if value != zero {
		return value
	}
	return defaultValue
}

// ArgsToMap converts variadic key-value pairs to a map.
// Keys must be strings; non-string keys are silently skipped.
// If the number of arguments is odd, an empty map is returned and a warning is logged.
//
// Usage:
//
//	ArgsToMap("name", "john", "age", 18) -> map[string]any{"name": "john", "age": 18}
func ArgsToMap(args ...any) map[string]any {
	if len(args)%2 != 0 {
		slog.Warn("invalid args in ArgsToMap", "args", args)
		return make(map[string]any)
	}

	result := make(map[string]any, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		result[key] = args[i+1]
	}
	return result
}

