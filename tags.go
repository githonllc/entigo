package entigo

import (
	"reflect"
	"strings"
	"sync"
)

// ScopeAttribute represents a parsed attribute from an ent scope tag.
//
// Example:
//
//	type User struct {
//	    entigo.BaseEntity
//	    Name   string `json:"name" ent:"scope=update(required,max=100),sort(default=asc)"`
//	    Email  string `json:"email" ent:"scope=update(readonly,format=email)"`
//	    Status string `json:"status" ent:"scope=update(readonly,default=active),filter(name=status)"`
//	    Score  int    `json:"score" ent:"scope=update(min=0,max=100)"`
//	}
type ScopeAttribute struct {
	Name  string
	Value string
}

// typeCacheKey is used as the map key for the thread-safe type cache.
type typeCacheKey struct {
	modelType  reflect.Type
	scope      string
	usePointer bool
}

var (
	typeCacheMu sync.RWMutex
	typeCache   = make(map[typeCacheKey]reflect.Type)
)

// ParseScopeAttributes parses the attributes within a scope's parentheses.
//
// Valueless attribute:
//
//	scope=update(readonly)
//	scope=update(required,readonly)
//
// Valued attribute:
//
//	scope=update(max=100)
//	scope=update(type=json,max=100)
//
// Mixed attributes:
//
//	scope=update(readonly,max=100,min=0)
//	scope=update(required,type=string,max=100)
func ParseScopeAttributes(scopeStr string) []ScopeAttribute {
	// Remove parentheses and split attributes
	content := strings.TrimSuffix(strings.TrimPrefix(scopeStr, "("), ")")
	parts := strings.Split(content, ",")

	attributes := make([]ScopeAttribute, 0)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if there is an equal sign
		if idx := strings.Index(part, "="); idx != -1 {
			// Valued attribute
			name := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			attributes = append(attributes, ScopeAttribute{
				Name:  name,
				Value: value,
			})
		} else {
			// Valueless attribute
			attributes = append(attributes, ScopeAttribute{
				Name:  part,
				Value: "",
			})
		}
	}
	return attributes
}

// GetScopeAndAttributes returns whether the given tag contains the specified scope,
// and if so, returns its parsed attributes.
func GetScopeAndAttributes(tag, scopeName string) (bool, []ScopeAttribute) {
	if !strings.Contains(tag, "scope=") {
		return false, nil
	}

	scopes := strings.Split(strings.Split(tag, "scope=")[1], ",")
	for _, s := range scopes {
		s = strings.TrimSpace(s)

		// Check basic scope match
		if s == scopeName {
			return true, nil
		}

		// Check scope with parenthesized attributes
		if strings.HasPrefix(s, scopeName+"(") && strings.HasSuffix(s, ")") {
			attributes := ParseScopeAttributes(strings.TrimPrefix(s, scopeName))
			return true, attributes
		}
	}
	return false, nil
}

// HasScopeAttribute checks whether a specific attribute exists in the given scope.
func HasScopeAttribute(tag, scope, attrName string) bool {
	hasScope, attrs := GetScopeAndAttributes(tag, scope)
	if !hasScope {
		return false
	}

	for _, attr := range attrs {
		if attr.Name == attrName {
			return true
		}
	}
	return false
}

// GetScopeAttributeValue returns the value of a specific attribute in the given scope.
// The second return value indicates whether the attribute was found.
func GetScopeAttributeValue(tag, scope, attrName string) (string, bool) {
	hasScope, attrs := GetScopeAndAttributes(tag, scope)
	if !hasScope {
		return "", false
	}

	for _, attr := range attrs {
		if attr.Name == attrName {
			return attr.Value, true
		}
	}
	return "", false
}

// GetDbFields extracts all database field names from a struct,
// including fields from embedded structs.
func GetDbFields(s any) []string {
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	var fields []string
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		// Handle embedded/nested structs
		if field.Anonymous {
			nestedFields := GetDbFields(val.Field(i).Interface())
			fields = append(fields, nestedFields...)
		} else if fieldName := GetDbFieldName(field); fieldName != "" {
			fields = append(fields, fieldName)
		}
	}
	return fields
}

// GetScopeFields extracts the database field names for fields that have the specified scope.
func GetScopeFields(s any, action string) []string {
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	var fields []string
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)

		// Handle embedded/nested structs
		if field.Anonymous {
			nestedFields := GetScopeFields(val.Field(i).Interface(), action)
			fields = append(fields, nestedFields...)
		} else if hasScope(field.Tag.Get("ent"), action) {
			if fieldName := GetDbFieldName(field); fieldName != "" {
				fields = append(fields, fieldName)
			}
		}
	}
	return fields
}

// GetDbFieldName extracts the database column name from a struct field.
// It checks the gorm tag first (for column: directive), then falls back
// to the json tag, and finally converts the Go field name to snake_case.
func GetDbFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("gorm")
	if tag == "-" {
		return ""
	}

	// If there is no gorm tag, try to get it from json tag
	if tag == "" {
		return GetJSONName(field)
	}

	// Extract field names from gorm tag
	tags := strings.Split(tag, ";")
	for _, t := range tags {
		// Check if it is marked as ignored
		if t == "-" {
			return ""
		}
		if strings.HasPrefix(t, "column:") {
			return strings.TrimPrefix(t, "column:")
		}
	}

	return ToSnakeCase(field.Name)
}

// CreateTypeFromScopeTag creates a new reflect.Type based on the model and scope.
// It filters fields by scope tag, handles embedded structs, and caches the result
// for thread-safe reuse.
func CreateTypeFromScopeTag(model any, scope string, usePointer bool) reflect.Type {
	modelType := reflect.TypeOf(model)
	// If it is a pointer, get the type pointed to by the pointer
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	if modelType.Kind() != reflect.Struct {
		return reflect.StructOf([]reflect.StructField{})
	}

	key := typeCacheKey{
		modelType:  modelType,
		scope:      scope,
		usePointer: usePointer,
	}

	// Check cache with read lock first
	typeCacheMu.RLock()
	if cached, ok := typeCache[key]; ok {
		typeCacheMu.RUnlock()
		return cached
	}
	typeCacheMu.RUnlock()

	// Cache miss: create the type and store it under write lock
	// Use a local recursion cache to prevent infinite loops during creation
	localCache := make(map[reflect.Type]reflect.Type)
	resultType := createStructType(modelType, scope, usePointer, localCache)

	typeCacheMu.Lock()
	typeCache[key] = resultType
	typeCacheMu.Unlock()

	return resultType
}

// createStructType handles struct type creation with embedded fields.
// It uses a local cache to prevent recursive infinite loops.
func createStructType(modelType reflect.Type, scope string, usePointer bool, cache map[reflect.Type]reflect.Type) reflect.Type {
	// Check cache to prevent recursion
	if cachedType, ok := cache[modelType]; ok {
		return cachedType
	}

	var fields []reflect.StructField

	// Process all fields
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		tag := field.Tag.Get("ent")

		if !field.IsExported() {
			continue
		}

		// Handle embedded structures
		if field.Type.Kind() == reflect.Struct && field.Anonymous {
			nestedType := createStructType(field.Type, scope, usePointer, cache)
			for j := 0; j < nestedType.NumField(); j++ {
				nestedField := nestedType.Field(j)
				nestedTag := nestedField.Tag.Get("ent")
				if !hasScope(nestedTag, scope) {
					continue
				}
				newField := reflect.StructField{
					Name: nestedField.Name,
					Type: nestedField.Type,
					Tag:  nestedField.Tag,
				}
				if usePointer && newField.Type.Kind() != reflect.Ptr {
					newField.Type = reflect.PointerTo(newField.Type)
				}
				ensureJSONTag(&newField)
				fields = append(fields, newField)
			}
			continue
		}

		if !hasScope(tag, scope) {
			continue
		}

		// Create a new field
		newField := reflect.StructField{
			Name: field.Name,
			Type: field.Type,
			Tag:  field.Tag,
		}

		// If the generated new type field requires pointer type and the field is not a pointer
		if usePointer && newField.Type.Kind() != reflect.Ptr {
			newField.Type = reflect.PointerTo(newField.Type)
		}

		ensureJSONTag(&newField)
		fields = append(fields, newField)
	}

	// If there are no fields, return the empty struct type
	if len(fields) == 0 {
		return reflect.StructOf([]reflect.StructField{})
	}

	result := reflect.StructOf(fields)
	cache[modelType] = result
	return result
}

// ensureJSONTag ensures that the struct field has a json tag.
// If missing, it adds one based on the snake_case conversion of the field name.
func ensureJSONTag(field *reflect.StructField) {
	if _, ok := field.Tag.Lookup("json"); !ok {
		field.Tag = reflect.StructTag(
			string(field.Tag) + ` json:"` + ToSnakeCase(field.Name) + `"`,
		)
	}
}

// hasScope checks whether the ent tag contains the specified scope.
func hasScope(tag, action string) bool {
	found, _ := GetScopeAndAttributes(tag, action)
	return found
}
