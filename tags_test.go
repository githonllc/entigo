package entigo

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestModel is a sample model used across tag-related tests.
type TestModel struct {
	BaseEntity
	Name   string `json:"name" ent:"scope=create,update,patch,response,filter"`
	Email  string `json:"email" ent:"scope=create,response,filter"`
	Status string `json:"status" ent:"scope=response,filter"`
	Secret string `json:"-"`
}

func TestParseScopeAttributes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []ScopeAttribute
	}{
		{
			name:  "single valueless attribute",
			input: "(readonly)",
			want: []ScopeAttribute{
				{Name: "readonly", Value: ""},
			},
		},
		{
			name:  "multiple valueless attributes",
			input: "(required,readonly)",
			want: []ScopeAttribute{
				{Name: "required", Value: ""},
				{Name: "readonly", Value: ""},
			},
		},
		{
			name:  "valued attributes",
			input: "(max=100,min=0)",
			want: []ScopeAttribute{
				{Name: "max", Value: "100"},
				{Name: "min", Value: "0"},
			},
		},
		{
			name:  "mixed attributes",
			input: "(readonly,max=100,min=0)",
			want: []ScopeAttribute{
				{Name: "readonly", Value: ""},
				{Name: "max", Value: "100"},
				{Name: "min", Value: "0"},
			},
		},
		{
			name:  "empty input",
			input: "()",
			want:  []ScopeAttribute{},
		},
		{
			name:  "attributes with spaces",
			input: "( readonly , max=100 )",
			want: []ScopeAttribute{
				{Name: "readonly", Value: ""},
				{Name: "max", Value: "100"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseScopeAttributes(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetScopeAndAttributes(t *testing.T) {
	tests := []struct {
		name      string
		tag       string
		scope     string
		wantFound bool
		wantAttrs []ScopeAttribute
	}{
		{
			name:      "simple scope present",
			tag:       "scope=create,update,response",
			scope:     "create",
			wantFound: true,
			wantAttrs: nil,
		},
		{
			name:      "scope not present",
			tag:       "scope=create,update,response",
			scope:     "delete",
			wantFound: false,
			wantAttrs: nil,
		},
		{
			name:      "scope with single attribute",
			tag:       "scope=update(readonly)",
			scope:     "update",
			wantFound: true,
			wantAttrs: []ScopeAttribute{
				{Name: "readonly", Value: ""},
			},
		},
		{
			name:      "scope with valued attribute",
			tag:       "scope=update(max=100)",
			scope:     "update",
			wantFound: true,
			wantAttrs: []ScopeAttribute{
				{Name: "max", Value: "100"},
			},
		},
		{
			name:      "tag without scope prefix",
			tag:       "some_other_tag",
			scope:     "create",
			wantFound: false,
			wantAttrs: nil,
		},
		{
			name:      "empty tag",
			tag:       "",
			scope:     "create",
			wantFound: false,
			wantAttrs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, attrs := GetScopeAndAttributes(tt.tag, tt.scope)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantAttrs, attrs)
		})
	}
}

func TestHasScopeAttribute(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		scope    string
		attrName string
		want     bool
	}{
		{
			name:     "single attribute present",
			tag:      "scope=update(readonly)",
			scope:    "update",
			attrName: "readonly",
			want:     true,
		},
		{
			name:     "valued attribute present",
			tag:      "scope=update(max=100)",
			scope:    "update",
			attrName: "max",
			want:     true,
		},
		{
			name:     "attribute absent",
			tag:      "scope=update(readonly)",
			scope:    "update",
			attrName: "max",
			want:     false,
		},
		{
			name:     "scope absent",
			tag:      "scope=create,response",
			scope:    "update",
			attrName: "required",
			want:     false,
		},
		{
			name:     "scope without attributes",
			tag:      "scope=update",
			scope:    "update",
			attrName: "required",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasScopeAttribute(tt.tag, tt.scope, tt.attrName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateTypeFromScopeTag(t *testing.T) {
	// Clear the type cache before this test to ensure deterministic results
	typeCacheMu.Lock()
	typeCache = make(map[typeCacheKey]reflect.Type)
	typeCacheMu.Unlock()

	t.Run("create scope", func(t *testing.T) {
		typ := CreateTypeFromScopeTag(TestModel{}, "create", false)
		assert.Equal(t, reflect.Struct, typ.Kind())

		// TestModel has Name and Email in create scope
		fieldNames := getFieldNames(typ)
		assert.Contains(t, fieldNames, "Name")
		assert.Contains(t, fieldNames, "Email")
		assert.NotContains(t, fieldNames, "Status", "Status is not in create scope")
		assert.NotContains(t, fieldNames, "Secret", "Secret has no ent tag")
	})

	t.Run("response scope", func(t *testing.T) {
		typ := CreateTypeFromScopeTag(TestModel{}, "response", false)
		assert.Equal(t, reflect.Struct, typ.Kind())

		// Response scope should include ID, CreatedAt, UpdatedAt from BaseEntity
		// plus Name, Email, Status from TestModel
		fieldNames := getFieldNames(typ)
		assert.Contains(t, fieldNames, "ID")
		assert.Contains(t, fieldNames, "CreatedAt")
		assert.Contains(t, fieldNames, "UpdatedAt")
		assert.Contains(t, fieldNames, "Name")
		assert.Contains(t, fieldNames, "Email")
		assert.Contains(t, fieldNames, "Status")
		assert.NotContains(t, fieldNames, "Secret")
	})

	t.Run("patch scope uses pointers", func(t *testing.T) {
		typ := CreateTypeFromScopeTag(TestModel{}, "patch", true)
		assert.Equal(t, reflect.Struct, typ.Kind())

		// Patch scope should use pointer fields
		fieldNames := getFieldNames(typ)
		assert.Contains(t, fieldNames, "Name")

		// Verify the Name field is a pointer type
		nameField, ok := typ.FieldByName("Name")
		assert.True(t, ok, "Name field should exist in patch type")
		assert.Equal(t, reflect.Ptr, nameField.Type.Kind(), "Name should be a pointer type in patch scope")
	})

	t.Run("update scope", func(t *testing.T) {
		typ := CreateTypeFromScopeTag(TestModel{}, "update", false)
		assert.Equal(t, reflect.Struct, typ.Kind())

		fieldNames := getFieldNames(typ)
		assert.Contains(t, fieldNames, "Name")
		assert.NotContains(t, fieldNames, "Email", "Email is not in update scope")
		assert.NotContains(t, fieldNames, "Status", "Status is not in update scope")
	})

	t.Run("filter scope", func(t *testing.T) {
		typ := CreateTypeFromScopeTag(TestModel{}, "filter", false)
		assert.Equal(t, reflect.Struct, typ.Kind())

		fieldNames := getFieldNames(typ)
		assert.Contains(t, fieldNames, "Name")
		assert.Contains(t, fieldNames, "Email")
		assert.Contains(t, fieldNames, "Status")
		assert.Contains(t, fieldNames, "ID")
	})

	t.Run("nonexistent scope returns empty struct", func(t *testing.T) {
		typ := CreateTypeFromScopeTag(TestModel{}, "nonexistent", false)
		assert.Equal(t, reflect.Struct, typ.Kind())
		assert.Equal(t, 0, typ.NumField(), "nonexistent scope should produce zero fields")
	})

	t.Run("pointer model input", func(t *testing.T) {
		typ := CreateTypeFromScopeTag(&TestModel{}, "create", false)
		assert.Equal(t, reflect.Struct, typ.Kind())

		fieldNames := getFieldNames(typ)
		assert.Contains(t, fieldNames, "Name")
		assert.Contains(t, fieldNames, "Email")
	})

	t.Run("non-struct input returns empty struct", func(t *testing.T) {
		typ := CreateTypeFromScopeTag("not a struct", "create", false)
		assert.Equal(t, reflect.Struct, typ.Kind())
		assert.Equal(t, 0, typ.NumField())
	})
}

func TestCreateTypeFromScopeTagThreadSafety(t *testing.T) {
	// Clear the type cache to test concurrent creation
	typeCacheMu.Lock()
	typeCache = make(map[typeCacheKey]reflect.Type)
	typeCacheMu.Unlock()

	var wg sync.WaitGroup
	scopes := []string{"create", "update", "patch", "response", "filter"}

	// Run multiple goroutines concurrently creating types
	for i := 0; i < 100; i++ {
		for _, scope := range scopes {
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				// This should not panic under concurrent access
				typ := CreateTypeFromScopeTag(TestModel{}, s, false)
				assert.Equal(t, reflect.Struct, typ.Kind())
			}(scope)
		}
	}

	wg.Wait()
}

func TestGetDbFields(t *testing.T) {
	type SimpleModel struct {
		BaseEntity
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	fields := GetDbFields(SimpleModel{})
	assert.Contains(t, fields, "id")
	assert.Contains(t, fields, "created_at")
	assert.Contains(t, fields, "updated_at")
	assert.Contains(t, fields, "name")
	assert.Contains(t, fields, "value")

	// Test with pointer
	fieldsPtr := GetDbFields(&SimpleModel{})
	assert.Equal(t, fields, fieldsPtr, "pointer and value should produce same fields")
}

func TestGetScopeFields(t *testing.T) {
	fields := GetScopeFields(TestModel{}, "create")
	assert.Contains(t, fields, "name")
	assert.Contains(t, fields, "email")
	assert.NotContains(t, fields, "status", "status is not in create scope")

	responseFields := GetScopeFields(TestModel{}, "response")
	assert.Contains(t, responseFields, "id")
	assert.Contains(t, responseFields, "created_at")
	assert.Contains(t, responseFields, "updated_at")
	assert.Contains(t, responseFields, "name")
	assert.Contains(t, responseFields, "email")
	assert.Contains(t, responseFields, "status")

	// Test with pointer
	ptrFields := GetScopeFields(&TestModel{}, "create")
	assert.Equal(t, fields, ptrFields)
}

func TestGetDbFieldName(t *testing.T) {
	tests := []struct {
		name     string
		field    reflect.StructField
		expected string
	}{
		{
			name: "json tag",
			field: reflect.StructField{
				Name: "UserName",
				Tag:  `json:"user_name"`,
			},
			expected: "user_name",
		},
		{
			name: "gorm column tag",
			field: reflect.StructField{
				Name: "UserName",
				Tag:  `gorm:"column:username"`,
			},
			expected: "username",
		},
		{
			name: "gorm ignored",
			field: reflect.StructField{
				Name: "Internal",
				Tag:  `gorm:"-"`,
			},
			expected: "",
		},
		{
			name: "no tags fallback to snake_case",
			field: reflect.StructField{
				Name: "UserName",
			},
			expected: "user_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the Type field since reflect.StructField needs it
			if tt.field.Type == nil {
				tt.field.Type = reflect.TypeOf("")
			}
			got := GetDbFieldName(tt.field)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetScopeAttributeValue(t *testing.T) {
	tests := []struct {
		name      string
		tag       string
		scope     string
		attrName  string
		wantValue string
		wantFound bool
	}{
		{
			name:      "attribute with value",
			tag:       "scope=update(max=100)",
			scope:     "update",
			attrName:  "max",
			wantValue: "100",
			wantFound: true,
		},
		{
			name:      "valueless attribute",
			tag:       "scope=update(readonly)",
			scope:     "update",
			attrName:  "readonly",
			wantValue: "",
			wantFound: true,
		},
		{
			name:      "attribute not found",
			tag:       "scope=update(max=100)",
			scope:     "update",
			attrName:  "min",
			wantValue: "",
			wantFound: false,
		},
		{
			name:      "scope not found",
			tag:       "scope=create",
			scope:     "update",
			attrName:  "max",
			wantValue: "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := GetScopeAttributeValue(tt.tag, tt.scope, tt.attrName)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}

// getFieldNames returns all field names from a reflect.Type
func getFieldNames(typ reflect.Type) []string {
	names := make([]string, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		names[i] = typ.Field(i).Name
	}
	return names
}
