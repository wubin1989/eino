package compose

import "github.com/cloudwego/eino/components"

type GraphDSL struct {
	ID              string           `json:"id"`
	Namespace       string           `json:"namespace"`
	Name            *string          `json:"name,omitempty"`
	InputType       TypeMeta         `json:"input_type"`
	OutputType      TypeMeta         `json:"output_type"`
	StateType       *TypeMeta        `json:"state_type,omitempty"`
	NodeTriggerMode *NodeTriggerMode `json:"node_trigger_mode,omitempty"`
	MaxRunStep      *int             `json:"max_run_step,omitempty"`
	Nodes           []*NodeDSL       `json:"nodes,omitempty"`
	Edges           []*EdgeDSL       `json:"edges,omitempty"`
	Branches        []*BranchDSL     `json:"branches,omitempty"`
}

type WorkflowDSL struct {
	ID              string                  `json:"id"`
	Namespace       string                  `json:"namespace"`
	Name            string                  `json:"name"`
	InputType       TypeMeta                `json:"input_type"`
	OutputType      TypeMeta                `json:"output_type"`
	StateType       *TypeMeta               `json:"state_type,omitempty"`
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
	BasicTypeFunction  BasicType = "function"
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

type ArrayType string

const (
	ArrayTypeString      ArrayType = "string"
	ArrayTypeMessagePtr  ArrayType = "*Message"
	ArrayTypeAny         ArrayType = "any"
	ArrayTypeDocumentPtr ArrayType = "*Document"
)

type MapType string

const (
	MapTypeStringAny MapType = "map[string]any"
)

type StructType string

const (
	StructTypeMessage  StructType = "Message"
	StructTypeDocument StructType = "Document"
)

type InterfaceType string

const (
	InterfaceTypeAny                 InterfaceType = "any"
	InterfaceTypeChatModel           InterfaceType = "ChatModel"
	InterfaceTypeChatTemplate        InterfaceType = "ChatTemplate"
	InterfaceTypeDocumentTransformer InterfaceType = "DocumentTransformer"
	InterfaceTypeDocumentLoader      InterfaceType = "DocumentLoader"
	InterfaceTypeEmbedding           InterfaceType = "Embedding"
	InterfaceTypeIndexer             InterfaceType = "Indexer"
	InterfaceTypeRetriever           InterfaceType = "Retriever"
	InterfaceTypeTool                InterfaceType = "Tool"
)

// TypeMeta is the metadata of a type.
// 使用场景：
// 1. 作为 Workflow 整体的输入或输出类型
// 2. 作为 Component interface 的输入或输出类型
// 3. 作为 Component implementation 的 Config 类型
// 4. 作为 State 的类型
// 5. 作为 Lambda 的输入或输出类型
type TypeMeta struct {
	ID            string         `json:"id"`
	Version       *string        `json:"version,omitempty"` // TODO: how to define version?
	BasicType     BasicType      `json:"basic_type"`
	IsPtr         bool           `json:"is_ptr"`
	IntegerType   *IntegerType   `json:"integer_type,omitempty"`
	FloatType     *FloatType     `json:"float_type,omitempty"`
	ArrayType     *ArrayType     `json:"array_type,omitempty"`
	MapType       *MapType       `json:"map_type,omitempty"`
	StructType    *StructType    `json:"struct_type,omitempty"`
	InterfaceType *InterfaceType `json:"interface_type,omitempty"`
}

type FieldMeta struct {
	TypeMeta
	Name string `json:"name"`
}

type StructMeta struct {
	Fields []*FieldMeta `json:"fields"`
	Name   string       `json:"name"`
}

type NodeDSL struct {
	Key                    string               `json:"key"`
	ComponentType          components.Component `json:"component_type"`
	InputType              *TypeMeta            `json:"input_type,omitempty"`
	OutputType             *TypeMeta            `json:"output_type,omitempty"`
	ImplementationType     *TypeMeta            `json:"implementation_type,omitempty"`
	ConfigType             *TypeMeta            `json:"config_type,omitempty"`
	InstantiationFunction  *TypeMeta            `json:"instantiation_function,omitempty"`
	Name                   *string              `json:"name,omitempty"`
	InputKey               *string              `json:"input_key,omitempty"`
	OutputKey              *string              `json:"output_key,omitempty"`
	GraphDSL               *GraphDSL            `json:"graph_dsl,omitempty"`
	StatePreHandler        *TypeMeta            `json:"state_pre_handler,omitempty"`
	StatePostHandler       *TypeMeta            `json:"state_post_handler,omitempty"`
	StreamStatePreHandler  *TypeMeta            `json:"stream_state_pre_handler,omitempty"`
	StreamStatePostHandler *TypeMeta            `json:"stream_state_post_handler,omitempty"`
}

type EdgeDSL struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type BranchDSL struct {
	FromNodes string   `json:"from_nodes"`
	EndNodes  string   `json:"end_nodes"`
	Condition TypeMeta `json:"condition"`
}

type WorkflowNodeDSL struct {
	NodeDSL
	Inputs       []*WorkflowNodeInputDSL `json:"inputs,omitempty"`
	Dependencies []string                `json:"dependencies,omitempty"`
}

type WorkflowNodeInputDSL struct {
	FromNodeKey        string   `json:"from_node_key"`
	FromField          string   `json:"from_field,omitempty"`
	ToField            string   `json:"to_field,omitempty"`
	FromFieldPath      []string `json:"from_field_path,omitempty"`
	ToFieldPath        []string `json:"to_field_path,omitempty"`
	NoDirectDependency bool     `json:"no_direct_dependency,omitempty"`
}

type WorkflowBranchDSL struct {
	Key string `json:"key"`
	*BranchDSL
	Inputs       []*WorkflowNodeInputDSL `json:"inputs,omitempty"`
	Dependencies []string                `json:"dependencies,omitempty"`
}
