package entigo

import (
	"fmt"
	"reflect"
)

// Converter handles conversion between DTOs and models using reflection.
// It pre-computes the response, create, update, and patch types from the model's
// ent scope tags for efficient runtime DTO generation.
type Converter[T any] struct {
	model        T
	responseType reflect.Type
	createType   reflect.Type
	updateType   reflect.Type
	patchType    reflect.Type
}

// NewConverter creates a new Converter instance with pre-computed types
// derived from the model's ent scope tags.
func NewConverter[T any](model T) *Converter[T] {
	return &Converter[T]{
		model:        model,
		responseType: CreateTypeFromScopeTag(model, "response", false),
		createType:   CreateTypeFromScopeTag(model, "create", false),
		updateType:   CreateTypeFromScopeTag(model, "update", false),
		patchType:    CreateTypeFromScopeTag(model, "patch", true),
	}
}

// GenCreateRequest generates a new create request DTO instance.
func (c *Converter[T]) GenCreateRequest() any {
	return reflect.New(c.createType).Interface()
}

// GenUpdateRequest generates a new update request DTO instance.
func (c *Converter[T]) GenUpdateRequest() any {
	return reflect.New(c.updateType).Interface()
}

// GenPatchRequest generates a new patch request DTO instance.
// Patch DTOs use pointer fields to distinguish between zero values and absent fields.
func (c *Converter[T]) GenPatchRequest() any {
	return reflect.New(c.patchType).Interface()
}

// GenResponse generates a new response DTO instance.
func (c *Converter[T]) GenResponse() any {
	return reflect.New(c.responseType).Interface()
}

// NewModelInstance creates a new zero-value instance of the model type T.
func (c *Converter[T]) NewModelInstance() T {
	return NewInstance[T]()
}

// ToModel converts input data to a new model instance of type T.
func (c *Converter[T]) ToModel(input any) (T, error) {
	var zero T
	model := c.NewModelInstance()

	if err := c.ToExistingModel(input, &model); err != nil {
		return zero, err
	}

	return model, nil
}

// ToExistingModel converts input data into an existing model instance.
func (c *Converter[T]) ToExistingModel(input any, model *T) error {
	if model == nil {
		return fmt.Errorf("model cannot be nil")
	}
	return Copy(input, model)
}

// ToResponse converts a model to a response DTO.
func (c *Converter[T]) ToResponse(model T) (any, error) {
	resp := c.GenResponse()
	err := Copy(model, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to copy model to response: %v", err)
	}
	return resp, nil
}

// ToListResponse converts a slice of models to a slice of response DTOs.
func (c *Converter[T]) ToListResponse(models []T) ([]any, error) {
	items := make([]any, len(models))
	for i, model := range models {
		resp, err := c.ToResponse(model)
		if err != nil {
			return nil, err
		}
		items[i] = resp
	}
	return items, nil
}

// GetResponseType returns the pre-computed response struct type.
func (c *Converter[T]) GetResponseType() reflect.Type {
	return c.responseType
}

// GetCreateType returns the pre-computed create struct type.
func (c *Converter[T]) GetCreateType() reflect.Type {
	return c.createType
}

// GetUpdateType returns the pre-computed update struct type.
func (c *Converter[T]) GetUpdateType() reflect.Type {
	return c.updateType
}

// GetPatchType returns the pre-computed patch struct type.
func (c *Converter[T]) GetPatchType() reflect.Type {
	return c.patchType
}

// HasCreateType returns true if the model has fields tagged with the "create" scope.
func (c *Converter[T]) HasCreateType() bool {
	return !IsEmptyStructType(c.createType)
}

// HasUpdateType returns true if the model has fields tagged with the "update" scope.
func (c *Converter[T]) HasUpdateType() bool {
	return !IsEmptyStructType(c.updateType)
}

// HasPatchType returns true if the model has fields tagged with the "patch" scope.
func (c *Converter[T]) HasPatchType() bool {
	return !IsEmptyStructType(c.patchType)
}

// HasResponseType returns true if the model has fields tagged with the "response" scope.
func (c *Converter[T]) HasResponseType() bool {
	return !IsEmptyStructType(c.responseType)
}

// IsEmptyStructType checks if the given reflect.Type represents an empty struct (zero fields).
func IsEmptyStructType(t reflect.Type) bool {
	if t == nil {
		return true
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	return t.NumField() == 0
}
