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
	g                *graph
	key              string
	addInputs        []func()
	staticValues     map[string]any
	dependencySetter func(fromNodeKey string, typ dependencyType)
}

// Workflow is wrapper of graph, replacing AddEdge with declaring dependencies and field mappings between nodes.
// Under the hood it uses NodeTriggerMode(AllPredecessor), so does not support cycles.
type Workflow[I, O any] struct {
	g                *graph
	workflowNodes    map[string]*WorkflowNode
	workflowBranches map[string]*WorkflowBranch
	dependencies     map[string]map[string]dependencyType
}

type dependencyType int

const (
	directDependency dependencyType = iota
	noDirectDependency
	branchDependency
)

func withInputType(typ reflect.Type) NewGraphOption {
	return func(ngo *newGraphOptions) {
		ngo.inputType = typ
	}
}

func withOutputType(typ reflect.Type) NewGraphOption {
	return func(ngo *newGraphOptions) {
		ngo.outputType = typ
	}
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
			options.inputType,
			options.outputType,
		),
		workflowNodes:    make(map[string]*WorkflowNode),
		workflowBranches: make(map[string]*WorkflowBranch),
		dependencies:     make(map[string]map[string]dependencyType),
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

// AddInput creates both data and execution dependencies between nodes.
// It configures how data flows from the predecessor node (fromNodeKey) to the current node,
// and ensures the current node only executes after the predecessor completes.
//
// Parameters:
//   - fromNodeKey: the key of the predecessor node
//   - inputs: field mappings that specify how data should flow from the predecessor
//     to the current node. If no mappings are provided, the entire output of the
//     predecessor will be used as input.
//
// Example:
//
//	// Map between specific field
//	node.AddInput("userNode", MapFields("user.name", "displayName"))
//
//	// Use entire output
//	node.AddInput("dataNode")
//
// Returns the current node for method chaining.
func (n *WorkflowNode) AddInput(fromNodeKey string, inputs ...*FieldMapping) *WorkflowNode {
	return n.addDependencyRelation(fromNodeKey, inputs, &workflowAddInputOpts{})
}

type workflowAddInputOpts struct {
	// noDirectDependency indicates whether to create a data mapping without establishing
	// a direct execution dependency. When true, the current node can access data from
	// the predecessor node but its execution is not directly blocked by it.
	noDirectDependency bool
	// dependencyWithoutInput indicates whether to create an execution dependency
	// without any data mapping. When true, the current node will wait for the
	// predecessor node to complete but won't receive any data from it.
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

// WithNoDirectDependency creates a data mapping without establishing a direct execution dependency.
// The predecessor node will still complete before the current node executes, but through indirect
// execution paths rather than a direct dependency.
//
// In a workflow graph, node dependencies typically serve two purposes:
// 1. Execution order: determining when nodes should run
// 2. Data flow: specifying how data passes between nodes
//
// This option separates these concerns by:
//   - Creating data mapping from the predecessor to the current node
//   - Relying on the predecessor's path to reach the current node through other nodes
//     that have direct execution dependencies
//
// Example:
//
//	node.AddInputWithOptions("dataNode", mappings, WithNoDirectDependency())
//
// Important:
//
//  1. Branch scenarios: When connecting nodes on different sides of a branch,
//     WithNoDirectDependency MUST be used to let the branch itself handle the
//     execution order, preventing incorrect dependencies that could bypass the branch.
//
//  2. Execution guarantee: The predecessor will still complete before the current
//     node executes because the predecessor must have a path (through other nodes)
//     that eventually reaches the current node.
//
//  3. Graph validity: There MUST be a path from the predecessor that eventually
//     reaches the current node through other nodes with direct dependencies.
//     This ensures the execution order while avoiding redundant direct dependencies.
//
// Common use cases:
// - Cross-branch data access where the branch handles execution order
// - Avoiding redundant dependencies when a path already exists
func WithNoDirectDependency() WorkflowAddInputOpt {
	return func(opt *workflowAddInputOpts) {
		opt.noDirectDependency = true
	}
}

// AddInputWithOptions creates a dependency between nodes with custom configuration options.
// It allows fine-grained control over both data flow and execution dependencies.
//
// Parameters:
//   - fromNodeKey: the key of the predecessor node
//   - inputs: field mappings that specify how data flows from the predecessor to the current node.
//     If no mappings are provided, the entire output of the predecessor will be used as input.
//   - opts: configuration options that control how the dependency is established
//
// Example:
//
//	// Create data mapping without direct execution dependency
//	node.AddInputWithOptions("dataNode", mappings, WithNoDirectDependency())
//
// Returns the current node for method chaining.
func (n *WorkflowNode) AddInputWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *WorkflowNode {
	return n.addDependencyRelation(fromNodeKey, inputs, getAddInputOpts(opts))
}

// AddDependency creates an execution-only dependency between nodes.
// The current node will wait for the predecessor node to complete before executing,
// but no data will be passed between them.
//
// Parameters:
//   - fromNodeKey: the key of the predecessor node that must complete before this node starts
//
// Example:
//
//	// Wait for "setupNode" to complete before executing
//	node.AddDependency("setupNode")
//
// This is useful when:
// - You need to ensure execution order without data transfer
// - The predecessor performs setup or initialization that must complete first
// - You want to explicitly separate execution dependencies from data flow
//
// Returns the current node for method chaining.
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
			n.dependencySetter(fromNodeKey, noDirectDependency)
		})
	} else if options.dependencyWithoutInput {
		n.addInputs = append(n.addInputs, func() {
			_ = n.g.addControlOnlyEdgeWithMappings(fromNodeKey, n.key, inputs...)
			n.dependencySetter(fromNodeKey, directDependency)
		})
	} else {
		n.addInputs = append(n.addInputs, func() {
			_ = n.g.addEdgeWithMappings(fromNodeKey, n.key, inputs...)
			n.dependencySetter(fromNodeKey, directDependency)
		})
	}

	return n
}

type WorkflowBranch struct {
	branchKey string
	*GraphBranch
	inputs                      map[string][]*FieldMapping
	inputWithNoDirectDependency map[string]bool
	dependenciesWithoutInput    []string
	actualInputType             reflect.Type
}

type addWorkflowBranchOptions struct {
	actualInputType reflect.Type
}

type AddWorkflowBranchOption func(*addWorkflowBranchOptions)

func withBranchInputType(typ reflect.Type) AddWorkflowBranchOption {
	return func(args *addWorkflowBranchOptions) {
		args.actualInputType = typ
	}
}

func (wf *Workflow[I, O]) AddBranch(branchKey string, branch *GraphBranch, opts ...AddWorkflowBranchOption) *WorkflowBranch {
	options := &addWorkflowBranchOptions{}
	for _, opt := range opts {
		opt(options)
	}

	wb := &WorkflowBranch{
		branchKey:                   branchKey,
		GraphBranch:                 branch,
		inputs:                      make(map[string][]*FieldMapping),
		inputWithNoDirectDependency: make(map[string]bool),
		actualInputType:             options.actualInputType,
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

	if options.noDirectDependency {
		wb.inputWithNoDirectDependency[fromNodeKey] = true
		wb.inputs[fromNodeKey] = inputs
	} else if options.dependencyWithoutInput {
		wb.dependenciesWithoutInput = append(wb.dependenciesWithoutInput, fromNodeKey)
	} else {
		wb.inputs[fromNodeKey] = inputs
	}

	return wb
}

func (wb *WorkflowBranch) conditionInputType() reflect.Type {
	if wb.actualInputType != nil {
		return wb.actualInputType
	}

	return wb.condition.inputType
}

func (wb *WorkflowBranch) conditionFieldMappingConverter() handlerPair {
	if wb.actualInputType != nil {
		return handlerPair{
			invoke:    buildFieldMappingConverterWithReflect(wb.actualInputType),
			transform: buildStreamFieldMappingConverterWithReflect(wb.actualInputType),
		}
	}

	return wb.condition.inputFieldMappingConverter
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

	if _, ok := wf.dependencies[END]; !ok {
		wf.dependencies[END] = make(map[string]dependencyType)
	}

	if options.noDirectDependency {
		_ = wf.g.addIndirectEdgeWithMappings(fromNodeKey, END, inputs...)
		wf.dependencies[END][fromNodeKey] = noDirectDependency
	} else if options.dependencyWithoutInput {
		_ = wf.g.addControlOnlyEdgeWithMappings(fromNodeKey, END, inputs...)
		wf.dependencies[END][fromNodeKey] = directDependency
	} else {
		_ = wf.g.addEdgeWithMappings(fromNodeKey, END, inputs...)
		wf.dependencies[END][fromNodeKey] = directDependency
	}

	return wf
}

func (wf *Workflow[I, O]) controlledByBranch(key string) bool {
	fromNodeKey2DepType, ok := wf.dependencies[key]
	if !ok {
		return false
	}

	for pre, dep := range fromNodeKey2DepType {
		if dep == branchDependency {
			return true
		}

		if wf.controlledByBranch(pre) {
			return true
		}
	}

	return false
}

func (wf *Workflow[I, O]) compile(ctx context.Context, options *graphCompileOptions) (*composableRunnable, error) {
	if wf.g.buildError != nil {
		return nil, wf.g.buildError
	}

	for _, n := range wf.workflowNodes {
		for _, addInput := range n.addInputs {
			addInput()
		}
		n.addInputs = nil
	}

	for _, wb := range wf.workflowBranches {
		passthrough := wf.addPassthroughNode(wb.branchKey)
		wf.g.nodes[wb.branchKey].cr.inputType = wb.conditionInputType()
		wf.g.nodes[wb.branchKey].cr.outputType = wf.g.nodes[wb.branchKey].cr.inputType
		wf.g.nodes[wb.branchKey].cr.inputConverter = wb.GraphBranch.condition.inputConverter
		wf.g.nodes[wb.branchKey].cr.inputFieldMappingConverter = wb.conditionFieldMappingConverter()
		for fromNodeKey, inputs := range wb.inputs {
			if wb.inputWithNoDirectDependency[fromNodeKey] {
				passthrough.AddInputWithOptions(fromNodeKey, inputs, WithNoDirectDependency())
			} else {
				passthrough.AddInput(fromNodeKey, inputs...)
			}
		}
		for i := range wb.dependenciesWithoutInput {
			passthrough.AddDependency(wb.dependenciesWithoutInput[i])
		}

		for _, addInput := range passthrough.addInputs {
			addInput()
		}
		passthrough.addInputs = nil

		_ = wf.g.addBranch(passthrough.key, wb.GraphBranch, true)

		for endNode := range wb.endNodes {
			if endNode == END {
				if _, ok := wf.dependencies[END]; !ok {
					wf.dependencies[END] = make(map[string]dependencyType)
				}
				wf.dependencies[END][passthrough.key] = branchDependency
			} else {
				n := wf.workflowNodes[endNode]
				n.dependencySetter(passthrough.key, branchDependency)
			}
		}
	}

	for _, n := range wf.workflowNodes {
		const valueProviderSuffix = "\x1Fvalue\x1Fprovider"
		if len(n.staticValues) > 0 {
			provider := wf.AddLambdaNode(fmt.Sprintf("%s%s", n.key, valueProviderSuffix), valueProvider(n.staticValues)).AddInput(START)
			fieldMappings := make([]*FieldMapping, 0, len(n.staticValues))
			for path := range n.staticValues {
				fieldPath := splitFieldPath(path)
				fieldMappings = append(fieldMappings, MapFieldPaths(fieldPath, fieldPath))
			}

			if wf.controlledByBranch(n.key) {
				n.AddInputWithOptions(provider.key, fieldMappings, WithNoDirectDependency())
			} else {
				n.AddInput(provider.key, fieldMappings...)
			}

			for _, addInput := range provider.addInputs {
				addInput()
			}
			for _, addInput := range n.addInputs {
				addInput()
			}
		}
	}

	// TODO: check indirect edges are legal

	return wf.g.compile(ctx, options)
}

func (wf *Workflow[I, O]) initNode(key string) *WorkflowNode {
	n := &WorkflowNode{
		g:            wf.g,
		key:          key,
		staticValues: make(map[string]any),
		dependencySetter: func(fromNodeKey string, typ dependencyType) {
			if _, ok := wf.dependencies[key]; !ok {
				wf.dependencies[key] = make(map[string]dependencyType)
			}
			wf.dependencies[key][fromNodeKey] = typ
		}}
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
