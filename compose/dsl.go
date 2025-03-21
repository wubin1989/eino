package compose

type GraphDSL struct {
	ID              string            `json:"id" yaml:"id"`
	Namespace       string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name            *string           `json:"name,omitempty" yaml:"name,omitempty"`
	InputType       TypeID            `json:"input_type" yaml:"input_type"`
	StateType       *TypeID           `json:"state_type,omitempty" yaml:"state_type,omitempty"`
	NodeTriggerMode *NodeTriggerMode  `json:"node_trigger_mode,omitempty" yaml:"node_trigger_mode,omitempty"`
	MaxRunStep      *int              `json:"max_run_step,omitempty" yaml:"max_run_step,omitempty"`
	Nodes           []*NodeDSL        `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Edges           []*EdgeDSL        `json:"edges,omitempty" yaml:"edges,omitempty"`
	Branches        []*GraphBranchDSL `json:"branches,omitempty" yaml:"branches,omitempty"`
}

type WorkflowDSL struct {
	ID              string                  `json:"id" yaml:"id"`
	Namespace       string                  `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name            *string                 `json:"name" yaml:"name"`
	InputType       TypeID                  `json:"input_type" yaml:"input_type"`
	OutputType      TypeID                  `json:"output_type" yaml:"output_type"`
	StateType       *TypeID                 `json:"state_type,omitempty" yaml:"state_type,omitempty"`
	Nodes           []*WorkflowNodeDSL      `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Branches        []*WorkflowBranchDSL    `json:"branches,omitempty" yaml:"branches,omitempty"`
	EndInputs       []*WorkflowNodeInputDSL `json:"end_inputs,omitempty" yaml:"end_inputs,omitempty"`
	EndDependencies []string                `json:"end_dependencies,omitempty" yaml:"end_dependencies,omitempty"`
}

type TypeID string

type NodeDSL struct {
	Key              string          `json:"key" yaml:"key"`
	ImplID           string          `json:"impl_id,omitempty" yaml:"impl_id,omitempty"`
	Name             *string         `json:"name,omitempty" yaml:"name,omitempty"`
	Configs          []Config        `json:"configs,omitempty" yaml:"configs,omitempty"`
	InputKey         *string         `json:"input_key,omitempty" yaml:"input_key,omitempty"`
	OutputKey        *string         `json:"output_key,omitempty" yaml:"output_key,omitempty"`
	Graph            *GraphDSL       `json:"graph,omitempty" yaml:"graph,omitempty"`
	Workflow         *WorkflowDSL    `json:"workflow,omitempty" yaml:"workflow,omitempty"`
	StatePreHandler  *StateHandlerID `json:"state_pre_handler,omitempty" yaml:"state_pre_handler,omitempty"`
	StatePostHandler *StateHandlerID `json:"state_post_handler,omitempty" yaml:"state_post_handler,omitempty"`
}

type Config struct {
	Value any    `json:"value,omitempty" yaml:"value,omitempty"`
	Slot  *Slot  `json:"slot,omitempty" yaml:"slot,omitempty"`
	Slots []Slot `json:"slots,omitempty" yaml:"slots,omitempty"`
	isCtx bool
}

type Slot struct {
	TypeID TypeID `json:"type_id" yaml:"type_id"` // the actual type ID of the slot instance. BasicType should not be interface

	Path FieldPath `json:"path,omitempty" yaml:"path,omitempty"`

	Configs []Config `json:"configs,omitempty" yaml:"configs,omitempty"`
}

type EdgeDSL struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

type BranchDSL struct {
	Condition BranchFunctionID `json:"condition" yaml:"condition"`
	EndNodes  []string         `json:"end_nodes" yaml:"end_nodes"`
}

type BranchFunctionID string

type GraphBranchDSL struct {
	BranchDSL `json:"branch" yaml:"branch"`
	FromNode  string `json:"from_node" yaml:"from_node"`
}

type StateHandlerID string

type WorkflowNodeDSL struct {
	NodeDSL      `json:"node" yaml:"node"`
	Inputs       []*WorkflowNodeInputDSL `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Dependencies []string                `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	StaticValues []StaticValue           `json:"static_value,omitempty" yaml:"static_value,omitempty"`
}

type StaticValue struct {
	TypeID TypeID    `json:"type_id" yaml:"type_id"`
	Path   FieldPath `json:"path" yaml:"path"`
	Value  any       `json:"value" yaml:"value"`
}

type WorkflowNodeInputDSL struct {
	FromNodeKey        string             `json:"from_node_key" yaml:"from_node_key"`
	FieldPathMappings  []FieldPathMapping `json:"field_mappings,omitempty" yaml:"field_mappings,omitempty"`
	NoDirectDependency bool               `json:"no_direct_dependency,omitempty" yaml:"no_direct_dependency,omitempty"`
}

type FieldPathMapping struct {
	From FieldPath `json:"from" yaml:"from"`
	To   FieldPath `json:"to" yaml:"to"`
}

type WorkflowBranchDSL struct {
	Key          string `json:"key" yaml:"key"`
	BranchDSL    `json:"branch" yaml:"branch"`
	Inputs       []*WorkflowNodeInputDSL `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Dependencies []string                `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}
