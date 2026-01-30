package nai

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// ChildExecution represents an isolated execution context for child operations
// (provider calls, tool calls) within the orchestrator. Each child gets its own
// tracker so its data doesn't leak into the parent orchestrator's output.
type ChildExecution struct {
	ExecutionID idwrap.IDWrap
	ParentID    idwrap.IDWrap // Parent orchestrator's execution ID
	NodeID      idwrap.IDWrap
	Name        string
	Tracker     *tracking.VariableTracker
}

// NewChildExecution creates a new isolated execution context for a child operation.
func NewChildExecution(parentID, nodeID idwrap.IDWrap, name string) *ChildExecution {
	return &ChildExecution{
		ExecutionID: idwrap.NewMonotonic(),
		ParentID:    parentID,
		NodeID:      nodeID,
		Name:        name,
		Tracker:     tracking.NewVariableTracker(),
	}
}

// TrackInput records input data for this child execution.
func (e *ChildExecution) TrackInput(key string, value any) {
	if e.Tracker != nil {
		e.Tracker.TrackRead(key, value)
	}
}

// TrackOutput records output data for this child execution.
func (e *ChildExecution) TrackOutput(key string, value any) {
	if e.Tracker != nil {
		e.Tracker.TrackWrite(key, value)
	}
}

// GetInputData returns the tracked input data as a tree structure.
func (e *ChildExecution) GetInputData() map[string]any {
	if e.Tracker == nil {
		return nil
	}
	return e.Tracker.GetReadVarsAsTree()
}

// GetOutputData returns the tracked output data as a tree structure.
func (e *ChildExecution) GetOutputData() map[string]any {
	if e.Tracker == nil {
		return nil
	}
	return e.Tracker.GetWrittenVarsAsTree()
}

// EmitStatus emits a FlowNodeStatus for this child execution.
func (e *ChildExecution) EmitStatus(
	logFunc func(runner.FlowNodeStatus),
	state mflow.NodeState,
	err error,
	iterContext *runner.IterationContext,
	iterIndex int,
	loopNodeID idwrap.IDWrap,
) {
	if logFunc == nil {
		return
	}

	status := runner.FlowNodeStatus{
		ExecutionID:      e.ExecutionID,
		NodeID:           e.NodeID,
		Name:             e.Name,
		State:            state,
		IterationEvent:   true,
		IterationIndex:   iterIndex,
		LoopNodeID:       loopNodeID,
		IterationContext: iterContext,
	}

	if err != nil {
		status.Error = err
	}

	// Add tracked data
	if inputData := e.GetInputData(); len(inputData) > 0 {
		status.InputData = inputData
	}
	if outputData := e.GetOutputData(); len(outputData) > 0 {
		status.OutputData = outputData
	}

	logFunc(status)
}

// BuiltinToolExecution represents an execution of a built-in tool (get_variable, set_variable).
type BuiltinToolExecution struct {
	*ChildExecution
	ToolName string
}

// NewBuiltinToolExecution creates a new isolated execution for a built-in tool.
func NewBuiltinToolExecution(parentID, orchestratorID idwrap.IDWrap, toolName string, iterIndex int) *BuiltinToolExecution {
	name := toolName // Simple name for built-in tools
	return &BuiltinToolExecution{
		ChildExecution: NewChildExecution(parentID, orchestratorID, name),
		ToolName:       toolName,
	}
}

// ProviderExecution represents an execution of the LLM provider.
type ProviderExecution struct {
	*ChildExecution
	Iteration int
}

// NewProviderExecution creates a new isolated execution for a provider call.
func NewProviderExecution(parentID, providerNodeID idwrap.IDWrap, orchestratorName string, iteration int) *ProviderExecution {
	name := orchestratorName + " LLM Call"
	return &ProviderExecution{
		ChildExecution: NewChildExecution(parentID, providerNodeID, name),
		Iteration:      iteration,
	}
}

// NodeToolExecution represents an execution of a node tool (HTTP, etc.).
type NodeToolExecution struct {
	*ChildExecution
	ToolName string
}

// NewNodeToolExecution creates a new isolated execution for a node tool.
func NewNodeToolExecution(parentID, toolNodeID idwrap.IDWrap, toolName string) *NodeToolExecution {
	return &NodeToolExecution{
		ChildExecution: NewChildExecution(parentID, toolNodeID, toolName),
		ToolName:       toolName,
	}
}
