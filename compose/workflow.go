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
	"reflect"

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
	g         *graph
	key       string
	addInputs []func()
}

// Workflow is wrapper of Graph, replacing AddEdge with declaring FieldMapping between one node's output and current node's input.
// Under the hood it uses NodeTriggerMode(AllPredecessor), so does not support branches or cycles.
type Workflow[I, O any] struct {
	g                *graph
	branchCnt        int
	workflowNodes    map[string]*WorkflowNode
	err              error
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
		workflowNodes: make(map[string]*WorkflowNode),
	}

	return wf
}

func (wf *Workflow[I, O]) Compile(ctx context.Context, opts ...GraphCompileOption) (Runnable[I, O], error) {
	if len(globalGraphCompileCallbacks) > 0 {
		opts = append([]GraphCompileOption{WithGraphCompileCallbacks(globalGraphCompileCallbacks...)}, opts...)
	}
	option := newGraphCompileOptions(opts...)

	for _, n := range wf.workflowNodes {
		for _, addInput := range n.addInputs {
			addInput()
		}
	}

	for _, wb := range wf.workflowBranches {
		passthrough := wf.addPassthroughNode(wb.branchKey)
		for fromNodeKey, inputs := range wb.inputs {
			if wb.inputAsIndirectEdge[fromNodeKey] {
				passthrough.AddInputWithOptions(fromNodeKey, inputs, WithIndirectEdgeEnable())
			} else {
				passthrough.AddInput(fromNodeKey, inputs...)
			}
		}

		_ = wf.g.AddBranch(passthrough.key, wb.GraphBranch)
	}

	// TODO: check indirect edges are legal

	cr, err := wf.g.compile(ctx, option)
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

// AddInput 创建两个节点的流转关系（对应 UI 中的边），同时在两个节点间创建数据映射（对应节点 UI 里的输入配置）.
func (n *WorkflowNode) AddInput(fromNodeKey string, inputs ...*FieldMapping) *WorkflowNode {
	return n.addInput(fromNodeKey, inputs)
}

type workflowAddInputOpts struct {
	asIndirectEdge bool
}

type WorkflowAddInputOpt func(*workflowAddInputOpts)

// WithIndirectEdgeEnable AddInput 时，创建数据映射，但不创建控制流转关系.
// 即：当前 Node 可以引用特定前序 Node 的输出，但该前序 Node 的执行状态不影响当前 Node 是否可开始执行。
// 注意： 如果当前 Node 和存在数据映射关系的前序 Node 之间存在 branch，则务必通过 WithIndirectEdgeEnable 转化为 indirect edge.
// 注意: 当前 Node 必须为存在数据映射关系的 Node 的拓扑后续，否则数据映射关系无法成立。
// 注意： 使用此 Option 会导致失去节点间控制流转关系。
func WithIndirectEdgeEnable() WorkflowAddInputOpt {
	return func(opt *workflowAddInputOpts) {
		opt.asIndirectEdge = true
	}
}

func (n *WorkflowNode) AddInputWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *WorkflowNode {
	return n.addInput(fromNodeKey, inputs, opts...)
}

func (n *WorkflowNode) addInput(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *WorkflowNode {
	options := &workflowAddInputOpts{}
	for _, opt := range opts {
		opt(options)
	}

	for _, input := range inputs {
		input.fromNodeKey = fromNodeKey
	}

	if len(inputs) == 0 {
		inputs = append(inputs, &FieldMapping{
			fromNodeKey: fromNodeKey,
		})
	}

	if options.asIndirectEdge {
		n.addInputs = append(n.addInputs, func() {
			_ = n.g.addIndirectEdgeWithMappings(fromNodeKey, n.key, inputs...)
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
	return &WorkflowBranch{
		branchKey:           branchKey,
		GraphBranch:         branch,
		inputs:              make(map[string][]*FieldMapping),
		inputAsIndirectEdge: make(map[string]bool),
	}
}

func (wb *WorkflowBranch) AddInput(fromNodeKey string, inputs ...*FieldMapping) *WorkflowBranch {
	return wb.addInputWithOptions(fromNodeKey, inputs)
}

func (wb *WorkflowBranch) AddInputWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *WorkflowBranch {
	return wb.addInputWithOptions(fromNodeKey, inputs, opts...)
}

func (wb *WorkflowBranch) addInputWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *WorkflowBranch {
	options := &workflowAddInputOpts{}
	for _, opt := range opts {
		opt(options)
	}

	for _, input := range inputs {
		input.fromNodeKey = fromNodeKey
	}

	if len(inputs) == 0 {
		inputs = append(inputs, &FieldMapping{
			fromNodeKey: fromNodeKey,
		})
	}

	wb.inputs[fromNodeKey] = inputs

	if !options.asIndirectEdge {
		wb.inputAsIndirectEdge[fromNodeKey] = true
	}

	return wb
}

// AddEnd 创建 fromNodeKey 到 END 的流转关系，同时在 fromNodeKey 和 END 之间创建数据映射.
func (wf *Workflow[I, O]) AddEnd(fromNodeKey string, inputs ...*FieldMapping) *Workflow[I, O] {
	return wf.addEndWithOptions(fromNodeKey, inputs)
}

func (wf *Workflow[I, O]) AddEndWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *Workflow[I, O] {
	return wf.addEndWithOptions(fromNodeKey, inputs, opts...)
}

func (wf *Workflow[I, O]) addEndWithOptions(fromNodeKey string, inputs []*FieldMapping, opts ...WorkflowAddInputOpt) *Workflow[I, O] {
	options := &workflowAddInputOpts{}
	for _, opt := range opts {
		opt(options)
	}

	for _, input := range inputs {
		input.fromNodeKey = fromNodeKey
	}

	if len(inputs) == 0 {
		inputs = append(inputs, &FieldMapping{
			fromNodeKey: fromNodeKey,
		})
	}

	if options.asIndirectEdge {
		_ = wf.g.addIndirectEdgeWithMappings(fromNodeKey, END, inputs...)

	} else {
		_ = wf.g.addEdgeWithMappings(fromNodeKey, END, inputs...)
	}

	return wf
}

func (wf *Workflow[I, O]) compile(ctx context.Context, options *graphCompileOptions) (*composableRunnable, error) {
	if wf.err != nil {
		return nil, wf.err
	}

	for _, n := range wf.workflowNodes {
		for _, addInput := range n.addInputs {
			addInput()
		}
	}

	for _, wb := range wf.workflowBranches {
		passthrough := wf.addPassthroughNode(wb.branchKey)
		for fromNodeKey, inputs := range wb.inputs {
			if wb.inputAsIndirectEdge[fromNodeKey] {
				passthrough.AddInputWithOptions(fromNodeKey, inputs, WithIndirectEdgeEnable())
			} else {
				passthrough.AddInput(fromNodeKey, inputs...)
			}
		}

		_ = wf.g.AddBranch(passthrough.key, wb.GraphBranch)
	}

	// TODO: check indirect edges are legal

	return wf.g.compile(ctx, options)
}

func (wf *Workflow[I, O]) initNode(key string) *WorkflowNode {
	n := &WorkflowNode{g: wf.g, key: key}
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
