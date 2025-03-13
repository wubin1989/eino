/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package compose

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/internal/generic"
)

// WorkflowNode is the node of the Workflow.
type WorkflowNode struct {
	g            *graph
	key          string
	addInputs    []func()
	staticValues map[string]any
}

// Workflow is wrapper of graph, replacing AddEdge with declaring dependencies and field mappings between nodes.
// Under the hood it uses NodeTriggerMode(AllPredecessor), so does not support cycles.
type Workflow[I, O any] struct {
	g                *graph
	workflowNodes    map[string]*WorkflowNode
	workflowBranches map[string]*WorkflowBranch
}

// NewWorkflow creates a new Workflow.
func NewWorkflow[I, O any](opts ...NewGraphOption) *Workflow[I, O] {
	options := &newGraphOptions{}
	for _, opt := range opts {
		opt(options)
	}

	wf := &Workflow[I, O]{
		g: newGraphFromGeneric[I, O](
			ComponentOfWorkflow,
			options.withState,
			options.stateType,
		),
		workflowNodes:    make(map[string]*WorkflowNode),
		workflowBranches: make(map[string]*WorkflowBranch),
	}

	return wf
}

func (wf *Workflow[I, O]) Compile(ctx context.Context, opts ...GraphCompileOption) (Runnable[I, O], error) {
	if len(globalGraphCompileCallbacks) > 0 {
		opts = append([]GraphCompileOption{WithGraphCompileCallbacks(globalGraphCompileCallbacks...)}, opts...)
	}
	option := newGraphCompileOptions(opts...)

	cr, err := wf.compile(ctx, option)
	if err != nil {
		return nil, err
	}

	cr.meta = &executorMeta{
		component:                  wf.g.cmp,
		isComponentCallbackEnabled: true,
		componentImplType:          "",
	}

	cr.nodeInfo = &nodeInfo{
		name: option.graphName,
	}

	ctxWrapper := func(ctx context.Context, opts ...Option) context.Context {
		return initGraphCallbacks(ctx, cr.nodeInfo, cr.meta, opts...)
	}

	rp, err := toGenericRunnable[I, O](cr, ctxWrapper)
	if err != nil {
		return nil, err
	}

	return rp, nil
}

func (wf *Workflow[I, O]) AddChatModelNode(key string, chatModel model.ChatModel, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddChatModelNode(key, chatModel, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddChatTemplateNode(key string, chatTemplate prompt.ChatTemplate, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddChatTemplateNode(key, chatTemplate, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddToolsNode(key string, tools *ToolsNode, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddToolsNode(key, tools, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddRetrieverNode(key string, retriever retriever.Retriever, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddRetrieverNode(key, retriever, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddEmbeddingNode(key string, embedding embedding.Embedder, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddEmbeddingNode(key, embedding, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddIndexerNode(key string, indexer indexer.Indexer, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddIndexerNode(key, indexer, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddLoaderNode(key string, loader document.Loader, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddLoaderNode(key, loader, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddDocumentTransformerNode(key string, transformer document.Transformer, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddDocumentTransformerNode(key, transformer, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddGraphNode(key string, graph AnyGraph, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddGraphNode(key, graph, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) AddLambdaNode(key string, lambda *Lambda, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddLambdaNode(key, lambda, opts...)
	return wf.initNode(key)
}

func (wf *Workflow[I, O]) addPassthroughNode(key string, opts ...GraphAddNodeOpt) *WorkflowNode {
	_ = wf.g.AddPassthroughNode(key, opts...)
	return wf.initNode(key)
}

func (n *WorkflowNode) AddInput(fromNodeKey string, inputs ...*FieldMapping) *WorkflowNode {
	return n.addDependencyRelation(fromNodeKey, inputs, &workflowAddInputOpts{})
}

type workflowAddInputOpts struct {
	noDirectDependency     bool
	dependencyWithoutInput bool
}

type WorkflowAddInputOpt func(*workflowAddInputOpts)

func getAddInputOpts(opts []WorkflowAddInputOpt) *workflowAddInputOpts {
	opt := &workflowAddInputOpts{}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

// WithNoDirectDependency creates data mapping without establishing control flow relationship when adding an input.
//
// In a workflow graph, edges typically serve two purposes:
// 1. Control flow: determining the execution order of nodes
// 2. Data flow: passing data from one node to another
//
// This option creates an edge that only handles data flow, meaning:
// - The current node can access outputs from the predecessor node
// - The current node's execution is NOT blocked waiting for the predecessor to complete
//
// Usage:
//
//	node.AddInputWithOptions("fromNode", mappings, WithNoDirectDependency())
//
// Important considerations:
//
//  1. Branch scenarios: If there is a branch between the current node and the predecessor node,
//     you MUST use WithNoDirectDependency to ensure correct data flow.
//
//  2. Topological order: The current node must be a topological successor of the predecessor node
//     in the graph. Otherwise, data mapping cannot be established as the data may not be available
//     when needed.
//
//  3. Control flow impact: This option removes the execution order dependency between nodes.
//     Ensure this doesn't break your workflow's logical sequence.
//
// Common use cases:
// - Accessing reference data from earlier steps without blocking execution
// - Parallel processing with shared input data
// - Complex branching scenarios where control flow and data flow need to be handled separately
func WithNoDirectDependency() WorkflowAddInputOpt {
	return func(opt *workflowAddInputOpts) {
		opt.noDirectDependency = true
	}
}

func (n *WorkflowNode) AddInputWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *WorkflowNode {
	return n.addDependencyRelation(fromNodeKey, inputs, getAddInputOpts(opts))
}

func (n *WorkflowNode) AddDependency(fromNodeKey string) *WorkflowNode {
	return n.addDependencyRelation(fromNodeKey, nil, &workflowAddInputOpts{dependencyWithoutInput: true})
}

// SetStaticValue sets a static value for a field path that will be available
// during workflow execution. These values are determined at compile time and
// remain constant throughout the workflow's lifecycle.
//
// Example:
//
//	node.SetStaticValue(FieldPath{"query"}, "static query")
func (n *WorkflowNode) SetStaticValue(path FieldPath, value any) *WorkflowNode {
	n.staticValues[strings.Join(path, pathSeparator)] = value
	return n
}

func (n *WorkflowNode) addDependencyRelation(fromNodeKey string, inputs []*FieldMapping, options *workflowAddInputOpts) *WorkflowNode {
	for _, input := range inputs {
		input.fromNodeKey = fromNodeKey
	}

	if len(inputs) == 0 && !options.dependencyWithoutInput {
		inputs = append(inputs, &FieldMapping{
			fromNodeKey: fromNodeKey,
		})
	}

	if options.noDirectDependency {
		n.addInputs = append(n.addInputs, func() {
			_ = n.g.addIndirectEdgeWithMappings(fromNodeKey, n.key, inputs...)
		})
	} else if options.dependencyWithoutInput {
		n.addInputs = append(n.addInputs, func() {
			_ = n.g.addControlOnlyEdgeWithMappings(fromNodeKey, n.key, inputs...)
		})
	} else {
		n.addInputs = append(n.addInputs, func() {
			_ = n.g.addEdgeWithMappings(fromNodeKey, n.key, inputs...)
		})
	}

	return n
}

type WorkflowBranch struct {
	branchKey string
	*GraphBranch
	inputs              map[string][]*FieldMapping
	inputAsIndirectEdge map[string]bool
}

// AddBranch 创建一个分支（选择器），同时在 passthrough 和 endNodes 之间创建流转关系，不配置 endNodes 的数据映射
func (wf *Workflow[I, O]) AddBranch(branchKey string, branch *GraphBranch) *WorkflowBranch {
	wb := &WorkflowBranch{
		branchKey:           branchKey,
		GraphBranch:         branch,
		inputs:              make(map[string][]*FieldMapping),
		inputAsIndirectEdge: make(map[string]bool),
	}
	wf.workflowBranches[branchKey] = wb
	return wb
}

func (wb *WorkflowBranch) AddInput(fromNodeKey string, inputs ...*FieldMapping) *WorkflowBranch {
	return wb.addDependencyRelation(fromNodeKey, inputs, &workflowAddInputOpts{})
}

func (wb *WorkflowBranch) AddInputWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *WorkflowBranch {
	return wb.addDependencyRelation(fromNodeKey, inputs, getAddInputOpts(opts))
}

func (wb *WorkflowBranch) AddDependency(fromNodeKey string) *WorkflowBranch {
	return wb.addDependencyRelation(fromNodeKey, nil, &workflowAddInputOpts{dependencyWithoutInput: true})
}

func (wb *WorkflowBranch) addDependencyRelation(fromNodeKey string, inputs []*FieldMapping, options *workflowAddInputOpts) *WorkflowBranch {
	for _, input := range inputs {
		input.fromNodeKey = fromNodeKey
	}

	if len(inputs) == 0 && !options.dependencyWithoutInput {
		inputs = append(inputs, &FieldMapping{
			fromNodeKey: fromNodeKey,
		})
	}

	wb.inputs[fromNodeKey] = inputs

	if options.noDirectDependency {
		wb.inputAsIndirectEdge[fromNodeKey] = true
	}

	return wb
}

// AddEnd 创建 fromNodeKey 到 END 的流转关系，同时在 fromNodeKey 和 END 之间创建数据映射.
func (wf *Workflow[I, O]) AddEnd(fromNodeKey string, inputs ...*FieldMapping) *Workflow[I, O] {
	return wf.addEndDependencyRelation(fromNodeKey, inputs, &workflowAddInputOpts{})
}

func (wf *Workflow[I, O]) AddEndWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *Workflow[I, O] {
	return wf.addEndDependencyRelation(fromNodeKey, inputs, getAddInputOpts(opts))
}

func (wf *Workflow[I, O]) AddEndDependency(fromNodeKey string) *Workflow[I, O] {
	return wf.addEndDependencyRelation(fromNodeKey, nil, &workflowAddInputOpts{dependencyWithoutInput: true})
}

func (wf *Workflow[I, O]) addEndDependencyRelation(fromNodeKey string, inputs []*FieldMapping, options *workflowAddInputOpts) *Workflow[I, O] {
	for _, input := range inputs {
		input.fromNodeKey = fromNodeKey
	}

	if len(inputs) == 0 && !options.dependencyWithoutInput {
		inputs = append(inputs, &FieldMapping{
			fromNodeKey: fromNodeKey,
		})
	}

	if options.noDirectDependency {
		_ = wf.g.addIndirectEdgeWithMappings(fromNodeKey, END, inputs...)
	} else if options.dependencyWithoutInput {
		_ = wf.g.addControlOnlyEdgeWithMappings(fromNodeKey, END, inputs...)
	} else {
		_ = wf.g.addEdgeWithMappings(fromNodeKey, END, inputs...)
	}

	return wf
}

func (wf *Workflow[I, O]) compile(ctx context.Context, options *graphCompileOptions) (*composableRunnable, error) {
	for _, n := range wf.workflowNodes {
		const valueProviderSuffix = "\x1Fvalue\x1Fprovider"
		if len(n.staticValues) > 0 {
			provider := wf.AddLambdaNode(fmt.Sprintf("%s%s", n.key, valueProviderSuffix), valueProvider(n.staticValues)).AddInput(START)
			fieldMappings := make([]*FieldMapping, 0, len(n.staticValues))
			for path := range n.staticValues {
				fieldPath := splitFieldPath(path)
				fieldMappings = append(fieldMappings, MapFieldPaths(fieldPath, fieldPath))
			}
			n.AddInput(provider.key, fieldMappings...)
		}
	}

	for _, n := range wf.workflowNodes {
		for _, addInput := range n.addInputs {
			addInput()
		}
	}

	for _, wb := range wf.workflowBranches {
		passthrough := wf.addPassthroughNode(wb.branchKey)
		wf.g.nodes[wb.branchKey].cr.inputType = wb.GraphBranch.condition.inputType
		wf.g.nodes[wb.branchKey].cr.outputType = wf.g.nodes[wb.branchKey].cr.inputType
		wf.g.nodes[wb.branchKey].cr.inputConverter = wb.GraphBranch.condition.inputConverter
		wf.g.nodes[wb.branchKey].cr.inputFieldMappingConverter = wb.GraphBranch.condition.inputFieldMappingConverter
		for fromNodeKey, inputs := range wb.inputs {
			if wb.inputAsIndirectEdge[fromNodeKey] {
				passthrough.AddInputWithOptions(fromNodeKey, inputs, WithNoDirectDependency())
			} else {
				passthrough.AddInput(fromNodeKey, inputs...)
			}
		}

		for _, addInput := range passthrough.addInputs {
			addInput()
		}

		_ = wf.g.addBranch(passthrough.key, wb.GraphBranch, true)
	}

	// TODO: check indirect edges are legal

	return wf.g.compile(ctx, options)
}

func (wf *Workflow[I, O]) initNode(key string) *WorkflowNode {
	n := &WorkflowNode{g: wf.g, key: key, staticValues: make(map[string]any)}
	wf.workflowNodes[key] = n
	return n
}

func (wf *Workflow[I, O]) inputConverter() handlerPair {
	return handlerPair{
		invoke:    defaultValueChecker[I],
		transform: defaultStreamConverter[I],
	}
}

func (wf *Workflow[I, O]) inputFieldMappingConverter() handlerPair {
	return handlerPair{
		invoke:    buildFieldMappingConverter[I](),
		transform: buildStreamFieldMappingConverter[I](),
	}
}

func (wf *Workflow[I, O]) inputType() reflect.Type {
	return generic.TypeOf[I]()
}

func (wf *Workflow[I, O]) outputType() reflect.Type {
	return generic.TypeOf[O]()
}

func (wf *Workflow[I, O]) component() component {
	return wf.g.component()
}

func valueProvider(prefilledValues map[string]any) *Lambda {
	i := func(ctx context.Context, _ any, opts ...any) (map[string]any, error) {
		return prefilledValues, nil

	}

	return anyLambda(i, nil, nil, nil)
}
