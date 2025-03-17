package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/internal/generic"
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
		InputType:       TypeMeta{BasicType: BasicTypeString},
		OutputType:      TypeMeta{BasicType: BasicTypeMap, MapType: generic.PtrOf(MapTypeStringAny)},
		NodeTriggerMode: generic.PtrOf(AllPredecessor),
		Nodes: []*NodeDSL{
			{
				Key:           "1",
				ComponentType: ComponentOfPassthrough,
				OutputKey:     generic.PtrOf("1"),
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

	out, err := InvokeCompiledGraph[string, map[string]any](ctx, c, "hello")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"1": "hello"}, out)
}
