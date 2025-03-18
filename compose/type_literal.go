package compose

import (
	"reflect"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

var defaultChatTemplateType = &ImplMeta{
	ID:            "DefaultChatTemplate",
	ComponentType: components.ComponentOfPrompt,
}

var passthrough = &ImplMeta{
	ID:            "Passthrough",
	ComponentType: ComponentOfPassthrough,
}

var ctxType = &TypeMeta{
	ID: "Context",
}

var errType = &TypeMeta{
	ID: "Error",
}

var formatType = &TypeMeta{
	ID:          "schema.Format",
	BasicType:   BasicTypeInteger,
	IntegerType: generic.PtrOf(IntegerTypeUint8),
}

var implMap = map[string]*ImplMeta{
	"Passthrough": passthrough,
	"prompt.DefaultChatTemplate": {
		ID:            "prompt.DefaultChatTemplate",
		ComponentType: components.ComponentOfPrompt,
		InstantiationFunction: &FunctionMeta{
			Name:       "prompt.FromMessages",
			FuncValue:  reflect.ValueOf(prompt.FromMessages),
			IsVariadic: true,
			InputTypes: []TypeID{
				"schema.Format",
				"*schema.Message", // dependencies only appear as fields or nested fields within Config, or as parameters to factory functions
			},
			OutputTypes: []TypeID{
				"prompt.ChatTemplate",
			},
		},
	},
}

var comp2AddFn = map[components.Component]reflect.Value{
	components.ComponentOfPrompt: reflect.ValueOf((*graph).AddChatTemplateNode),
}

var typeMap = map[TypeID]*TypeMeta{
	"string": {
		ID:                "string",
		BasicType:         BasicTypeString,
		InstantiationType: InstantiationTypeLiteral,
	},
	"bool": {
		ID:                "bool",
		BasicType:         BasicTypeBool,
		InstantiationType: InstantiationTypeLiteral,
	},
	"schema.MessageTemplate": {
		ID:        "schema.MessageTemplate",
		BasicType: BasicTypeInterface,
	},
	"schema.Format": {
		ID:                "schema.Format",
		BasicType:         BasicTypeString,
		InstantiationType: InstantiationTypeUnmarshal,
		ReflectType:       generic.PtrOf(reflect.TypeOf(schema.FormatType(0))),
	},
	"schema.messagePlaceholder": {
		ID:                "schema.messagePlaceholder",
		BasicType:         BasicTypeStruct,
		IsPtr:             true,
		InterfaceType:     (*TypeID)(generic.PtrOf("schema.MessageTemplate")),
		InstantiationType: InstantiationTypeFunction,
		FunctionMeta: &FunctionMeta{
			Name:      "schema.MessagesPlaceholder",
			FuncValue: reflect.ValueOf(schema.MessagesPlaceholder),
			InputTypes: []TypeID{
				"string",
				"bool",
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
		InterfaceType:     (*TypeID)(generic.PtrOf("schema.MessageTemplate")),
		ReflectType:       generic.PtrOf(reflect.TypeOf(&schema.Message{})),
	},
	"schema.UserMessage": {
		ID:                "schema.UserMessage",
		BasicType:         BasicTypeStruct,
		IsPtr:             true,
		InstantiationType: InstantiationTypeFunction,
		FunctionMeta: &FunctionMeta{
			Name:      "schema.UserMessage",
			FuncValue: reflect.ValueOf(schema.UserMessage),
			InputTypes: []TypeID{
				"string",
			},
			OutputTypes: []TypeID{
				"*schema.Message",
			},
		},
	},
	"schema.SystemMessage": {
		ID:                "schema.SystemMessage",
		BasicType:         BasicTypeStruct,
		IsPtr:             true,
		InstantiationType: InstantiationTypeFunction,
		FunctionMeta: &FunctionMeta{
			Name:      "schema.SystemMessage",
			FuncValue: reflect.ValueOf(schema.SystemMessage),
			InputTypes: []TypeID{
				"string",
			},
			OutputTypes: []TypeID{
				"*schema.Message",
			},
		},
	},
	"prompts.ChatTemplate": {
		ID:        "prompt.ChatTemplate",
		BasicType: BasicTypeInterface,
	},
}
