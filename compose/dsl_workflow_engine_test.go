package compose

import (
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

func TestWorkflowFromDSL(t *testing.T) {
	lambda1 := func(ctx context.Context, in map[string]any) (map[string]any, error) {
		in["lambda1"] = "1"
		return in, nil
	}

	lambda2 := func(ctx context.Context, in map[string]any) (map[string]any, error) {
		in["lambda2"] = "2"
		return in, nil
	}

	lambda3 := func(ctx context.Context, in map[string]any) (map[string]any, error) {
		in["lambda3"] = "3"
		return in, nil
	}

	lambda5 := func(ctx context.Context, in map[string]any) (map[string]any, error) {
		in["lambda5"] = "5"
		return in, nil
	}

	implMap["lambda1"] = &ImplMeta{
		ComponentType: ComponentOfLambda,
		Lambda:        func() *Lambda { return InvokableLambda(lambda1) },
	}

	implMap["lambda2"] = &ImplMeta{
		ComponentType: ComponentOfLambda,
		Lambda:        func() *Lambda { return InvokableLambda(lambda2) },
	}

	implMap["lambda3"] = &ImplMeta{
		ComponentType: ComponentOfLambda,
		Lambda:        func() *Lambda { return InvokableLambda(lambda3) },
	}

	implMap["lambda5"] = &ImplMeta{
		ComponentType: ComponentOfLambda,
		Lambda:        func() *Lambda { return InvokableLambda(lambda5) },
	}

	condition := func(ctx context.Context, in *schema.StreamReader[map[string]any]) (string, error) {
		defer in.Close()

		for {
			chunk, err := in.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return "", err
			}

			startValue, ok := chunk[START]
			if ok {
				if startValue == "hello" {
					return "sub_workflow", nil
				} else {
					return "lambda3", nil
				}
			}
		}

		return "lambda3", nil
	}

	branchFunctionMap["condition"] = &BranchFunction{
		ID:              "condition",
		FuncValue:       reflect.ValueOf(condition),
		InputType:       reflect.TypeOf(map[string]any{}),
		IsStream:        true,
		StreamConverter: &StreamConverterImpl[map[string]any]{},
	}

	defer func() {
		delete(implMap, "lambda1")
		delete(implMap, "lambda2")
		delete(implMap, "lambda3")
		delete(implMap, "lambda5")
		delete(branchFunctionMap, "condition")
	}()

	subDSL := &WorkflowDSL{
		ID:         "sub_workflow",
		Namespace:  "test",
		Name:       generic.PtrOf("sub_workflow"),
		InputType:  "map[string]any",
		OutputType: "map[string]any",
		EndInputs: []*WorkflowNodeInputDSL{
			{
				FromNodeKey: START,
			},
		},
	}

	dsl := &WorkflowDSL{
		ID:         "test",
		Namespace:  "test",
		Name:       generic.PtrOf("test_workflow"),
		InputType:  "map[string]any",
		OutputType: "map[string]any",
		Nodes: []*WorkflowNodeDSL{
			{
				NodeDSL: &NodeDSL{
					Key:    "lambda1",
					ImplID: "lambda1",
				},
				Inputs: []*WorkflowNodeInputDSL{
					{
						FromNodeKey: START,
					},
				},
			},
			{
				NodeDSL: &NodeDSL{
					Key:    "lambda2",
					ImplID: "lambda2",
				},
				Inputs: []*WorkflowNodeInputDSL{
					{
						FromNodeKey: START,
					},
				},
			},
			{
				NodeDSL: &NodeDSL{
					Key:    "lambda3",
					ImplID: "lambda3",
				},
				Inputs: []*WorkflowNodeInputDSL{
					{
						FromNodeKey: "lambda1",
						FieldPathMappings: []FieldPathMapping{
							{
								From: FieldPath{"lambda1"},
								To:   FieldPath{"lambda1"},
							},
						},
						NoDirectDependency: true,
					},
				},
				StaticValues: []StaticValue{
					{
						TypeID: "string",
						Path:   FieldPath{"static_value"},
						Value:  "static_value",
					},
				},
			},
			{
				NodeDSL: &NodeDSL{
					Key:      "sub_workflow",
					Workflow: subDSL,
				},
				Inputs: []*WorkflowNodeInputDSL{
					{
						FromNodeKey: "lambda2",
						FieldPathMappings: []FieldPathMapping{
							{
								From: FieldPath{"lambda2"},
								To:   FieldPath{"lambda2"},
							},
						},
						NoDirectDependency: true,
					},
				},
			},
			{
				NodeDSL: &NodeDSL{
					Key:    "lambda5",
					ImplID: "lambda5",
				},
				Inputs: []*WorkflowNodeInputDSL{
					{
						FromNodeKey: "lambda1",
						FieldPathMappings: []FieldPathMapping{
							{
								From: FieldPath{"lambda1"},
								To:   FieldPath{"lambda1"},
							},
						},
						NoDirectDependency: true,
					},
				},
				Dependencies: []string{
					"lambda3",
				},
			},
		},
		Branches: []*WorkflowBranchDSL{
			{
				Key: "branch",
				BranchDSL: BranchDSL{
					Condition: "condition",
					EndNodes: []string{
						"lambda3",
						"sub_workflow",
					},
				},
				Inputs: []*WorkflowNodeInputDSL{
					{
						FromNodeKey: "lambda1",
						FieldPathMappings: []FieldPathMapping{
							{
								From: FieldPath{"lambda1"},
								To:   FieldPath{"lambda1"},
							},
						},
					},
					{
						FromNodeKey: START,
						FieldPathMappings: []FieldPathMapping{
							{
								From: FieldPath{START},
								To:   FieldPath{START},
							},
						},
						NoDirectDependency: true,
					},
				},
				Dependencies: []string{
					"lambda2",
				},
			},
		},
		EndInputs: []*WorkflowNodeInputDSL{
			{
				FromNodeKey: "sub_workflow",
				FieldPathMappings: []FieldPathMapping{
					{
						From: FieldPath{"lambda2"},
						To:   FieldPath{"lambda2"},
					},
				},
			},
			{
				FromNodeKey: "lambda3",
				FieldPathMappings: []FieldPathMapping{
					{
						From: FieldPath{"lambda3"},
						To:   FieldPath{"lambda3"},
					},
					{
						From: FieldPath{"static_value"},
						To:   FieldPath{"static_value"},
					},
				},
				NoDirectDependency: true,
			},
		},
		EndDependencies: []string{
			"lambda5",
		},
	}

	content, err := yaml.Marshal(dsl)
	assert.NoError(t, err)
	t.Log(string(content))

	ctx := context.Background()
	r, err := CompileWorkflow(ctx, dsl)
	assert.NoError(t, err)
	out, err := r.Invoke(ctx, `{"start": "hello"}`)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{
		"lambda2": "2",
	}, out)

	outS, err := r.Transform(ctx, `{"start": "hello1"}`)
	assert.NoError(t, err)
	out, err = outS.Recv()
	assert.NoError(t, err)
	outS.Close()
	assert.Equal(t, map[string]any{
		"lambda3":      "3",
		"static_value": "static_value",
	}, out)
}
