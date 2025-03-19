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

var ctxType = &TypeMeta{
	ID: "Context",
}

var errType = &TypeMeta{
	ID: "Error",
}

var implMap = map[string]*ImplMeta{
	"Passthrough": passthrough,
	"prompt.DefaultChatTemplate": {
		TypeID:        "prompt.DefaultChatTemplate",
		ComponentType: components.ComponentOfPrompt,
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
		BasicType: BasicTypeInterface, // this is a slot
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
		InterfaceType:     (*TypeID)(generic.PtrOf("schema.MessageTemplate")), // can be used to fill the slot with schema.MessageTemplate
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
				"*schema.Message", // dependencies only appear as fields or nested fields within Config, or as parameters to factory functions

				// we can know this is an interface type, which is a slot
				//"schema.MessageTemplate",
			},
			OutputTypes: []TypeID{
				"prompt.ChatTemplate",
			},
		},
	},
}
