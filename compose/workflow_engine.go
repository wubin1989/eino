package compose

import (
	"context"
	"fmt"
	"reflect"
)

var (
	addInputFn                 = reflect.ValueOf((*WorkflowNode).AddInput)
	addInputWithOptionFn       = reflect.ValueOf((*WorkflowNode).AddInputWithOptions)
	addDependencyFn            = reflect.ValueOf((*WorkflowNode).AddDependency)
	addEndFn                   = reflect.ValueOf((*Workflow[any, any]).AddEnd)
	addEndWithOptionFn         = reflect.ValueOf((*Workflow[any, any]).AddEndWithOptions)
	addEndDependencyFn         = reflect.ValueOf((*Workflow[any, any]).AddEndDependency)
	branchAddInputFn           = reflect.ValueOf((*WorkflowBranch).AddInput)
	branchAddInputWithOptionFn = reflect.ValueOf((*WorkflowBranch).AddInputWithOptions)
	branchAddDependencyFn      = reflect.ValueOf((*WorkflowBranch).AddDependency)
	setStaticValueFn           = reflect.ValueOf((*WorkflowNode).SetStaticValue)
)

func NewWorkflowFromDSL(ctx context.Context, dsl *WorkflowDSL) (*Workflow[any, any], error) {
	newGraphOptions, err := genNewGraphOptions(dsl.StateType)
	if err != nil {
		return nil, err
	}

	inputType, ok := typeMap[dsl.InputType]
	if !ok {
		return nil, fmt.Errorf("input type not found: %v", dsl.InputType)
	}
	outputType, ok := typeMap[dsl.OutputType]
	if !ok {
		return nil, fmt.Errorf("output type not found: %v", dsl.OutputType)
	}

	newGraphOptions = append(newGraphOptions, withInputType(*inputType.ReflectType), withOutputType(*outputType.ReflectType))

	wf := NewWorkflow[any, any](newGraphOptions...)

	// add workflow nodes, then add inputs and dependencies
	for i := range dsl.Nodes {
		node := dsl.Nodes[i]
		if err = workflowAddNode(ctx, wf, node); err != nil {
			return nil, err
		}
	}

	wfValue := reflect.ValueOf(wf)

	// add workflow branches, then add inputs and dependencies
	for i := range dsl.Branches {
		branch := dsl.Branches[i]
		if err = workflowAddBranch(ctx, wf, branch); err != nil {
			return nil, err
		}
	}

	// add workflow ends, then add inputs and dependencies
	_ = addInputsAndDependencies(wfValue, dsl.EndInputs, dsl.EndDependencies, addEndFn, addEndWithOptionFn, addEndDependencyFn)

	return wf, nil
}

func CompileWorkflow(ctx context.Context, dsl *WorkflowDSL) (*DSLRunner, error) {
	wf, err := NewWorkflowFromDSL(ctx, dsl)
	if err != nil {
		return nil, err
	}

	compileOptions := genWorkflowCompileOptions(dsl)
	r, err := wf.Compile(ctx, compileOptions...)
	if err != nil {
		return nil, err
	}

	inputType, ok := typeMap[dsl.InputType]
	if !ok {
		return nil, fmt.Errorf("type not found: %v", dsl.InputType)
	}

	return &DSLRunner{
		Runnable:  r,
		InputType: inputType,
	}, nil
}

func workflowAddNode(ctx context.Context, wf *Workflow[any, any], node *WorkflowNodeDSL) error {
	addNodeOpts, err := genAddNodeOptions(node.NodeDSL)
	if err != nil {
		return err
	}

	dsl := node.NodeDSL

	var wfNode reflect.Value
	if dsl.GraphDSL != nil {
		subGraph, err := NewGraphFromDSL(ctx, dsl.GraphDSL)
		if err != nil {
			return err
		}

		compileOptions := genGraphCompileOptions(dsl.GraphDSL)

		addNodeOpts = append(addNodeOpts, WithGraphCompileOptions(compileOptions...))

		wfNode = reflect.ValueOf(wf.AddGraphNode(dsl.Key, subGraph, addNodeOpts...))
	}

	if dsl.WorkflowDSL != nil {
		subGraph, err := NewWorkflowFromDSL(ctx, dsl.WorkflowDSL)
		if err != nil {
			return err
		}

		compileOptions := genWorkflowCompileOptions(dsl.WorkflowDSL)
		addNodeOpts = append(addNodeOpts, WithGraphCompileOptions(compileOptions...))

		wfNode = reflect.ValueOf(wf.AddGraphNode(dsl.Key, subGraph, addNodeOpts...))
	}

	if !wfNode.IsValid() {
		implMeta, ok := implMap[dsl.ImplID]
		if !ok {
			return fmt.Errorf("impl not found: %v", dsl.ImplID)
		}

		if implMeta.ComponentType == ComponentOfLambda {
			lambda := implMeta.Lambda
			if lambda == nil {
				return fmt.Errorf("lambda not found: %v", dsl.ImplID)
			}

			wfNode = reflect.ValueOf(wf.AddLambdaNode(dsl.Key, lambda(), addNodeOpts...))
		} else {
			typeMeta, ok := typeMap[implMeta.TypeID]
			if !ok {
				return fmt.Errorf("type not found: %v", implMeta.TypeID)
			}

			result, err := typeMeta.Instantiate(ctx, nil, dsl.Configs)
			if err != nil {
				return err
			}

			addFn, ok := comp2WorkflowAddFn[implMeta.ComponentType]
			if !ok {
				return fmt.Errorf("add workflow node function not found: %v", implMeta.ComponentType)
			}
			results := addFn.CallSlice(append([]reflect.Value{reflect.ValueOf(wf), reflect.ValueOf(dsl.Key), result, reflect.ValueOf(addNodeOpts)}))
			if len(results) != 1 {
				return fmt.Errorf("add workflow node function return value length mismatch: given %d, defined %d", len(results), 1)
			}

			wfNode = results[0]
		}
	}

	_ = addInputsAndDependencies(wfNode, node.Inputs, node.Dependencies, addInputFn, addInputWithOptionFn, addDependencyFn)

	for i := range node.StaticValues {
		staticValue := node.StaticValues[i]
		typeMeta, ok := typeMap[staticValue.TypeID]
		if !ok {
			return fmt.Errorf("type not found: %v", staticValue.TypeID)
		}

		v, err := typeMeta.Instantiate(ctx, &staticValue.Value, nil)
		if err != nil {
			return err
		}

		results := setStaticValueFn.Call([]reflect.Value{wfNode, reflect.ValueOf(staticValue.Path), v})
		wfNode = results[0]
	}

	return nil
}

func workflowAddBranch(_ context.Context, wf *Workflow[any, any], dsl *WorkflowBranchDSL) error {
	branch, inputType, err := genBranch(dsl.BranchDSL)
	if err != nil {
		return err
	}

	wfBranch := wf.AddBranch(dsl.Key, branch, withBranchInputType(inputType))

	wfBranchV := reflect.ValueOf(wfBranch)

	_ = addInputsAndDependencies(wfBranchV, dsl.Inputs, dsl.Dependencies, branchAddInputFn, branchAddInputWithOptionFn, branchAddDependencyFn)

	return nil
}

func genWorkflowCompileOptions(dsl *WorkflowDSL) []GraphCompileOption {
	compileOptions := make([]GraphCompileOption, 0)
	if dsl.Name != nil {
		compileOptions = append(compileOptions, WithGraphName(*dsl.Name))
	}
	return compileOptions
}

func addInputsAndDependencies(v reflect.Value, inputs []*WorkflowNodeInputDSL, dependencies []string, addInputFn, addInputWithOptFn, addDependencyFn reflect.Value) reflect.Value {
	for i := range inputs {
		input := inputs[i]
		fromNode := reflect.ValueOf(input.FromNodeKey)
		var fieldMappings []*FieldMapping
		for i := range input.FieldPathMappings {
			mapping := input.FieldPathMappings[i]
			fieldMappings = append(fieldMappings, MapFieldPaths(mapping.From, mapping.To))
		}
		if !input.NoDirectDependency {
			result := addInputFn.CallSlice([]reflect.Value{v, fromNode, reflect.ValueOf(fieldMappings)})
			v = result[0]
		} else {
			result := addInputWithOptFn.Call([]reflect.Value{v, fromNode, reflect.ValueOf(fieldMappings), reflect.ValueOf(WithNoDirectDependency())})
			v = result[0]
		}
	}

	for i := range dependencies {
		result := addDependencyFn.Call([]reflect.Value{v, reflect.ValueOf(dependencies[i])})
		v = result[0]
	}

	return v
}
