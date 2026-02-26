package entigo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple camelCase",
			input: "userName",
			want:  "user_name",
		},
		{
			name:  "PascalCase",
			input: "UserName",
			want:  "user_name",
		},
		{
			name:  "with ID suffix",
			input: "UserID",
			want:  "user_id",
		},
		{
			name:  "with CPU abbreviation",
			input: "CPUUsage",
			want:  "cpu_usage",
		},
		{
			name:  "with API",
			input: "APIKey",
			want:  "api_key",
		},
		{
			name:  "with URL",
			input: "BaseURL",
			want:  "base_url",
		},
		{
			name:  "single word lowercase",
			input: "name",
			want:  "name",
		},
		{
			name:  "single word uppercase",
			input: "Name",
			want:  "name",
		},
		{
			name:  "multiple words",
			input: "CreatedAt",
			want:  "created_at",
		},
		{
			name:  "already snake_case",
			input: "already_snake",
			want:  "already_snake",
		},
		{
			name:  "consecutive uppercase with IP",
			input: "IPAddress",
			want:  "ip_address",
		},
		{
			name:  "BLE abbreviation",
			input: "BLEDevice",
			want:  "ble_device",
		},
		{
			name:  "MAC address",
			input: "MACAddress",
			want:  "mac_address",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToSnakeCase(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetJSONName(t *testing.T) {
	tests := []struct {
		name  string
		field reflect.StructField
		want  string
	}{
		{
			name: "with json tag",
			field: reflect.StructField{
				Name: "UserName",
				Tag:  `json:"user_name"`,
				Type: reflect.TypeOf(""),
			},
			want: "user_name",
		},
		{
			name: "with json tag and options",
			field: reflect.StructField{
				Name: "UserName",
				Tag:  `json:"user_name,omitempty"`,
				Type: reflect.TypeOf(""),
			},
			want: "user_name",
		},
		{
			name: "json tag with hyphen (ignored)",
			field: reflect.StructField{
				Name: "Internal",
				Tag:  `json:"-"`,
				Type: reflect.TypeOf(""),
			},
			want: "internal",
		},
		{
			name: "no json tag",
			field: reflect.StructField{
				Name: "UserName",
				Type: reflect.TypeOf(""),
			},
			want: "user_name",
		},
		{
			name: "empty json tag",
			field: reflect.StructField{
				Name: "UserName",
				Tag:  `json:""`,
				Type: reflect.TypeOf(""),
			},
			want: "user_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetJSONName(tt.field)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseIntOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal int
		want       int
	}{
		{
			name:       "valid integer",
			input:      "42",
			defaultVal: 0,
			want:       42,
		},
		{
			name:       "negative integer",
			input:      "-10",
			defaultVal: 0,
			want:       -10,
		},
		{
			name:       "zero",
			input:      "0",
			defaultVal: 5,
			want:       0,
		},
		{
			name:       "invalid string returns default",
			input:      "not_a_number",
			defaultVal: 25,
			want:       25,
		},
		{
			name:       "empty string returns default",
			input:      "",
			defaultVal: 10,
			want:       10,
		},
		{
			name:       "float string returns default",
			input:      "3.14",
			defaultVal: 1,
			want:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseIntOrDefault(tt.input, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestArgsToMap(t *testing.T) {
	t.Run("even number of args", func(t *testing.T) {
		result := ArgsToMap("name", "john", "age", 18)
		assert.Equal(t, map[string]any{
			"name": "john",
			"age":  18,
		}, result)
	})

	t.Run("odd number of args returns empty map", func(t *testing.T) {
		result := ArgsToMap("name", "john", "extra")
		assert.Empty(t, result)
	})

	t.Run("non-string key is skipped", func(t *testing.T) {
		result := ArgsToMap(42, "value", "name", "john")
		assert.Len(t, result, 1)
		assert.Equal(t, "john", result["name"])
		_, exists := result["42"]
		assert.False(t, exists, "numeric key should be skipped")
	})

	t.Run("empty args returns empty map", func(t *testing.T) {
		result := ArgsToMap()
		assert.Empty(t, result)
	})

	t.Run("various value types", func(t *testing.T) {
		result := ArgsToMap("count", 42, "rate", 3.14, "active", true)
		assert.Equal(t, 42, result["count"])
		assert.Equal(t, 3.14, result["rate"])
		assert.Equal(t, true, result["active"])
	})
}

func TestGetOrDefault(t *testing.T) {
	t.Run("non-zero value returned", func(t *testing.T) {
		result := GetOrDefault("hello", "default")
		assert.Equal(t, "hello", result)
	})

	t.Run("zero string returns default", func(t *testing.T) {
		result := GetOrDefault("", "default")
		assert.Equal(t, "default", result)
	})

	t.Run("non-zero int returned", func(t *testing.T) {
		result := GetOrDefault(42, 0)
		assert.Equal(t, 42, result)
	})

	t.Run("zero int returns default", func(t *testing.T) {
		result := GetOrDefault(0, 99)
		assert.Equal(t, 99, result)
	})
}

func TestIsValidColumnName(t *testing.T) {
	valid := []struct {
		name  string
		input string
	}{
		{"simple lowercase", "name"},
		{"snake_case", "created_at"},
		{"leading underscore", "_id"},
		{"alphanumeric", "a1b2"},
		{"single char", "x"},
		{"uppercase", "Status"},
		{"mixed case", "UserName"},
		{"all underscore prefix", "_"},
		{"long name", "very_long_column_name_123"},
	}

	for _, tt := range valid {
		t.Run("accept_"+tt.name, func(t *testing.T) {
			assert.True(t, IsValidColumnName(tt.input), "should accept %q", tt.input)
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"starts with digit", "1abc"},
		{"SQL injection semicolon", "name; DROP TABLE users--"},
		{"space in name", "col name"},
		{"dot notation", "table.column"},
		{"SQL comment", "col--"},
		{"single quote injection", "col'OR'1"},
		{"parentheses", "id)"},
		{"asterisk", "col*"},
		{"equals sign", "col=1"},
		{"hyphen", "my-column"},
		{"just digits", "123"},
	}

	for _, tt := range invalid {
		t.Run("reject_"+tt.name, func(t *testing.T) {
			assert.False(t, IsValidColumnName(tt.input), "should reject %q", tt.input)
		})
	}
}

