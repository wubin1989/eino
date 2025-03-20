package compose

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

type BasicType string

const (
	BasicTypeString    BasicType = "string"
	BasicTypeInteger   BasicType = "integer"
	BasicTypeNumber    BasicType = "number"
	BasicTypeBool      BasicType = "bool"
	BasicTypeStruct    BasicType = "struct"
	BasicTypeArray     BasicType = "array"
	BasicTypeMap       BasicType = "map"
	BasicTypeInterface BasicType = "interface"
)

type IntegerType string

const (
	IntegerTypeInt8   IntegerType = "int8"
	IntegerTypeInt16  IntegerType = "int16"
	IntegerTypeInt32  IntegerType = "int32"
	IntegerTypeInt64  IntegerType = "int64"
	IntegerTypeUint8  IntegerType = "uint8"
	IntegerTypeUint16 IntegerType = "uint16"
	IntegerTypeUint32 IntegerType = "uint32"
	IntegerTypeUint64 IntegerType = "uint64"
	IntegerTypeInt    IntegerType = "int"
	IntegerTypeUInt   IntegerType = "uint"
)

type FloatType string

const (
	FloatTypeFloat32 FloatType = "float32"
	FloatTypeFloat64 FloatType = "float64"
)

type InstantiationType string

const (
	InstantiationTypeFunction  InstantiationType = "function"
	InstantiationTypeUnmarshal InstantiationType = "unmarshal"
)

// TypeMeta is the metadata of a type.
// 使用场景：
// 1. 作为 Workflow 整体的输入或输出类型
// 2. 作为 Component interface 的输入或输出类型
// 3. 作为 Component implementation 的 Config 类型
// 4. 作为 State 的类型
// 5. 作为 Lambda 的输入或输出类型
type TypeMeta struct {
	ID                TypeID            `json:"id" yaml:"id"`
	Version           *string           `json:"version,omitempty" yaml:"version,omitempty"` // TODO: how to define version?
	BasicType         BasicType         `json:"basic_type,omitempty" yaml:"basic_type,omitempty"`
	IsPtr             bool              `json:"is_ptr,omitempty" yaml:"is_ptr,omitempty"`
	IntegerType       *IntegerType      `json:"integer_type,omitempty" yaml:"integer_type,omitempty"`
	FloatType         *FloatType        `json:"float_type,omitempty" yaml:"float_type,omitempty"`
	InterfaceType     *TypeID           `json:"interface_type,omitempty" yaml:"interface_type,omitempty"`
	InstantiationType InstantiationType `json:"instantiation_type,omitempty" yaml:"instantiation_type,omitempty"`
	ReflectType       *reflect.Type     `json:"-" yaml:"-"`
	FunctionMeta      *FunctionMeta     `json:"function_meta,omitempty" yaml:"function_meta,omitempty"`
}

type FunctionMeta struct {
	Name        string        `json:"name" yaml:"name"`
	FuncValue   reflect.Value `json:"-" yaml:"-"`
	IsVariadic  bool          `json:"is_variadic,omitempty" yaml:"is_variadic,omitempty"`
	InputTypes  []TypeID      `json:"input_types,omitempty" yaml:"input_types,omitempty"`
	OutputTypes []TypeID      `json:"output_types,omitempty" yaml:"output_types,omitempty"`
}

type ImplMeta struct {
	TypeID        TypeID               `json:"type_id" yaml:"type_id"`
	ComponentType components.Component `json:"component_type" yaml:"component_type"`
	Lambda        func() *Lambda       `json:"-" yaml:"-"`
}

type BranchFunction struct {
	ID              BranchFunctionID
	FuncValue       reflect.Value
	InputType       reflect.Type
	IsStream        bool
	StreamConverter StreamConverter
}

type StateHandler struct {
	ID              StateHandlerID
	FuncValue       reflect.Value
	InputType       reflect.Type
	StateType       reflect.Type
	IsStream        bool
	StreamConverter StreamConverter
}

type StreamConverter interface {
	convertFromAny(in *schema.StreamReader[any]) (reflect.Value, error)
	packToAny(v reflect.Value) *schema.StreamReader[any]
	inputType() reflect.Type
}

type StreamConverterImpl[T any] struct{}

func (sh *StreamConverterImpl[T]) convertFromAny(in *schema.StreamReader[any]) (reflect.Value, error) {
	f := reflect.ValueOf(schema.StreamReaderWithConvert[any, T])
	c := reflect.ValueOf(anyConvert[T])
	converted := f.Call([]reflect.Value{reflect.ValueOf(in), c})
	if len(converted) != 1 {
		return reflect.Value{}, fmt.Errorf("stream handler convert function return value length mismatch: given %d, defined %d", len(converted), 1)
	}

	return converted[0], nil
}

func (sh *StreamConverterImpl[T]) packToAny(v reflect.Value) *schema.StreamReader[any] {
	f := reflect.ValueOf(packStreamReader[T])
	return f.Call([]reflect.Value{v})[0].Interface().(streamReader).toAnyStreamReader()
}

func (sh *StreamConverterImpl[T]) inputType() reflect.Type {
	return generic.TypeOf[T]()
}
