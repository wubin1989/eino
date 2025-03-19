package compose

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

func TestGraphWithPassthrough(t *testing.T) {
	dsl := &GraphDSL{
		ID:              "test",
		Namespace:       "test",
		Name:            generic.PtrOf("test_passthrough"),
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
		NodeTriggerMode: generic.PtrOf(AllPredecessor),
		Nodes: []*NodeDSL{
			{
				Key:    "1",
				ImplID: "prompt.DefaultChatTemplate",
				Configs: []Config{
					{
						Index: 0,
						Value: "0",
					},
				},
				Slots: []Slot{
					{
						TypeID: "schema.messagePlaceholder",
						Path:   FieldPath{"[1]"},
						Configs: []Config{
							{
								Index: 0,
								Value: "history",
							},
							{
								Index: 1,
								Value: "true",
							},
						},
					},
					{
						TypeID: "*schema.Message",
						Path:   FieldPath{"[2]"},
						Configs: []Config{
							{
								Value: `{"role":"system", "content":"you are a {certain} assistant"}`,
							},
						},
					},
					{
						TypeID: "*schema.Message",
						Path:   FieldPath{"[3]"},
						Configs: []Config{
							{
								Value: `{"role":"user", "content":"hello, {world}"}`,
							},
						},
					},
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

func TestGraphWithRetriever(t *testing.T) {
	implMap["testRetriever"] = testRetrieverImpl
	typeMap[testRetrieverType.ID] = testRetrieverType
	typeMap[testEmbeddingType.ID] = testEmbeddingType
	typeMap[testRetrieverConfigType.ID] = testRetrieverConfigType
	typeMap[testEmbeddingConfigType.ID] = testEmbeddingConfigType
	defer func() {
		delete(implMap, "testRetriever")
		delete(typeMap, testRetrieverType.ID)
		delete(typeMap, testEmbeddingType.ID)
		delete(typeMap, testRetrieverConfigType.ID)
		delete(typeMap, testEmbeddingConfigType.ID)
	}()

	dsl := &GraphDSL{
		ID:              "test",
		Namespace:       "test",
		Name:            generic.PtrOf("test_retriever"),
		NodeTriggerMode: generic.PtrOf(AllPredecessor),
		Nodes: []*NodeDSL{
			{
				Key:    "1",
				ImplID: "testRetriever",
				Configs: []Config{
					{
						Value: "{}",
						Slots: []Slot{
							{
								TypeID: "testEmbedding",
								Path:   FieldPath{"Embedding"},
								Configs: []Config{
									{
										Index: 1,
										Value: "{}",
									},
								},
							},
						},
					},
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
	out, err := InvokeCompiledGraph[string, []*schema.Document](ctx, c, "hello")
	assert.NoError(t, err)
	assert.Equal(t, []*schema.Document{
		{
			ID:      "1",
			Content: "hello",
		},
	}, out)
}

type testRetriever struct{}

type testRetrieverConfig struct {
	Embedding embedding.Embedder
}

func NewTestRetriever(config *testRetrieverConfig) (retriever.Retriever, error) {
	if config == nil || config.Embedding == nil {
		return nil, errors.New("invalid config")
	}
	return &testRetriever{}, nil
}

func (t *testRetriever) Retrieve(_ context.Context, query string, _ ...retriever.Option) ([]*schema.Document, error) {
	return []*schema.Document{
		{
			ID:      "1",
			Content: query,
		},
	}, nil
}

type testEmbedding struct{}

type testEmbeddingConfig struct{}

func NewTestEmbedding(_ context.Context, config *testEmbeddingConfig) (embedding.Embedder, error) {
	if config == nil {
		return nil, errors.New("invalid config")
	}
	return &testEmbedding{}, nil
}

func (t *testEmbedding) EmbedStrings(_ context.Context, _ []string, _ ...embedding.Option) ([][]float64, error) {
	panic("implement me")
}

var testRetrieverType = &TypeMeta{
	ID:                "testRetriever",
	BasicType:         BasicTypeStruct,
	IsPtr:             true,
	InterfaceType:     (*TypeID)(generic.PtrOf("retriever.Retriever")),
	InstantiationType: InstantiationTypeFunction,
	FunctionMeta: &FunctionMeta{
		Name:      "NewTestRetriever",
		FuncValue: reflect.ValueOf(NewTestRetriever),
		InputTypes: []TypeID{
			"testRetrieverConfig",
		},
		OutputTypes: []TypeID{
			"retriever.Retriever",
			errType.ID,
		},
	},
}

var testRetrieverConfigType = &TypeMeta{
	ID:                "testRetrieverConfig",
	BasicType:         BasicTypeStruct,
	IsPtr:             true,
	InstantiationType: InstantiationTypeUnmarshal,
	ReflectType:       generic.PtrOf(reflect.TypeOf(&testRetrieverConfig{})),
}

var testEmbeddingType = &TypeMeta{
	ID:                "testEmbedding",
	BasicType:         BasicTypeStruct,
	IsPtr:             true,
	InterfaceType:     (*TypeID)(generic.PtrOf("embedding.Embedder")),
	InstantiationType: InstantiationTypeFunction,
	FunctionMeta: &FunctionMeta{
		Name:      "NewTestEmbedding",
		FuncValue: reflect.ValueOf(NewTestEmbedding),
		InputTypes: []TypeID{
			ctxType.ID,
			"testEmbeddingConfig",
		},
		OutputTypes: []TypeID{
			"embedding.Embedder",
			errType.ID,
		},
	},
}

var testEmbeddingConfigType = &TypeMeta{
	ID:                "testEmbeddingConfig",
	BasicType:         BasicTypeStruct,
	IsPtr:             true,
	InstantiationType: InstantiationTypeUnmarshal,
	ReflectType:       generic.PtrOf(reflect.TypeOf(&testEmbeddingConfig{})),
}

var testRetrieverImpl = &ImplMeta{
	TypeID:        "testRetriever",
	ComponentType: components.ComponentOfRetriever,
}
