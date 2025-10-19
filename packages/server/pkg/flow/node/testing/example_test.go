package testing_test

import (
	"fmt"
	"testing"
	"time"

	flowtesting "the-dev-tools/server/pkg/flow/node/testing"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

// ExampleStatusCollector demonstrates basic usage of the StatusCollector.
func ExampleStatusCollector() {
	// Create a new status collector
	collector := flowtesting.NewStatusCollector()
	defer collector.Close()

	// Create some test statuses
	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	runningStatus := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "example-node",
		State:       mnnode.NODE_STATE_RUNNING,
		RunDuration: 100 * time.Millisecond,
	}

	successStatus := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "example-node",
		State:       mnnode.NODE_STATE_SUCCESS,
		RunDuration: 250 * time.Millisecond,
	}

	// Capture the statuses
	collector.Capture(runningStatus)
	collector.Capture(successStatus)

	// Query captured statuses
	allStatuses := collector.GetAll()
	runningStatuses := collector.GetByState(mnnode.NODE_STATE_RUNNING)
	successStatuses := collector.GetByState(mnnode.NODE_STATE_SUCCESS)

	// Output: Captured 2 statuses: 1 RUNNING, 1 SUCCESS
	fmt.Printf("Captured %d statuses: %d RUNNING, %d SUCCESS\n",
		len(allStatuses), len(runningStatuses), len(successStatuses))
}

// ExampleStatusValidator demonstrates validation of status sequences.
func ExampleStatusValidator() {
	// Create collector and validator
	collector := flowtesting.NewStatusCollector()
	defer collector.Close()
	validator := flowtesting.NewStatusValidator(collector)

	// Create a valid execution sequence
	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	statuses := []runner.FlowNodeStatus{
		{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "example-node",
			State:       mnnode.NODE_STATE_RUNNING,
		},
		{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "example-node",
			State:       mnnode.NODE_STATE_SUCCESS,
		},
	}

	// Capture statuses
	for _, status := range statuses {
		collector.Capture(status)
	}

	// Validate all captured statuses
	err := validator.ValidateAll()
	if err != nil {
		fmt.Printf("Validation failed: %s\n", err.Error())
	} else {
		fmt.Println("Validation passed - all status sequences are valid")
	}
}

// ExampleTestContext demonstrates comprehensive testing setup.
func ExampleTestContext() {
	// This would typically be used inside a test function
	t := &testing.T{} // In real usage, this would be the actual *testing.T

	// Create a test context with custom options
	tc := flowtesting.NewTestContext(t, flowtesting.TestContextOptions{
		Timeout:         10 * time.Second,
		EnableDebugLogs: true,
		AutoValidate:    true,
	})

	// Create a node request with automatic status capture
	nodeID := idwrap.NewNow()
	req := tc.CreateNodeRequest(nodeID, "example-node")

	// Simulate node execution by emitting statuses
	status := tc.CreateTestStatus(nodeID, req.ExecutionID, "example-node", mnnode.NODE_STATE_RUNNING)
	req.LogPushFunc(status)

	// Wait for the status to be captured
	filter := flowtesting.StatusFilter{
		NodeID: &nodeID,
		State:  &[]mnnode.NodeState{mnnode.NODE_STATE_RUNNING}[0],
	}

	capturedStatus, err := tc.WaitForStatus(filter)
	if err != nil {
		fmt.Printf("Error waiting for status: %s\n", err.Error())
		return
	}

	stateStr := mnnode.StringNodeState(capturedStatus.Status.State)
	if stateStr == "Running" {
		stateStr = "RUNNING"
	}
	fmt.Printf("Captured status: %s for node %s\n",
		stateStr, capturedStatus.Status.Name)

	// Assert expected counts
	tc.AssertStatusCount(1)
	tc.AssertStateCount(mnnode.NODE_STATE_RUNNING, 1)

	// Note: ValidateAndAssert() is called automatically on cleanup due to AutoValidate: true

	// Output: Captured status: RUNNING for node example-node
}
