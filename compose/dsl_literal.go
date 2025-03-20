package compose

import (
	"reflect"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

var passthrough = &ImplMeta{
	TypeID:        "Passthrough",
	ComponentType: ComponentOfPassthrough,
}

var implMap = map[string]*ImplMeta{
	"Passthrough": passthrough,
	"prompt.DefaultChatTemplate": {
		TypeID:        "prompt.DefaultChatTemplate",
		ComponentType: components.ComponentOfPrompt,
	},
	"lambda.MessagePtrToList": {
		ComponentType: ComponentOfLambda,
		Lambda:        func() *Lambda { return ToList[*schema.Message]() },
	},
	"*compose.ToolsNode": {
		TypeID:        "*compose.ToolsNode",
		ComponentType: ComponentOfToolsNode,
	},
}

var comp2AddFn = map[components.Component]reflect.Value{
	components.ComponentOfPrompt:    reflect.ValueOf((*graph).AddChatTemplateNode),
	components.ComponentOfRetriever: reflect.ValueOf((*graph).AddRetrieverNode),
	ComponentOfToolsNode:            reflect.ValueOf((*graph).AddToolsNode),
}

var comp2WorkflowAddFn = map[components.Component]reflect.Value{
	components.ComponentOfPrompt:    reflect.ValueOf((*Workflow[any, any]).AddChatTemplateNode),
	components.ComponentOfRetriever: reflect.ValueOf((*Workflow[any, any]).AddRetrieverNode),
	ComponentOfToolsNode:            reflect.ValueOf((*Workflow[any, any]).AddToolsNode),
}

const (
	TypeIDString     = "string"
	TypeIDBool       = "bool"
	TypeIDInt        = "int"
	TypeIDInt8       = "int8"
	TypeIDInt16      = "int16"
	TypeIDInt32      = "int32"
	TypeIDInt64      = "int64"
	TypeIDUint       = "uint"
	TypeIDUint8      = "uint8"
	TypeIDUint16     = "uint16"
	TypeIDUint32     = "uint32"
	TypeIDUint64     = "uint64"
	TypeIDFloat32    = "float32"
	TypeIDFloat64    = "float64"
	TypeIDStringPtr  = "*string"
	TypeIDBoolPtr    = "*bool"
	TypeIDIntPtr     = "*int"
	TypeIDInt8Ptr    = "*int8"
	TypeIDInt16Ptr   = "*int16"
	TypeIDInt32Ptr   = "*int32"
	TypeIDInt64Ptr   = "*int64"
	TypeIDUintPtr    = "*uint"
	TypeIDUint8Ptr   = "*uint8"
	TypeIDUint16Ptr  = "*uint16"
	TypeIDUint32Ptr  = "*uint32"
	TypeIDUint64Ptr  = "*uint64"
	TypeIDFloat32Ptr = "*float32"
	TypeIDFloat64Ptr = "*float64"
	TypeIDCtx        = "ctx"
	TypeIDError      = "error"
)

var typeMap = map[TypeID]*TypeMeta{
	TypeIDString: {
		ID:        TypeIDString,
		BasicType: BasicTypeString,
	},
	TypeIDBool: {
		ID:        TypeIDBool,
		BasicType: BasicTypeBool,
	},
	TypeIDCtx: {
		ID: TypeIDCtx,
	},
	TypeIDError: {
		ID: TypeIDError,
	},
	TypeIDFloat64: {
		ID:        TypeIDFloat64,
		BasicType: BasicTypeNumber,
		FloatType: generic.PtrOf(FloatTypeFloat64),
	},
	"schema.MessageTemplate": {
		ID:        "schema.MessageTemplate",
		BasicType: BasicTypeInterface, // this is a slot
	},
	"schema.Format": {
		ID:          "schema.Format",
		BasicType:   BasicTypeInteger,
		IntegerType: generic.PtrOf(IntegerTypeUint8),
		ReflectType: generic.PtrOf(reflect.TypeOf(schema.FormatType(0))),
	},
	"schema.messagePlaceholder": {
		ID:                "schema.messagePlaceholder",
		BasicType:         BasicTypeStruct,
		IsPtr:             true,
		InterfaceType:     (*TypeID)(generic.PtrOf("schema.MessageTemplate")), // can be used to fill the slot with schema.MessageTemplate
		InstantiationType: InstantiationTypeFunction,
		FunctionMeta: &FunctionMeta{
			Name:      "schema.MessagesPlaceholder",
			FuncValue: reflect.ValueOf(schema.MessagesPlaceholder),
			InputTypes: []TypeID{
				TypeIDString,
				TypeIDBool,
			},
			OutputTypes: []TypeID{
				"schema.MessageTemplate",
			},
		},
	},
	"*schema.Message": {
		ID:                "*schema.Message",
		BasicType:         BasicTypeStruct,
		IsPtr:             true,
		InstantiationType: InstantiationTypeUnmarshal,
		InterfaceType:     (*TypeID)(generic.PtrOf("schema.MessageTemplate")), // can be used to fill the slot with schema.MessageTemplate
		ReflectType:       generic.PtrOf(reflect.TypeOf(&schema.Message{})),
	},
	"prompts.ChatTemplate": {
		ID:        "prompt.ChatTemplate",
		BasicType: BasicTypeInterface,
	},
	"prompt.DefaultChatTemplate": {
		ID:                "prompt.DefaultChatTemplate",
		BasicType:         BasicTypeStruct,
		IsPtr:             true,
		InterfaceType:     (*TypeID)(generic.PtrOf("prompts.ChatTemplate")),
		InstantiationType: InstantiationTypeFunction,
		FunctionMeta: &FunctionMeta{
			Name:       "prompt.FromMessages",
			FuncValue:  reflect.ValueOf(prompt.FromMessages),
			IsVariadic: true,
			InputTypes: []TypeID{
				"schema.Format",
				// we can know this is an interface type, which is a slot
				"schema.MessageTemplate",
			},
			OutputTypes: []TypeID{
				"prompt.ChatTemplate",
			},
		},
	},
	"retriever.Retriever": {
		ID:        "retriever.Retriever",
		BasicType: BasicTypeInterface,
	},
	"embedding.Embedder": {
		ID:        "embedding.Embedder",
		BasicType: BasicTypeInterface,
	},
	"map[string]any": {
		ID:                "map[string]any",
		BasicType:         BasicTypeMap,
		InstantiationType: InstantiationTypeUnmarshal,
		ReflectType:       generic.PtrOf(reflect.TypeOf(map[string]any{})),
	},
	"*compose.ToolsNode": {
		ID:                "*compose.ToolsNode",
		BasicType:         BasicTypeStruct,
		InstantiationType: InstantiationTypeFunction,
		FunctionMeta: &FunctionMeta{
			Name:      "compose.NewToolNode",
			FuncValue: reflect.ValueOf(NewToolNode),
			InputTypes: []TypeID{
				TypeIDCtx,
				"*compose.ToolsNodeConfig",
			},
			OutputTypes: []TypeID{
				"*compose.ToolsNode",
				TypeIDError,
			},
		},
	},
	"*compose.ToolsNodeConfig": {
		ID:                "*compose.ToolsNodeConfig",
		BasicType:         BasicTypeStruct,
		InstantiationType: InstantiationTypeUnmarshal,
		ReflectType:       generic.PtrOf(reflect.TypeOf(&ToolsNodeConfig{})),
	},
	"tool.BaseTool": {
		ID:        "tool.BaseTool",
		BasicType: BasicTypeInterface,
	},
}

var branchFunctionMap = map[BranchFunctionID]*BranchFunction{}

func anyConvert[T any](a any) (T, error) {
	t, ok := a.(T)
	if !ok {
		return t, newUnexpectedInputTypeErr(reflect.TypeOf(t), reflect.TypeOf(a))
	}
	return t, nil
}

var stateHandlerMap = map[StateHandlerID]*StateHandler{}
