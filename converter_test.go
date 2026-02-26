package entigo

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ConverterTestModel is used specifically in converter tests to avoid
// interference with the global type cache from other tests.
type ConverterTestModel struct {
	BaseEntity
	Name   string `json:"name" ent:"scope=create,update,patch,response,filter"`
	Email  string `json:"email" ent:"scope=create,response,filter"`
	Status string `json:"status" ent:"scope=response,filter"`
	Secret string `json:"-"`
}

// NoScopeModel has no ent scope tags at all.
type NoScopeModel struct {
	BaseEntity
	Internal string `json:"-"`
}

func newConverterForTest() *Converter[*ConverterTestModel] {
	// Clear the type cache to avoid interference from other tests
	typeCacheMu.Lock()
	typeCache = make(map[typeCacheKey]reflect.Type)
	typeCacheMu.Unlock()

	return NewConverter[*ConverterTestModel](&ConverterTestModel{})
}

func TestNewConverter(t *testing.T) {
	conv := newConverterForTest()

	assert.NotNil(t, conv, "NewConverter should return a non-nil converter")
	assert.NotNil(t, conv.responseType, "response type should be initialized")
	assert.NotNil(t, conv.createType, "create type should be initialized")
	assert.NotNil(t, conv.updateType, "update type should be initialized")
	assert.NotNil(t, conv.patchType, "patch type should be initialized")
}

func TestGenCreateRequest(t *testing.T) {
	conv := newConverterForTest()

	req := conv.GenCreateRequest()
	assert.NotNil(t, req, "GenCreateRequest should return a non-nil value")

	// Verify it is a pointer to a struct
	reqType := reflect.TypeOf(req)
	assert.Equal(t, reflect.Ptr, reqType.Kind(), "create request should be a pointer")
	assert.Equal(t, reflect.Struct, reqType.Elem().Kind(), "create request should point to a struct")

	// Verify the create request has the correct fields (Name and Email)
	elemType := reqType.Elem()
	fieldNames := make([]string, elemType.NumField())
	for i := 0; i < elemType.NumField(); i++ {
		fieldNames[i] = elemType.Field(i).Name
	}
	assert.Contains(t, fieldNames, "Name")
	assert.Contains(t, fieldNames, "Email")
	assert.NotContains(t, fieldNames, "Status", "Status is not in create scope")
	assert.NotContains(t, fieldNames, "Secret", "Secret has no ent tag")
}

func TestGenResponseRequest(t *testing.T) {
	conv := newConverterForTest()

	resp := conv.GenResponse()
	assert.NotNil(t, resp, "GenResponse should return a non-nil value")

	reqType := reflect.TypeOf(resp)
	assert.Equal(t, reflect.Ptr, reqType.Kind())
	assert.Equal(t, reflect.Struct, reqType.Elem().Kind())

	// Response scope should include BaseEntity fields and model fields
	elemType := reqType.Elem()
	fieldNames := make([]string, elemType.NumField())
	for i := 0; i < elemType.NumField(); i++ {
		fieldNames[i] = elemType.Field(i).Name
	}
	assert.Contains(t, fieldNames, "ID")
	assert.Contains(t, fieldNames, "CreatedAt")
	assert.Contains(t, fieldNames, "UpdatedAt")
	assert.Contains(t, fieldNames, "Name")
	assert.Contains(t, fieldNames, "Email")
	assert.Contains(t, fieldNames, "Status")
	assert.NotContains(t, fieldNames, "Secret")
}

func TestGenPatchRequest(t *testing.T) {
	conv := newConverterForTest()

	req := conv.GenPatchRequest()
	assert.NotNil(t, req, "GenPatchRequest should return a non-nil value")

	reqType := reflect.TypeOf(req)
	assert.Equal(t, reflect.Ptr, reqType.Kind())

	elemType := reqType.Elem()
	// Patch scope fields should be pointer types
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		assert.Equal(t, reflect.Ptr, field.Type.Kind(),
			"patch field %s should be a pointer type", field.Name)
	}

	// Name should be in patch scope
	fieldNames := make([]string, elemType.NumField())
	for i := 0; i < elemType.NumField(); i++ {
		fieldNames[i] = elemType.Field(i).Name
	}
	assert.Contains(t, fieldNames, "Name")
}

func TestToModel(t *testing.T) {
	resetIDGenerator()
	conv := newConverterForTest()

	// Create an input struct with create-scope fields
	createReq := conv.GenCreateRequest()
	reqVal := reflect.ValueOf(createReq).Elem()

	// Set field values on the create request
	nameField := reqVal.FieldByName("Name")
	if nameField.IsValid() && nameField.CanSet() {
		nameField.SetString("John Doe")
	}
	emailField := reqVal.FieldByName("Email")
	if emailField.IsValid() && emailField.CanSet() {
		emailField.SetString("john@example.com")
	}

	// Convert to model
	model, err := conv.ToModel(createReq)
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, "John Doe", model.Name)
	assert.Equal(t, "john@example.com", model.Email)
}

func TestToResponse(t *testing.T) {
	resetIDGenerator()
	conv := newConverterForTest()

	model := &ConverterTestModel{}
	model.Init()
	model.Name = "Jane Doe"
	model.Email = "jane@example.com"
	model.Status = "active"
	model.Secret = "should_not_appear"

	resp, err := conv.ToResponse(model)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify response fields are populated via reflection
	respVal := reflect.ValueOf(resp).Elem()

	nameField := respVal.FieldByName("Name")
	assert.True(t, nameField.IsValid(), "response should have Name field")
	assert.Equal(t, "Jane Doe", nameField.String())

	emailField := respVal.FieldByName("Email")
	assert.True(t, emailField.IsValid(), "response should have Email field")
	assert.Equal(t, "jane@example.com", emailField.String())

	statusField := respVal.FieldByName("Status")
	assert.True(t, statusField.IsValid(), "response should have Status field")
	assert.Equal(t, "active", statusField.String())

	// Secret should not be in the response type
	secretField := respVal.FieldByName("Secret")
	assert.False(t, secretField.IsValid(), "Secret should not be in response")
}

func TestToListResponse(t *testing.T) {
	resetIDGenerator()
	conv := newConverterForTest()

	models := []*ConverterTestModel{
		{Name: "Alice", Email: "alice@example.com", Status: "active"},
		{Name: "Bob", Email: "bob@example.com", Status: "inactive"},
	}

	// Initialize models so they have IDs
	for _, m := range models {
		m.Init()
	}

	items, err := conv.ToListResponse(models)
	assert.NoError(t, err)
	assert.Len(t, items, 2)

	// Verify first item
	firstVal := reflect.ValueOf(items[0]).Elem()
	assert.Equal(t, "Alice", firstVal.FieldByName("Name").String())

	// Verify second item
	secondVal := reflect.ValueOf(items[1]).Elem()
	assert.Equal(t, "Bob", secondVal.FieldByName("Name").String())
}

func TestToListResponseEmpty(t *testing.T) {
	conv := newConverterForTest()

	items, err := conv.ToListResponse([]*ConverterTestModel{})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
}

func TestHasCreateType(t *testing.T) {
	// Clear the type cache before testing
	typeCacheMu.Lock()
	typeCache = make(map[typeCacheKey]reflect.Type)
	typeCacheMu.Unlock()

	t.Run("model with create scope", func(t *testing.T) {
		conv := NewConverter[*ConverterTestModel](&ConverterTestModel{})
		assert.True(t, conv.HasCreateType(), "ConverterTestModel should have create type")
	})

	t.Run("model without create scope", func(t *testing.T) {
		// Clear cache again for fresh evaluation
		typeCacheMu.Lock()
		typeCache = make(map[typeCacheKey]reflect.Type)
		typeCacheMu.Unlock()

		conv := NewConverter[*NoScopeModel](&NoScopeModel{})
		assert.False(t, conv.HasCreateType(), "NoScopeModel should not have create type")
	})
}

func TestHasResponseType(t *testing.T) {
	// Clear the type cache before testing
	typeCacheMu.Lock()
	typeCache = make(map[typeCacheKey]reflect.Type)
	typeCacheMu.Unlock()

	conv := NewConverter[*ConverterTestModel](&ConverterTestModel{})
	assert.True(t, conv.HasResponseType(), "ConverterTestModel should have response type")
}

func TestConverterGetTypes(t *testing.T) {
	conv := newConverterForTest()

	assert.NotNil(t, conv.GetResponseType())
	assert.NotNil(t, conv.GetCreateType())
	assert.NotNil(t, conv.GetUpdateType())
	assert.NotNil(t, conv.GetPatchType())

	// All should be struct types
	assert.Equal(t, reflect.Struct, conv.GetResponseType().Kind())
	assert.Equal(t, reflect.Struct, conv.GetCreateType().Kind())
	assert.Equal(t, reflect.Struct, conv.GetUpdateType().Kind())
	assert.Equal(t, reflect.Struct, conv.GetPatchType().Kind())
}

func TestIsEmptyStructType(t *testing.T) {
	// nil type
	assert.True(t, IsEmptyStructType(nil))

	// Empty struct type
	emptyType := reflect.StructOf([]reflect.StructField{})
	assert.True(t, IsEmptyStructType(emptyType))

	// Non-empty struct type
	nonEmptyType := reflect.TypeOf(struct{ Name string }{})
	assert.False(t, IsEmptyStructType(nonEmptyType))

	// Non-struct type
	intType := reflect.TypeOf(0)
	assert.False(t, IsEmptyStructType(intType))
}

func TestNewModelInstance(t *testing.T) {
	conv := newConverterForTest()
	model := conv.NewModelInstance()

	assert.NotNil(t, model, "NewModelInstance should return a non-nil model")
	assert.Equal(t, "", model.Name, "new model instance should have zero Name")
	assert.Equal(t, "", model.Email, "new model instance should have zero Email")
	assert.Equal(t, ID(0), model.ID, "new model instance should have zero ID")
}

func TestConverterThreadSafety(t *testing.T) {
	conv := newConverterForTest()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = conv.GenCreateRequest()
			_ = conv.GenResponse()
			_ = conv.GenPatchRequest()
			_ = conv.GenUpdateRequest()
		}()
	}
	wg.Wait()
}
