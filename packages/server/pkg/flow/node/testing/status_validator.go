package testing

import (
	"fmt"
	"strings"
	"time"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

// ValidationError represents a validation error with detailed context.
type ValidationError struct {
	Type        string
	Description string
	NodeID      idwrap.IDWrap
	ExecutionID idwrap.IDWrap
	Details     map[string]any
}

// Error implements the error interface.
func (ve ValidationError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", ve.Type, ve.Description))

	if ve.NodeID != (idwrap.IDWrap{}) {
		parts = append(parts, fmt.Sprintf("node=%s", ve.NodeID.String()))
	}

	if ve.ExecutionID != (idwrap.IDWrap{}) {
		parts = append(parts, fmt.Sprintf("execution=%s", ve.ExecutionID.String()))
	}

	if len(ve.Details) > 0 {
		var detailParts []string
		for k, v := range ve.Details {
			detailParts = append(detailParts, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("details=[%s]", strings.Join(detailParts, ", ")))
	}

	return strings.Join(parts, " ")
}

// ValidationErrors is a collection of ValidationError instances.
type ValidationErrors []ValidationError

// Error implements the error interface for the collection.
func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "no validation errors"
	}

	if len(ve) == 1 {
		return ve[0].Error()
	}

	var messages []string
	for _, err := range ve {
		messages = append(messages, err.Error())
	}

	return fmt.Sprintf("%d validation errors:\n%s", len(ve), strings.Join(messages, "\n"))
}

// StatusValidator validates FlowNodeStatus sequences and consistency.
type StatusValidator struct {
	collector *StatusCollector
	errors    ValidationErrors
}

// NewStatusValidator creates a new StatusValidator with the given StatusCollector.
func NewStatusValidator(collector *StatusCollector) *StatusValidator {
	return &StatusValidator{
		collector: collector,
		errors:    make(ValidationErrors, 0),
	}
}

// ValidateAll performs comprehensive validation of all captured statuses.
// Returns ValidationErrors if any issues are found, nil otherwise.
func (sv *StatusValidator) ValidateAll() error {
	sv.errors = make(ValidationErrors, 0)

	statuses := sv.collector.GetAll()
	if len(statuses) == 0 {
		return nil
	}

	// Run all validation checks
	sv.validateExecutionSequences(statuses)
	sv.validateStatusTransitions(statuses)
	sv.validateExecutionIDUniqueness(statuses)
	sv.validateIterationContext(statuses)
	sv.validateTimingConsistency(statuses)
	sv.validateLoopNodeConsistency(statuses)

	if len(sv.errors) > 0 {
		return sv.errors
	}
	return nil
}

// ValidateExecutionSequences ensures every RUNNING status has a corresponding final status.
func (sv *StatusValidator) ValidateExecutionSequences() error {
	sv.errors = make(ValidationErrors, 0)
	sv.validateExecutionSequences(sv.collector.GetAll())

	if len(sv.errors) > 0 {
		return sv.errors
	}
	return nil
}

// validateExecutionSequences checks that RUNNING statuses are properly terminated.
func (sv *StatusValidator) validateExecutionSequences(statuses []TimestampedStatus) {
	// Group by execution ID
	executions := make(map[idwrap.IDWrap][]TimestampedStatus)
	for _, ts := range statuses {
		execID := ts.Status.ExecutionID
		executions[execID] = append(executions[execID], ts)
	}

	// Validate each execution sequence
	for execID, execStatuses := range executions {
		sv.validateExecutionSequence(execID, execStatuses)
	}
}

// validateExecutionSequence validates a single execution's status sequence.
func (sv *StatusValidator) validateExecutionSequence(execID idwrap.IDWrap, statuses []TimestampedStatus) {
	if len(statuses) == 0 {
		return
	}

	// Sort by timestamp (they should already be in order from collector)
	// but we'll rely on the collector's ordering for performance

	var runningFound bool
	var finalStates []mnnode.NodeState

	for _, ts := range statuses {
		state := ts.Status.State

		switch state {
		case mnnode.NODE_STATE_RUNNING:
			if runningFound {
				// Multiple RUNNING states without final state
				sv.addError("multiple_running",
					"multiple RUNNING states without final state",
					ts.Status.NodeID, execID, map[string]any{
						"previous_running": true,
					})
			}
			runningFound = true

		case mnnode.NODE_STATE_SUCCESS, mnnode.NODE_STATE_FAILURE, mnnode.NODE_STATE_CANCELED:
			finalStates = append(finalStates, state)
			runningFound = false

		case mnnode.NODE_STATE_UNSPECIFIED:
			// Skip unspecified states
		}
	}

	// Check if we have a RUNNING without final state
	if runningFound {
		lastStatus := statuses[len(statuses)-1]
		sv.addError("unterminated_running",
			"RUNNING status without corresponding final state",
			lastStatus.Status.NodeID, execID, nil)
	}

	// Check for multiple final states
	if len(finalStates) > 1 {
		lastStatus := statuses[len(statuses)-1]
		sv.addError("multiple_final",
			fmt.Sprintf("multiple final states: %v", finalStates),
			lastStatus.Status.NodeID, execID, map[string]any{
				"final_states": finalStates,
			})
	}
}

// ValidateStatusTransitions ensures status transitions follow valid patterns.
func (sv *StatusValidator) ValidateStatusTransitions() error {
	sv.errors = make(ValidationErrors, 0)
	sv.validateStatusTransitions(sv.collector.GetAll())

	if len(sv.errors) > 0 {
		return sv.errors
	}
	return nil
}

// validateStatusTransitions checks that status transitions are valid.
func (sv *StatusValidator) validateStatusTransitions(statuses []TimestampedStatus) {
	// Group by node ID to track transitions per node
	nodeStatuses := make(map[idwrap.IDWrap][]TimestampedStatus)
	for _, ts := range statuses {
		nodeID := ts.Status.NodeID
		nodeStatuses[nodeID] = append(nodeStatuses[nodeID], ts)
	}

	// Validate transitions for each node
	for nodeID, nodeStatusList := range nodeStatuses {
		sv.validateNodeTransitions(nodeID, nodeStatusList)
	}
}

// validateNodeTransitions validates status transitions for a single node.
func (sv *StatusValidator) validateNodeTransitions(nodeID idwrap.IDWrap, statuses []TimestampedStatus) {
	if len(statuses) < 2 {
		return
	}

	for i := 1; i < len(statuses); i++ {
		prev := statuses[i-1].Status.State
		curr := statuses[i].Status.State

		if !sv.isValidTransition(prev, curr) {
			sv.addError("invalid_transition",
				fmt.Sprintf("invalid transition from %s to %s",
					mnnode.StringNodeState(prev), mnnode.StringNodeState(curr)),
				nodeID, statuses[i].Status.ExecutionID, map[string]any{
					"from_state": mnnode.StringNodeState(prev),
					"to_state":   mnnode.StringNodeState(curr),
				})
		}
	}
}

// isValidTransition checks if a transition between states is valid.
func (sv *StatusValidator) isValidTransition(from, to mnnode.NodeState) bool {
	// Allow any transition to RUNNING
	if to == mnnode.NODE_STATE_RUNNING {
		return true
	}

	// Allow transitions from RUNNING to final states
	if from == mnnode.NODE_STATE_RUNNING {
		return to == mnnode.NODE_STATE_SUCCESS ||
			to == mnnode.NODE_STATE_FAILURE ||
			to == mnnode.NODE_STATE_CANCELED
	}

	// Allow transitions from final states to RUNNING (for retries)
	if (from == mnnode.NODE_STATE_SUCCESS || from == mnnode.NODE_STATE_FAILURE || from == mnnode.NODE_STATE_CANCELED) &&
		to == mnnode.NODE_STATE_RUNNING {
		return true
	}

	// Allow same state (idempotent updates)
	if from == to {
		return true
	}

	return false
}

// ValidateExecutionIDUniqueness ensures execution IDs are properly used.
func (sv *StatusValidator) ValidateExecutionIDUniqueness() error {
	sv.errors = make(ValidationErrors, 0)
	sv.validateExecutionIDUniqueness(sv.collector.GetAll())

	if len(sv.errors) > 0 {
		return sv.errors
	}
	return nil
}

// validateExecutionIDUniqueness checks execution ID usage patterns.
func (sv *StatusValidator) validateExecutionIDUniqueness(statuses []TimestampedStatus) {
	// Check that execution IDs are not reused across different nodes
	nodeExecutions := make(map[idwrap.IDWrap]map[idwrap.IDWrap]bool) // nodeID -> executionIDs

	for _, ts := range statuses {
		nodeID := ts.Status.NodeID
		execID := ts.Status.ExecutionID

		if nodeExecutions[nodeID] == nil {
			nodeExecutions[nodeID] = make(map[idwrap.IDWrap]bool)
		}

		// Check if this execution ID was already used for a different node
		for otherNodeID, execIDs := range nodeExecutions {
			if otherNodeID != nodeID && execIDs[execID] {
				sv.addError("execution_id_reuse",
					"execution ID reused across different nodes",
					nodeID, execID, map[string]any{
						"other_node": otherNodeID,
					})
				return
			}
		}

		nodeExecutions[nodeID][execID] = true
	}
}

// ValidateIterationContext ensures iteration context is consistent.
func (sv *StatusValidator) ValidateIterationContext() error {
	sv.errors = make(ValidationErrors, 0)
	sv.validateIterationContext(sv.collector.GetAll())

	if len(sv.errors) > 0 {
		return sv.errors
	}
	return nil
}

// validateIterationContext checks iteration context consistency.
func (sv *StatusValidator) validateIterationContext(statuses []TimestampedStatus) {
	for _, ts := range statuses {
		status := ts.Status

		// If IterationEvent is true, IterationContext should not be nil
		if status.IterationEvent && status.IterationContext == nil {
			sv.addError("missing_iteration_context",
				"iteration event without iteration context",
				status.NodeID, status.ExecutionID, nil)
		}

		// If LoopNodeID is set, IterationContext should be present
		if status.LoopNodeID != (idwrap.IDWrap{}) && status.IterationContext == nil {
			sv.addError("loop_without_context",
				"loop node ID without iteration context",
				status.NodeID, status.ExecutionID, map[string]any{
					"loop_node_id": status.LoopNodeID.String(),
				})
		}

		// Validate iteration context structure
		if status.IterationContext != nil {
			sv.validateIterationContextStructure(status)
		}
	}
}

// validateIterationContextStructure validates the structure of iteration context.
func (sv *StatusValidator) validateIterationContextStructure(status runner.FlowNodeStatus) {
	ctx := status.IterationContext

	// IterationPath should not be empty for nested iterations
	if len(ctx.IterationPath) == 0 && len(ctx.ParentNodes) > 0 {
		sv.addError("invalid_iteration_path",
			"empty iteration path with parent nodes",
			status.NodeID, status.ExecutionID, map[string]any{
				"parent_nodes_count": len(ctx.ParentNodes),
			})
	}

	// ParentNodes and IterationPath should have consistent lengths
	if len(ctx.ParentNodes) != len(ctx.IterationPath) {
		sv.addError("inconsistent_iteration_data",
			"parent nodes and iteration path length mismatch",
			status.NodeID, status.ExecutionID, map[string]any{
				"parent_nodes_len":   len(ctx.ParentNodes),
				"iteration_path_len": len(ctx.IterationPath),
			})
	}

	// ExecutionIndex should be non-negative
	if ctx.ExecutionIndex < 0 {
		sv.addError("invalid_execution_index",
			"negative execution index",
			status.NodeID, status.ExecutionID, map[string]any{
				"execution_index": ctx.ExecutionIndex,
			})
	}
}

// ValidateTimingConsistency ensures timing information is reasonable.
func (sv *StatusValidator) ValidateTimingConsistency() error {
	sv.errors = make(ValidationErrors, 0)
	sv.validateTimingConsistency(sv.collector.GetAll())

	if len(sv.errors) > 0 {
		return sv.errors
	}
	return nil
}

// validateTimingConsistency checks timing information for consistency.
func (sv *StatusValidator) validateTimingConsistency(statuses []TimestampedStatus) {
	for _, ts := range statuses {
		status := ts.Status

		// RunDuration should be non-negative
		if status.RunDuration < 0 {
			sv.addError("negative_duration",
				"negative run duration",
				status.NodeID, status.ExecutionID, map[string]any{
					"duration": status.RunDuration.String(),
				})
		}

		// For very long durations, warn (but don't error)
		if status.RunDuration > 24*time.Hour {
			sv.addError("excessive_duration",
				"excessive run duration (may indicate error)",
				status.NodeID, status.ExecutionID, map[string]any{
					"duration": status.RunDuration.String(),
				})
		}
	}
}

// ValidateLoopNodeConsistency ensures loop node relationships are consistent.
func (sv *StatusValidator) ValidateLoopNodeConsistency() error {
	sv.errors = make(ValidationErrors, 0)
	sv.validateLoopNodeConsistency(sv.collector.GetAll())

	if len(sv.errors) > 0 {
		return sv.errors
	}
	return nil
}

// validateLoopNodeConsistency checks loop node relationships.
func (sv *StatusValidator) validateLoopNodeConsistency(statuses []TimestampedStatus) {
	// Group by loop node ID to validate loop execution
	loopExecutions := make(map[idwrap.IDWrap][]TimestampedStatus)

	for _, ts := range statuses {
		if ts.Status.LoopNodeID != (idwrap.IDWrap{}) {
			loopID := ts.Status.LoopNodeID
			loopExecutions[loopID] = append(loopExecutions[loopID], ts)
		}
	}

	// Validate each loop's execution pattern
	for loopID, loopStatuses := range loopExecutions {
		sv.validateLoopExecution(loopID, loopStatuses)
	}
}

// validateLoopExecution validates the execution pattern of a single loop.
func (sv *StatusValidator) validateLoopExecution(loopID idwrap.IDWrap, statuses []TimestampedStatus) {
	// Check that iteration indices are reasonable
	for _, ts := range statuses {
		if ts.Status.IterationIndex < 0 {
			sv.addError("negative_iteration_index",
				"negative iteration index",
				ts.Status.NodeID, ts.Status.ExecutionID, map[string]any{
					"iteration_index": ts.Status.IterationIndex,
					"loop_node_id":    loopID.String(),
				})
		}
	}

	// Check for reasonable iteration progression
	// (This is a basic check - more sophisticated validation could be added)
	iterationIndices := make(map[int]int)
	for _, ts := range statuses {
		iterationIndices[ts.Status.IterationIndex]++
	}

	// Warn if we have too many iterations of the same index
	for index, count := range iterationIndices {
		if count > 10 {
			sv.addError("excessive_iteration_repeats",
				"excessive repeats of same iteration index",
				loopID, idwrap.IDWrap{}, map[string]any{
					"iteration_index": index,
					"repeat_count":    count,
				})
		}
	}
}

// addError adds a validation error to the collection.
func (sv *StatusValidator) addError(errorType, description string, nodeID, executionID idwrap.IDWrap, details map[string]any) {
	sv.errors = append(sv.errors, ValidationError{
		Type:        errorType,
		Description: description,
		NodeID:      nodeID,
		ExecutionID: executionID,
		Details:     details,
	})
}

// GetErrors returns all validation errors from the last validation run.
func (sv *StatusValidator) GetErrors() ValidationErrors {
	return sv.errors
}

// HasErrors returns true if there are validation errors.
func (sv *StatusValidator) HasErrors() bool {
	return len(sv.errors) > 0
}

// ErrorCount returns the number of validation errors.
func (sv *StatusValidator) ErrorCount() int {
	return len(sv.errors)
}
