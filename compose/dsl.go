package compose

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/schema"
)

type GraphDSL struct {
	ID              string            `json:"id"`
	Namespace       string            `json:"namespace"`
	Name            *string           `json:"name,omitempty"`
	StateType       *TypeID           `json:"state_type,omitempty"`
	NodeTriggerMode *NodeTriggerMode  `json:"node_trigger_mode,omitempty"`
	MaxRunStep      *int              `json:"max_run_step,omitempty"`
	Nodes           []*NodeDSL        `json:"nodes,omitempty"`
	Edges           []*EdgeDSL        `json:"edges,omitempty"`
	Branches        []*GraphBranchDSL `json:"branches,omitempty"`
}

type WorkflowDSL struct {
	ID              string                  `json:"id"`
	Namespace       string                  `json:"namespace"`
	Name            *string                 `json:"name"`
	InputType       TypeID                  `json:"input_type"`
	OutputType      TypeID                  `json:"output_type"`
	StateType       *TypeID                 `json:"state_type,omitempty"`
	Nodes           []*WorkflowNodeDSL      `json:"nodes,omitempty"`
	Branches        []*WorkflowBranchDSL    `json:"branches,omitempty"`
	EndInputs       []*WorkflowNodeInputDSL `json:"end_inputs,omitempty"`
	EndDependencies []string                `json:"end_dependencies,omitempty"`
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
	ID                TypeID            `json:"id"`
	Version           *string           `json:"version,omitempty"` // TODO: how to define version?
	BasicType         BasicType         `json:"basic_type"`
	IsPtr             bool              `json:"is_ptr"`
	IntegerType       *IntegerType      `json:"integer_type,omitempty"`
	FloatType         *FloatType        `json:"float_type,omitempty"`
	InterfaceType     *TypeID           `json:"interface_type,omitempty"`
	InstantiationType InstantiationType `json:"instantiation_type,omitempty"`
	ReflectType       *reflect.Type     `json:"-"`
	FunctionMeta      *FunctionMeta     `json:"function_meta,omitempty"`
}

type FieldMeta struct {
	TypeMeta
	Name string `json:"name"`
}

type StructMeta struct {
	Fields []*FieldMeta `json:"fields"`
	Name   string       `json:"name"`
}

type FunctionMeta struct {
	Name        string        `json:"name"`
	FuncValue   reflect.Value `json:"-"`
	IsVariadic  bool          `json:"is_variadic,omitempty"`
	InputTypes  []TypeID      `json:"input_types,omitempty"`
	OutputTypes []TypeID      `json:"output_types,omitempty"`
}

type ImplMeta struct {
	TypeID        TypeID               `json:"type_id"`
	ComponentType components.Component `json:"component_type"`
	Lambda        func() *Lambda       `json:"-"`
}

type InstantiationType string

const (
	InstantiationTypeLiteral   InstantiationType = "literal"
	InstantiationTypeFunction  InstantiationType = "function"
	InstantiationTypeUnmarshal InstantiationType = "unmarshal"
)

type NodeDSL struct {
	Key                    string          `json:"key"`
	ImplID                 string          `json:"impl_id"`
	Name                   *string         `json:"name,omitempty"`
	Config                 *string         `json:"config,omitempty"`  // use when there is only one input parameter other than ctx
	Configs                []Config        `json:"configs,omitempty"` // use when there are multiple input parameters other than ctx
	Slots                  []Slot          `json:"slots,omitempty"`
	InputKey               *string         `json:"input_key,omitempty"`
	OutputKey              *string         `json:"output_key,omitempty"`
	GraphDSL               *GraphDSL       `json:"graph_dsl,omitempty"`
	WorkflowDSL            *WorkflowDSL    `json:"workflow_dsl,omitempty"`
	StatePreHandler        *StateHandlerID `json:"state_pre_handler,omitempty"`
	StatePostHandler       *StateHandlerID `json:"state_post_handler,omitempty"`
	StreamStatePreHandler  *StateHandlerID `json:"stream_state_pre_handler,omitempty"`
	StreamStatePostHandler *StateHandlerID `json:"stream_state_post_handler,omitempty"`
}

type Config struct {
	Index int    `json:"index"`
	Value string `json:"value"`
	Slots []Slot `json:"slots,omitempty"`
}

type Slot struct {
	TypeID TypeID `json:"type_id"` // the actual type ID of the slot instance. BasicType should not be interface

	Path FieldPath `json:"path"`

	Config  *string  `json:"config,omitempty"`
	Configs []Config `json:"configs,omitempty"`
	Slots   []Slot   `json:"slots,omitempty"` // nested slots
}

type EdgeDSL struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type BranchDSL struct {
	Condition BranchFunctionID
	EndNodes  []string
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
	*BranchDSL `json:"branch"`
	FromNode   string `json:"from_node"`
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
	*NodeDSL     `json:"node"`
	Inputs       []*WorkflowNodeInputDSL `json:"inputs,omitempty"`
	Dependencies []string                `json:"dependencies,omitempty"`
	StaticValues []StaticValue           `json:"static_value,omitempty"`
}

type StaticValue struct {
	TypeID TypeID    `json:"type_id"`
	Path   FieldPath `json:"path"`
	Value  string    `json:"value"`
}

type WorkflowNodeInputDSL struct {
	FromNodeKey        string             `json:"from_node_key"`
	FieldPathMappings  []FieldPathMapping `json:"field_mappings,omitempty"`
	NoDirectDependency bool               `json:"no_direct_dependency,omitempty"`
}

type FieldPathMapping struct {
	From FieldPath `json:"from"`
	To   FieldPath `json:"to"`
}

type WorkflowBranchDSL struct {
	Key          string `json:"key"`
	*BranchDSL   `json:"branch"`
	Inputs       []*WorkflowNodeInputDSL `json:"inputs,omitempty"`
	Dependencies []string                `json:"dependencies,omitempty"`
}

type StreamConverter interface {
	convertFromAny(in *schema.StreamReader[any]) (reflect.Value, error)
	packToAny(v reflect.Value) *schema.StreamReader[any]
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
