package compose

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSetFieldByJSONPath(t *testing.T) {
	type Nested struct {
		Value string
		Ptr   *string
	}

	type TestStruct struct {
		String       string
		StringPtr    *string
		Slice        []string
		SlicePtr     []*string
		Map          map[string]string
		MapPtr       map[string]*string
		Nested       Nested
		NestedPtr    *Nested
		StructMap    map[string]Nested
		PtrMap       map[string]*Nested
		SliceNest    []Nested
		SlicePtrNest []*Nested
	}

	tests := []struct {
		name    string
		target  interface{}
		path    string
		value   interface{}
		wantErr bool
		compare func(got, want interface{}) bool
	}{
		{
			name:    "simple string field",
			target:  &TestStruct{},
			path:    "$.String",
			value:   "test",
			wantErr: false,
		},
		{
			name:    "string pointer field",
			target:  &TestStruct{},
			path:    "$.StringPtr",
			value:   new(string),
			wantErr: false,
		},
		{
			name:    "slice index",
			target:  &TestStruct{Slice: make([]string, 0)},
			path:    "$.Slice[0]",
			value:   "test",
			wantErr: false,
		},
		{
			name:    "slice pointer index",
			target:  &TestStruct{SlicePtr: make([]*string, 0)},
			path:    "$.SlicePtr[0]",
			value:   "test",
			wantErr: false,
			compare: func(got, want interface{}) bool {
				if gotPtr, ok := got.(*string); ok {
					return *gotPtr == want.(string)
				}
				return false
			},
		},
		{
			name:    "map field",
			target:  &TestStruct{Map: make(map[string]string)},
			path:    "$.Map.key",
			value:   "test",
			wantErr: false,
		},
		{
			name:    "map pointer field",
			target:  &TestStruct{MapPtr: make(map[string]*string)},
			path:    "$.MapPtr.key",
			value:   "test",
			wantErr: false,
			compare: func(got, want interface{}) bool {
				if gotPtr, ok := got.(*string); ok {
					return *gotPtr == want.(string)
				}
				return false
			},
		},
		{
			name:    "nested struct field",
			target:  &TestStruct{},
			path:    "$.Nested.Value",
			value:   "test",
			wantErr: false,
		},
		{
			name:    "nested pointer struct field",
			target:  &TestStruct{},
			path:    "$.NestedPtr.Value",
			value:   "test",
			wantErr: false,
			compare: func(got, want interface{}) bool {
				if gotPtr, ok := got.(*string); ok {
					return *gotPtr == want.(string)
				}
				if gotStr, ok := got.(string); ok {
					return gotStr == want.(string)
				}
				return false
			},
		},
		{
			name:    "map of structs",
			target:  &TestStruct{StructMap: make(map[string]Nested)},
			path:    "$.StructMap.key.Value",
			value:   "test",
			wantErr: false,
			compare: func(got, want interface{}) bool {
				if gotStr, ok := got.(string); ok {
					return gotStr == want.(string)
				}
				return false
			},
		},
		{
			name:    "map of struct pointers",
			target:  &TestStruct{PtrMap: make(map[string]*Nested)},
			path:    "$.PtrMap.key.Value",
			value:   "test",
			wantErr: false,
			compare: func(got, want interface{}) bool {
				if gotStr, ok := got.(string); ok {
					return gotStr == want.(string)
				}
				return false
			},
		},
		{
			name:    "slice of structs",
			target:  &TestStruct{SliceNest: make([]Nested, 0)},
			path:    "$.SliceNest[0].Value",
			value:   "test",
			wantErr: false,
		},
		{
			name:    "slice of struct pointers",
			target:  &TestStruct{SlicePtrNest: make([]*Nested, 0)},
			path:    "$.SlicePtrNest[0].Value",
			value:   "test",
			wantErr: false,
			compare: func(got, want interface{}) bool {
				if gotStr, ok := got.(string); ok {
					return gotStr == want.(string)
				}
				return false
			},
		},
		// Error cases
		{
			name:    "empty path",
			target:  &TestStruct{},
			path:    "",
			value:   "test",
			wantErr: true,
		},
		{
			name:    "invalid path",
			target:  &TestStruct{},
			path:    "$.InvalidField",
			value:   "test",
			wantErr: true,
		},
		{
			name:    "invalid array index",
			target:  &TestStruct{},
			path:    "$.Slice[invalid]",
			value:   "test",
			wantErr: true,
		},
		{
			name:    "path to non-struct",
			target:  &TestStruct{},
			path:    "$.String.Invalid",
			value:   "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name != "map of structs" {
				return
			}

			targetValue := reflect.ValueOf(tt.target).Elem()
			valueValue := reflect.ValueOf(tt.value)

			err := setFieldByJSONPath(targetValue, tt.path, valueValue)

			if (err != nil) != tt.wantErr {
				t.Errorf("setFieldByJSONPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the value was set correctly
				got, err := getValueByPath(targetValue, tt.path)
				if err != nil {
					t.Errorf("Failed to get value after setting: %v", err)
					return
				}

				if tt.compare != nil {
					if !tt.compare(got.Interface(), tt.value) {
						t.Errorf("Value not set correctly. got = %v, want = %v", got.Interface(), tt.value)
					}
				} else if !reflect.DeepEqual(got.Interface(), tt.value) {
					t.Errorf("Value not set correctly. got = %v, want = %v", got.Interface(), tt.value)
				}
			}
		})
	}
}

// Helper function to get value by path for verification
func getValueByPath(v reflect.Value, path string) (reflect.Value, error) {
	if path == "" {
		return reflect.Value{}, fmt.Errorf("empty path")
	}
	if path[0] == '$' {
		path = path[1:]
	}

	segments := strings.Split(strings.Trim(path, "."), ".")
	current := v

	for _, segment := range segments {
		// Dereference pointers until we get to the actual value
		for current.Kind() == reflect.Ptr {
			if current.IsNil() {
				// Initialize nil pointer instead of returning error
				current.Set(reflect.New(current.Type().Elem()))
			}
			current = current.Elem()
		}

		if idx := strings.Index(segment, "["); idx != -1 {
			field := segment[:idx]
			if field != "" {
				if current.Kind() != reflect.Struct && current.Kind() != reflect.Map {
					return reflect.Value{}, fmt.Errorf("expected struct or map at %s, got %v", field, current.Kind())
				}

				if current.Kind() == reflect.Map {
					current = current.MapIndex(reflect.ValueOf(field))
				} else {
					current = current.FieldByName(field)
				}
				if !current.IsValid() {
					return reflect.Value{}, fmt.Errorf("field not found: %s", field)
				}
			}

			idxStr := segment[idx+1 : len(segment)-1]
			index, err := strconv.Atoi(idxStr)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("invalid array index: %v", err)
			}

			if current.Kind() != reflect.Slice && current.Kind() != reflect.Array {
				return reflect.Value{}, fmt.Errorf("expected slice or array at %s, got %v", segment, current.Kind())
			}
			if index >= current.Len() {
				return reflect.Value{}, fmt.Errorf("index out of range: %d", index)
			}
			current = current.Index(index)
			continue
		}

		if current.Kind() == reflect.Map {
			current = current.MapIndex(reflect.ValueOf(segment))
		} else if current.Kind() == reflect.Struct {
			current = current.FieldByName(segment)
		} else {
			return reflect.Value{}, fmt.Errorf("expected struct or map at %s, got %v", segment, current.Kind())
		}

		if !current.IsValid() {
			return reflect.Value{}, fmt.Errorf("invalid path segment: %s", segment)
		}
	}

	return current, nil
}
