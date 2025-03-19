package compose

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudwego/eino/internal/generic"
)

func TestAssignSlot(t *testing.T) {
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
		path    FieldPath
		value   interface{}
		wantErr bool
	}{
		{
			name:    "simple string field",
			target:  &TestStruct{},
			path:    FieldPath{"String"},
			value:   "test",
			wantErr: false,
		},
		{
			name:    "string pointer field",
			target:  &TestStruct{},
			path:    FieldPath{"StringPtr"},
			value:   new(string),
			wantErr: false,
		},
		{
			name:    "slice index",
			target:  &TestStruct{},
			path:    FieldPath{"Slice", "[0]"},
			value:   "test",
			wantErr: false,
		},
		{
			name:    "slice pointer index",
			target:  &TestStruct{},
			path:    FieldPath{"SlicePtr", "[0]"},
			value:   generic.PtrOf("test"),
			wantErr: false,
		},
		{
			name:    "map field",
			target:  &TestStruct{},
			path:    FieldPath{"Map", "key"},
			value:   "test",
			wantErr: false,
		},
		{
			name:    "map pointer field",
			target:  &TestStruct{},
			path:    FieldPath{"MapPtr", "key"},
			value:   generic.PtrOf("test"),
			wantErr: false,
		},
		{
			name:    "nested struct field",
			target:  &TestStruct{},
			path:    FieldPath{"Nested", "Value"},
			value:   "test",
			wantErr: false,
		},
		{
			name:    "nested pointer struct field",
			target:  &TestStruct{},
			path:    FieldPath{"NestedPtr", "Value"},
			value:   "test",
			wantErr: false,
		},
		{
			name:    "map of structs",
			target:  &TestStruct{},
			path:    FieldPath{"StructMap", "key", "Value"},
			value:   "test",
			wantErr: false,
		},
		{
			name:    "map of struct pointers",
			target:  &TestStruct{},
			path:    FieldPath{"PtrMap", "key", "Value"},
			value:   "test",
			wantErr: false,
		},
		{
			name:    "slice of structs",
			target:  &TestStruct{},
			path:    FieldPath{"SliceNest", "[0]", "Value"},
			value:   "test",
			wantErr: false,
		},
		{
			name:    "slice of struct pointers",
			target:  &TestStruct{},
			path:    FieldPath{"SlicePtrNest", "[0]", "Value"},
			value:   "test",
			wantErr: false,
		},
		{
			name:   "top level slice",
			target: []Nested{},
			path:   FieldPath{"[1]"},
			value: Nested{
				Value: "test",
			},
			wantErr: false,
		},
		// Error cases
		{
			name:    "empty path",
			target:  &TestStruct{},
			path:    FieldPath{},
			value:   "test",
			wantErr: true,
		},
		{
			name:    "invalid path",
			target:  &TestStruct{},
			path:    FieldPath{"InvalidField"},
			value:   "test",
			wantErr: true,
		},
		{
			name:    "invalid array index",
			target:  &TestStruct{},
			path:    FieldPath{"Slice", "[invalid]"},
			value:   "test",
			wantErr: true,
		},
		{
			name:    "path to non-struct",
			target:  &TestStruct{},
			path:    FieldPath{"String.Invalid"},
			value:   "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetValue := newInstanceByType(reflect.TypeOf(tt.target))

			_, err := assignSlot(targetValue, tt.value, tt.path)

			if (err != nil) != tt.wantErr {
				t.Errorf("assignSlot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the value was set correctly
				got, err := getValueByPath(targetValue, tt.path)
				if err != nil {
					t.Errorf("Failed to get value after setting: %v", err)
					return
				}

				gotInter := got.Interface()

				if !reflect.DeepEqual(gotInter, tt.value) {
					t.Errorf("Value not set correctly. got = %v, want = %v", gotInter, tt.value)
				}
			}
		})
	}
}

// Helper function to get value by path for verification
func getValueByPath(v reflect.Value, segments FieldPath) (reflect.Value, error) {
	current := v

	for _, segment := range segments {
		// Dereference pointers until we get to the actual value
		for current.Kind() == reflect.Ptr {
			if current.IsNil() {
				return reflect.Value{}, fmt.Errorf("nil pointer encountered")
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
					if !current.IsValid() {
						return reflect.Value{}, fmt.Errorf("key not found in map: %s", field)
					}
					// Always dereference pointers when reading
					for current.Kind() == reflect.Ptr && !current.IsNil() {
						current = current.Elem()
					}
				} else {
					current = current.FieldByName(field)
					if !current.IsValid() {
						return reflect.Value{}, fmt.Errorf("field not found: %s", field)
					}
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
			if !current.IsValid() {
				return reflect.Value{}, fmt.Errorf("key not found in map: %s", segment)
			}
		} else if current.Kind() == reflect.Struct {
			current = current.FieldByName(segment)
			if !current.IsValid() {
				return reflect.Value{}, fmt.Errorf("field not found: %s", segment)
			}
		} else {
			return reflect.Value{}, fmt.Errorf("expected struct or map at %s, got %v", segment, current.Kind())
		}
	}

	return current, nil
}
