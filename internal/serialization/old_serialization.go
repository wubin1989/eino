/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package serialization

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/bytedance/sonic"
)

func oldUnmarshal(data []byte) (any, error) {
	is := &oldInternalStruct{}
	err := sonic.Unmarshal(data, is)
	if err != nil {
		return nil, err
	}
	return oldInternalUnmarshal(is)
}

type oldInternalStruct struct {
	PointerNum uint32 `json:",omitempty"`

	// based type
	Type      string          `json:",omitempty"`
	JSONValue json.RawMessage `json:",omitempty"`

	// struct type
	StructType string `json:",omitempty"`
	// map key type
	MapKeyPointerNum uint32 `json:",omitempty"`
	MapKeyType       string `json:",omitempty"`
	// map value type
	MapValuePointerNum uint32 `json:",omitempty"`
	MapValueType       string `json:",omitempty"`
	// map or struct
	// in map, the key is the serialized map key anyway todo: if key is string, don't serialize
	// in struct, the key is the original field name
	MapValues map[string]*oldInternalStruct `json:",omitempty"`

	// slice value type
	SliceValuePointerNum uint32               `json:",omitempty"`
	SliceValueType       string               `json:",omitempty"`
	SliceValues          []*oldInternalStruct `json:",omitempty"`
}

func oldInternalUnmarshal(v *oldInternalStruct) (any, error) {
	if v == nil {
		return nil, nil
	}

	if len(v.Type) != 0 {
		// based type
		t, ok := m[v.Type]
		if !ok {
			return nil, fmt.Errorf("unknown type key: %v", v.Type)
		}
		pResult := reflect.New(resolvePointerNum(v.PointerNum, t))
		err := sonic.Unmarshal(v.JSONValue, pResult.Interface())
		if err != nil {
			return nil, fmt.Errorf("unmarshal type[%s] fail: %v, data: %s", v.Type, err, string(v.JSONValue))
		}
		return pResult.Elem().Interface(), nil
	}

	if len(v.StructType) > 0 {
		// struct
		rt, ok := m[v.StructType]
		if !ok {
			return nil, fmt.Errorf("unknown type key: %v", v.StructType)
		}
		result, dResult := createValueFromType(resolvePointerNum(v.PointerNum, rt))

		// todo: if all fields are based, can use unmarshal instead of internalUnmarshal
		for k, internalValue := range v.MapValues {
			value, err := oldInternalUnmarshal(internalValue)
			if err != nil {
				return nil, fmt.Errorf("unmarshal map field[%v] fail: %v", k, err)
			}
			field := dResult.FieldByName(k)
			if !field.CanSet() {
				return nil, fmt.Errorf("unmarshal map fail, can not set field %v", k)
			}
			if value == nil {
				rft, ok := rt.FieldByName(k)
				if !ok {
					return nil, fmt.Errorf("unmarshal map fail, cannot find field: %v", k)
				}
				field.Set(reflect.New(rft.Type).Elem())
			} else {
				field.Set(reflect.ValueOf(value))
			}
		}

		return result.Interface(), nil
	}

	if len(v.MapKeyType) > 0 {
		// map
		rkt, ok := m[v.MapKeyType]
		if !ok {
			return nil, fmt.Errorf("unknown type: %v", v.MapKeyType)
		}
		rkt = resolvePointerNum(v.MapKeyPointerNum, rkt)
		rvt, ok := m[v.MapValueType]
		if !ok {
			return nil, fmt.Errorf("unknown type: %v", v.MapValueType)
		}
		rvt = resolvePointerNum(v.MapValuePointerNum, rvt)

		// todo: if all values are based, can use unmarshal instead of internalUnmarshal
		result, dResult := createValueFromType(reflect.MapOf(rkt, rvt))
		for marshaledMapKey, internalValue := range v.MapValues {
			prkv := reflect.New(rkt)
			err := sonic.UnmarshalString(marshaledMapKey, prkv.Interface())
			if err != nil {
				return nil, fmt.Errorf("unmarshal map key[%v] to type[%s] fail: %v", marshaledMapKey, v.MapKeyType, err)
			}

			value, err := oldInternalUnmarshal(internalValue)
			if err != nil {
				return nil, fmt.Errorf("unmarshal map value fail: %v", err)
			}
			if value == nil {
				dResult.SetMapIndex(prkv.Elem(), reflect.New(rvt).Elem())
			} else {
				dResult.SetMapIndex(prkv.Elem(), reflect.ValueOf(value))
			}
		}
		return result.Interface(), nil
	}

	// slice
	rvt, ok := m[v.SliceValueType]
	if !ok {
		return nil, fmt.Errorf("unknown type: %v", v.SliceValueType)
	}
	rvt = resolvePointerNum(v.SliceValuePointerNum, rvt)

	// todo: if all slice values are based, can use unmarshal instead of internalUnmarshal
	result, dResult := createValueFromType(reflect.SliceOf(rvt))
	for _, internalValue := range v.SliceValues {
		value, err := oldInternalUnmarshal(internalValue)
		if err != nil {
			return nil, fmt.Errorf("unmarshal slice[%s] fail: %v", v.SliceValueType, err)
		}
		if value == nil {
			// empty value
			dResult.Set(reflect.Append(dResult, reflect.New(rvt).Elem()))
		} else {
			dResult.Set(reflect.Append(dResult, reflect.ValueOf(value)))
		}
	}
	return result.Interface(), nil
}
