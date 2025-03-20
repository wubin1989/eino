package compose

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

type GraphDSL struct {
	ID              string            `json:"id" yaml:"id"`
	Namespace       string            `json:"namespace" yaml:"namespace"`
	Name            *string           `json:"name,omitempty" yaml:"name,omitempty"`
	InputType       TypeID            `json:"input_type" yaml:"input_type"`
	StateType       *TypeID           `json:"state_type,omitempty" yaml:"state_type,omitempty"`
	NodeTriggerMode *NodeTriggerMode  `json:"node_trigger_mode,omitempty" yaml:"node_trigger_mode,omitempty"`
	MaxRunStep      *int              `json:"max_run_step,omitempty" yaml:"max_run_step,omitempty"`
	Nodes           []*NodeDSL        `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Edges           []*EdgeDSL        `json:"edges,omitempty" yaml:"edges,omitempty"`
	Branches        []*GraphBranchDSL `json:"branches,omitempty" yaml:"branches,omitempty"`
}

type WorkflowDSL struct {
	ID              string                  `json:"id" yaml:"id"`
	Namespace       string                  `json:"namespace" yaml:"namespace"`
	Name            *string                 `json:"name" yaml:"name"`
	InputType       TypeID                  `json:"input_type" yaml:"input_type"`
	OutputType      TypeID                  `json:"output_type" yaml:"output_type"`
	StateType       *TypeID                 `json:"state_type,omitempty" yaml:"state_type,omitempty"`
	Nodes           []*WorkflowNodeDSL      `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Branches        []*WorkflowBranchDSL    `json:"branches,omitempty" yaml:"branches,omitempty"`
	EndInputs       []*WorkflowNodeInputDSL `json:"end_inputs,omitempty" yaml:"end_inputs,omitempty"`
	EndDependencies []string                `json:"end_dependencies,omitempty" yaml:"end_dependencies,omitempty"`
}

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

type TypeID string

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
	BasicType         BasicType         `json:"basic_type" yaml:"basic_type"`
	IsPtr             bool              `json:"is_ptr" yaml:"is_ptr"`
	IntegerType       *IntegerType      `json:"integer_type,omitempty" yaml:"integer_type,omitempty"`
	FloatType         *FloatType        `json:"float_type,omitempty" yaml:"float_type,omitempty"`
	InterfaceType     *TypeID           `json:"interface_type,omitempty" yaml:"interface_type,omitempty"`
	InstantiationType InstantiationType `json:"instantiation_type,omitempty" yaml:"instantiation_type,omitempty"`
	ReflectType       *reflect.Type     `json:"-" yaml:"-"`
	FunctionMeta      *FunctionMeta     `json:"function_meta,omitempty" yaml:"function_meta,omitempty"`
}

type FieldMeta struct {
	TypeMeta
	Name string `json:"name" yaml:"name"`
}

type StructMeta struct {
	Fields []*FieldMeta `json:"fields" yaml:"fields"`
	Name   string       `json:"name" yaml:"name"`
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

type InstantiationType string

const (
	InstantiationTypeLiteral   InstantiationType = "literal"
	InstantiationTypeFunction  InstantiationType = "function"
	InstantiationTypeUnmarshal InstantiationType = "unmarshal"
)

type NodeDSL struct {
	Key              string          `json:"key" yaml:"key"`
	ImplID           string          `json:"impl_id,omitempty" yaml:"impl_id,omitempty"`
	Name             *string         `json:"name,omitempty" yaml:"name,omitempty"`
	Configs          []Config        `json:"configs,omitempty" yaml:"configs,omitempty"`
	InputKey         *string         `json:"input_key,omitempty" yaml:"input_key,omitempty"`
	OutputKey        *string         `json:"output_key,omitempty" yaml:"output_key,omitempty"`
	GraphDSL         *GraphDSL       `json:"graph_dsl,omitempty" yaml:"graph_dsl,omitempty"`
	WorkflowDSL      *WorkflowDSL    `json:"workflow_dsl,omitempty" yaml:"workflow_dsl,omitempty"`
	StatePreHandler  *StateHandlerID `json:"state_pre_handler,omitempty" yaml:"state_pre_handler,omitempty"`
	StatePostHandler *StateHandlerID `json:"state_post_handler,omitempty" yaml:"state_post_handler,omitempty"`
}

type Config struct {
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
	Slot  *Slot  `json:"slot,omitempty" yaml:"slot,omitempty"`
	Slots []Slot `json:"slots,omitempty" yaml:"slots,omitempty"`
	isCtx bool
}

type Slot struct {
	TypeID TypeID `json:"type_id" yaml:"type_id"` // the actual type ID of the slot instance. BasicType should not be interface

	Path FieldPath `json:"path,omitempty" yaml:"path,omitempty"`

	Config  *string  `json:"config,omitempty" yaml:"config,omitempty"`
	Configs []Config `json:"configs,omitempty" yaml:"configs,omitempty"`
	Slots   []Slot   `json:"slots,omitempty" yaml:"slots,omitempty"` // nested slots
}

type EdgeDSL struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

type BranchDSL struct {
	Condition BranchFunctionID `json:"condition" yaml:"condition"`
	EndNodes  []string         `json:"end_nodes" yaml:"end_nodes"`
}

type BranchFunctionID string

type BranchFunction struct {
	ID              BranchFunctionID
	FuncValue       reflect.Value
	InputType       reflect.Type
	IsStream        bool
	StreamConverter StreamConverter
}

type GraphBranchDSL struct {
	*BranchDSL `json:"branch" yaml:"branch"`
	FromNode   string `json:"from_node" yaml:"from_node"`
}

type StateHandlerID string
type StateHandler struct {
	ID              StateHandlerID
	FuncValue       reflect.Value
	InputType       reflect.Type
	StateType       reflect.Type
	IsStream        bool
	StreamConverter StreamConverter
}

type WorkflowNodeDSL struct {
	*NodeDSL     `json:"node" yaml:"node"`
	Inputs       []*WorkflowNodeInputDSL `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Dependencies []string                `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	StaticValues []StaticValue           `json:"static_value,omitempty" yaml:"static_value,omitempty"`
}

type StaticValue struct {
	TypeID TypeID    `json:"type_id" yaml:"type_id"`
	Path   FieldPath `json:"path" yaml:"path"`
	Value  string    `json:"value" yaml:"value"`
}

type WorkflowNodeInputDSL struct {
	FromNodeKey        string             `json:"from_node_key" yaml:"from_node_key"`
	FieldPathMappings  []FieldPathMapping `json:"field_mappings,omitempty" yaml:"field_mappings,omitempty"`
	NoDirectDependency bool               `json:"no_direct_dependency,omitempty" yaml:"no_direct_dependency,omitempty"`
}

type FieldPathMapping struct {
	From FieldPath `json:"from" yaml:"from"`
	To   FieldPath `json:"to" yaml:"to"`
}

type WorkflowBranchDSL struct {
	Key          string `json:"key" yaml:"key"`
	*BranchDSL   `json:"branch" yaml:"branch"`
	Inputs       []*WorkflowNodeInputDSL `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Dependencies []string                `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
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
