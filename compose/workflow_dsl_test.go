package compose

import (
	"context"
	"reflect"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

func TestWorkflowDSL(t *testing.T) {
	condition := func(ctx context.Context, in map[string]any) (end string, err error) {
		if in[START] == "hello" {
			return "1", nil
		}

		return "2", nil
	}

	conditionMap := map[string]GraphBranchCondition[map[string]any]{
		"test_cond": condition,
	}

	_ = conditionMap

	type state struct {
		OK bool
	}

	genStateFunc := func(ctx context.Context) *state {
		return &state{}
	}

	genStateFuncMap := map[string]GenLocalState[any]{
		"test_state": func(ctx context.Context) any {
			return genStateFunc(ctx)
		},
	}

	_ = genStateFuncMap

	type statePreHandler func(ctx context.Context, in any, state any) (newIn any, err error)

	type statePreHandlerConf struct {
		nodeType components.Component

		f statePreHandler
	}

	// 业务提供一个 state pre handler function 的 full path
	// 以及输入类型的 full path, state 类型的 full path
	// code gen 生成一个新的 any 类型的 pre handler，内部做类型转换

	// Lambda:
	// 业务提供的：
	// 1. function 的 full path。最多四个流式范式的 function.
	// 2. [可能不需要] reflect 出来 function 的输入输出和 option 类型
	// 3. code gen 出来 Lambda 变量
	// code gen 一个 map，lambda name -> lambda 变量

	// Config:
	// 已知的 Eino-Ext 组件：
	//

	// Slot:
	//

	// 内外场支持的组件清单如何区分？
	// coze，抖音支持的组件清单如何区分？
}

func TestGraphWithPassthrough(t *testing.T) {
	dsl := &GraphDSL{
		ID:              "test",
		Namespace:       "test",
		Name:            generic.PtrOf("test_passthrough"),
		InputType:       TypeMeta{BasicType: BasicTypeMap, MapType: generic.PtrOf(MapTypeStringAny)},
		OutputType:      TypeMeta{BasicType: BasicTypeMap, MapType: generic.PtrOf(MapTypeStringAny)},
		NodeTriggerMode: generic.PtrOf(AllPredecessor),
		Nodes: []*NodeDSL{
			{
				Key:       "1",
				ImplID:    "Passthrough",
				InputKey:  generic.PtrOf("1"),
				OutputKey: generic.PtrOf("1"),
			},
		},
		Edges: []*EdgeDSL{
			{
				From: START,
				To:   "1",
			},
			{
				From: "1",
				To:   END,
			},
		},
	}

	ctx := context.Background()
	c, err := CompileGraph(ctx, dsl)
	assert.NoError(t, err)

	out, err := InvokeCompiledGraph[map[string]any, map[string]any](ctx, c, map[string]any{"1": "hello"})
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"1": "hello"}, out)
}

func TestInstantiation(t *testing.T) {
	msgType := reflect.TypeOf(&schema.Message{})
	msg := &schema.Message{
		Role:    schema.User,
		Content: "hello",
	}
	marshalled, _ := sonic.Marshal(msg)

	msgValue1 := newInstanceByType(msgType).Interface()
	_ = sonic.Unmarshal(marshalled, msgValue1)
	assert.Equal(t, msg, msgValue1)

	msg2 := &schema.Message{
		Role:    schema.System,
		Content: "world",
	}
	marshalled, _ = sonic.Marshal(msg2)
	msgValue2 := newInstanceByType(msgType).Interface()
	_ = sonic.Unmarshal(marshalled, msgValue2)
	assert.Equal(t, msg2, msgValue2)

	formatType := reflect.TypeOf(schema.FString)
	marshalled, _ = sonic.Marshal(schema.FString)
	formatValue := newInstanceByType(formatType).Interface()
	_ = sonic.Unmarshal(marshalled, formatValue)
	assert.Equal(t, schema.FString, formatValue)

	factoryFunc := reflect.ValueOf(prompt.FromMessages)
	result := factoryFunc.Call([]reflect.Value{
		reflect.ValueOf(formatValue),
		reflect.ValueOf(msgValue2),
		reflect.ValueOf(msgValue1)})
	template := result[0]

	g := NewGraph[any, any]()

	addPromptFn := reflect.ValueOf((*graph).AddChatTemplateNode)
	result = addPromptFn.Call([]reflect.Value{
		reflect.ValueOf(g.graph),
		reflect.ValueOf("prompt_node"),
		template,
		reflect.ValueOf(WithNodeName("prompt_node_name")),
	})
	//assert.NoError(t, result[0].Interface().(error))

	_ = g.AddEdge(START, "prompt_node")
	_ = g.AddEdge("prompt_node", END)
	ctx := context.Background()
	r, err := g.Compile(ctx)
	assert.NoError(t, err)
	out, err := r.Invoke(ctx, map[string]any{})
	assert.NoError(t, err)
	assert.Equal(t, []*schema.Message{
		{
			Role:    schema.System,
			Content: "world",
		},
		{
			Role:    schema.User,
			Content: "hello",
		},
	}, out)
}

func TestGraphWithChatTemplate(t *testing.T) {
	dsl := &GraphDSL{
		ID:              "test",
		Namespace:       "test",
		Name:            generic.PtrOf("test_chat_template"),
		InputType:       TypeMeta{BasicType: BasicTypeMap, MapType: generic.PtrOf(MapTypeStringAny)},
		OutputType:      TypeMeta{BasicType: BasicTypeArray, ArrayType: generic.PtrOf(ArrayTypeMessagePtr)},
		NodeTriggerMode: generic.PtrOf(AllPredecessor),
		Nodes: []*NodeDSL{
			{
				Key:    "1",
				ImplID: "prompt.DefaultChatTemplate",
				Configs: []string{
					`0`,
					`{
						"role":    "system",
						"content": "you are a {certain} assistant"
					}`,
					`{
						"role":    "user",
						"content": "hello, {world}"
					}`,
				},
			},
		},
		Edges: []*EdgeDSL{
			{
				From: START,
				To:   "1",
			},
			{
				From: "1",
				To:   END,
			},
		},
	}

	ctx := context.Background()
	c, err := CompileGraph(ctx, dsl)
	if err != nil {
		t.Fatal(err)
	}
	out, err := InvokeCompiledGraph[map[string]any, []*schema.Message](ctx, c, map[string]any{"world": "awesome", "certain": "shy"})
	assert.NoError(t, err)
	assert.Equal(t, []*schema.Message{
		{
			Role:    "system",
			Content: "you are a shy assistant",
		},
		{
			Role:    "user",
			Content: "hello, awesome",
		},
	}, out)
}
