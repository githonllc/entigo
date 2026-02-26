package ginx

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/gocarina/gocsv"
)

// Common CSV delimiters.
const (
	CommaSeparator     = ','
	SemicolonSeparator = ';'
	TabSeparator       = '\t'
)

// CSVOption defines the function type for the options pattern.
type CSVOption func(*CSVOptions)

// CSVOptions holds all configurable CSV options.
type CSVOptions struct {
	UseCRLF bool // Use CRLF as line terminator
	Comma   rune // CSV field separator
}

// WithUseCRLF sets line ending to CRLF instead of LF.
func WithUseCRLF(useCRLF bool) CSVOption {
	return func(opts *CSVOptions) {
		opts.UseCRLF = useCRLF
	}
}

// WithComma sets a custom delimiter for CSV fields.
// Common delimiters: CommaSeparator, SemicolonSeparator, TabSeparator.
func WithComma(comma rune) CSVOption {
	return func(opts *CSVOptions) {
		opts.Comma = comma
	}
}

// ToCSV converts a slice of structs (or a single struct) to CSV bytes.
//
// Usage:
//
//	type User struct {
//	    ID   int    `csv:"id"`
//	    Name string `csv:"name"`
//	}
//
//	users := []User{{ID: 1, Name: "John"}}
//	data, err := ToCSV(users, WithComma(SemicolonSeparator))
func ToCSV(entity any, opts ...CSVOption) ([]byte, error) {
	prepared, err := prepareEntities(entity)
	if err != nil {
		return nil, err
	}
	if prepared == nil {
		return nil, nil
	}
	entity = prepared

	// Get reflect value and handle pointer
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Create slice with proper type
	var entities interface{}
	if v.Kind() != reflect.Slice {
		// For single object
		entities = []any{entity}
	} else {
		// For slice
		entities = entity
	}

	// Apply options
	options := &CSVOptions{
		Comma: CommaSeparator, // Default to comma separator
	}
	for _, opt := range opts {
		opt(options)
	}

	var buf bytes.Buffer
	csvWriter := gocsv.DefaultCSVWriter(&buf)
	csvWriter.UseCRLF = options.UseCRLF
	csvWriter.Comma = options.Comma

	if err := gocsv.MarshalCSV(entities, csvWriter); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// getConcreteValue converts an interface{} or pointer to its concrete struct value.
func getConcreteValue(v reflect.Value) (reflect.Value, error) {
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("expected struct type, got %v", v.Kind())
	}

	return v, nil
}

// prepareEntities converts input to proper type for CSV marshaling.
func prepareEntities(entity any) (interface{}, error) {
	if entity == nil {
		return nil, nil
	}

	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Handle single object
	if v.Kind() != reflect.Slice {
		concreteValue, err := getConcreteValue(v)
		if err != nil {
			return nil, err
		}
		// Create slice with single element
		sliceType := reflect.SliceOf(concreteValue.Type())
		newSlice := reflect.MakeSlice(sliceType, 1, 1)
		newSlice.Index(0).Set(concreteValue)
		return newSlice.Interface(), nil
	}

	// Handle empty slice
	sliceLen := v.Len()
	if sliceLen == 0 {
		return nil, nil
	}

	// Get concrete type from first element
	firstElem, err := getConcreteValue(v.Index(0))
	if err != nil {
		return nil, err
	}

	// Create and fill new slice with concrete type
	sliceType := reflect.SliceOf(firstElem.Type())
	newSlice := reflect.MakeSlice(sliceType, sliceLen, sliceLen)

	for i := 0; i < sliceLen; i++ {
		elem, err := getConcreteValue(v.Index(i))
		if err != nil {
			return nil, fmt.Errorf("invalid element at index %d: %w", i, err)
		}
		newSlice.Index(i).Set(elem)
	}

	return newSlice.Interface(), nil
}
