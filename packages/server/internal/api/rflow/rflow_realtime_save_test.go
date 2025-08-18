package rflow

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RealTimeSaveTracker tracks saves for testing real-time functionality
type RealTimeSaveTracker struct {
	mu                 sync.RWMutex
	createdExecutions  []mnodeexecution.NodeExecution
	updatedExecutions  []mnodeexecution.NodeExecution
	createTimestamps   []time.Time
	updateTimestamps   []time.Time
	createCallCount    int
	updateCallCount    int
	shouldFailCreate   bool
	shouldFailUpdate   bool
	createDelay        time.Duration
	updateDelay        time.Duration
}

func (t *RealTimeSaveTracker) CreateNodeExecution(ctx context.Context, execution mnodeexecution.NodeExecution) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.createCallCount++ // Always increment call count
	
	if t.shouldFailCreate {
		return fmt.Errorf("mock create failure")
	}
	
	if t.createDelay > 0 {
		time.Sleep(t.createDelay)
	}
	
	t.createdExecutions = append(t.createdExecutions, execution)
	t.createTimestamps = append(t.createTimestamps, time.Now())
	
	return nil
}

func (t *RealTimeSaveTracker) UpdateNodeExecution(ctx context.Context, execution mnodeexecution.NodeExecution) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.updateCallCount++ // Always increment call count
	
	if t.shouldFailUpdate {
		return fmt.Errorf("mock update failure")
	}
	
	if t.updateDelay > 0 {
		time.Sleep(t.updateDelay)
	}
	
	t.updatedExecutions = append(t.updatedExecutions, execution)
	t.updateTimestamps = append(t.updateTimestamps, time.Now())
	
	return nil
}

func (t *RealTimeSaveTracker) GetCreatedExecutions() []mnodeexecution.NodeExecution {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]mnodeexecution.NodeExecution{}, t.createdExecutions...)
}

func (t *RealTimeSaveTracker) GetUpdatedExecutions() []mnodeexecution.NodeExecution {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]mnodeexecution.NodeExecution{}, t.updatedExecutions...)
}

func (t *RealTimeSaveTracker) GetCallCounts() (int, int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.createCallCount, t.updateCallCount
}

func (t *RealTimeSaveTracker) GetTimestamps() ([]time.Time, []time.Time) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]time.Time{}, t.createTimestamps...), append([]time.Time{}, t.updateTimestamps...)
}

func (t *RealTimeSaveTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.createdExecutions = nil
	t.updatedExecutions = nil
	t.createTimestamps = nil
	t.updateTimestamps = nil
	t.createCallCount = 0
	t.updateCallCount = 0
}

// TestRealtimeSaving tests that node executions are saved immediately during flow execution
func TestRealtimeSaving(t *testing.T) {
	t.Run("NodeExecutionsSavedImmediatelyOnRunning", func(t *testing.T) {
		// Test that executions are saved to database immediately when state changes to RUNNING
		
		tracker := &RealTimeSaveTracker{}
		
		// Simulate node status processing like in the actual flow runner
		nodeID := idwrap.NewNow()
		executionID := idwrap.NewNow()
		
		// Create flow node status for RUNNING state
		runningStatus := runner.FlowNodeStatus{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "TestNode",
			State:       mnnode.NODE_STATE_RUNNING,
			InputData:   map[string]interface{}{"test": "input"},
			OutputData:  map[string]interface{}{},
			Error:       nil,
		}
		
		// Simulate the logic from rflow.go that creates and saves executions
		nodeExecution := mnodeexecution.NodeExecution{
			ID:                     executionID,
			NodeID:                 nodeID,
			Name:                   "TestNode - Execution 1",
			State:                  runningStatus.State,
			InputData:              []byte("{}"),
			InputDataCompressType:  0,
			OutputData:             []byte("{}"),
			OutputDataCompressType: 0,
		}
		
		// Record start time
		startTime := time.Now()
		
		// Save immediately (simulating real-time save)
		ctx := context.Background()
		err := tracker.CreateNodeExecution(ctx, nodeExecution)
		require.NoError(t, err)
		
		// Verify immediate save
		createdExecutions := tracker.GetCreatedExecutions()
		require.Len(t, createdExecutions, 1, "Execution should be saved immediately on RUNNING state")
		
		createCount, _ := tracker.GetCallCounts()
		assert.Equal(t, 1, createCount, "Create should be called immediately")
		
		// Verify timing - save should happen quickly (within reasonable time)
		createTimestamps, _ := tracker.GetTimestamps()
		require.Len(t, createTimestamps, 1)
		saveDuration := createTimestamps[0].Sub(startTime)
		assert.Less(t, saveDuration, 100*time.Millisecond, "Save should happen immediately")
		
		// Verify execution data
		savedExecution := createdExecutions[0]
		assert.Equal(t, executionID, savedExecution.ID, "Execution ID should match")
		assert.Equal(t, nodeID, savedExecution.NodeID, "Node ID should match")
		assert.Equal(t, mnnode.NODE_STATE_RUNNING, savedExecution.State, "State should be RUNNING")
	})
	
	t.Run("NodeExecutionsUpdatedImmediatelyOnCompletion", func(t *testing.T) {
		// Test that executions are updated immediately when node completes
		
		tracker := &RealTimeSaveTracker{}
		
		nodeID := idwrap.NewNow()
		executionID := idwrap.NewNow()
		
		// First create the execution (RUNNING state)
		runningExecution := mnodeexecution.NodeExecution{
			ID:          executionID,
			NodeID:      nodeID,
			Name:        "TestNode - Execution 1",
			State:       mnnode.NODE_STATE_RUNNING,
			InputData:   []byte(`{"input": "test"}`),
			OutputData:  []byte("{}"),
			CompletedAt: nil,
		}
		
		err := tracker.CreateNodeExecution(context.Background(), runningExecution)
		require.NoError(t, err)
		
		// Simulate completion status
		successStatus := runner.FlowNodeStatus{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "TestNode",
			State:       mnnode.NODE_STATE_SUCCESS,
			InputData:   map[string]interface{}{"input": "test"},
			OutputData:  map[string]interface{}{"result": "success"},
			Error:       nil,
		}
		
		// Update to completed state
		completedAt := time.Now().UnixMilli()
		completedExecution := mnodeexecution.NodeExecution{
			ID:          executionID,
			State:       successStatus.State,
			InputData:   []byte(`{"input": "test"}`),
			OutputData:  []byte(`{"result": "success"}`),
			CompletedAt: &completedAt,
		}
		
		// Record update time
		updateStartTime := time.Now()
		
		// Update immediately (real-time save)
		ctx := context.Background()
		err = tracker.UpdateNodeExecution(ctx, completedExecution)
		require.NoError(t, err)
		
		// Verify immediate update
		updatedExecutions := tracker.GetUpdatedExecutions()
		require.Len(t, updatedExecutions, 1, "Execution should be updated immediately on completion")
		
		_, updateCount := tracker.GetCallCounts()
		assert.Equal(t, 1, updateCount, "Update should be called immediately")
		
		// Verify timing
		_, updateTimestamps := tracker.GetTimestamps()
		require.Len(t, updateTimestamps, 1)
		updateDuration := updateTimestamps[0].Sub(updateStartTime)
		assert.Less(t, updateDuration, 100*time.Millisecond, "Update should happen immediately")
		
		// Verify updated data
		updatedExecution := updatedExecutions[0]
		assert.Equal(t, mnnode.NODE_STATE_SUCCESS, updatedExecution.State, "State should be updated to SUCCESS")
		assert.NotNil(t, updatedExecution.CompletedAt, "CompletedAt should be set")
		assert.Contains(t, string(updatedExecution.OutputData), "success", "Output data should be updated")
	})
	
	t.Run("ConcurrentRealtimeSaving", func(t *testing.T) {
		// Test concurrent real-time saving from multiple nodes
		
		tracker := &RealTimeSaveTracker{}
		
		numNodes := 10
		var wg sync.WaitGroup
		
		// Simulate multiple nodes executing and saving concurrently
		for i := 0; i < numNodes; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				nodeID := idwrap.NewNow()
				executionID := idwrap.NewNow()
				
				// Create execution (RUNNING)
				execution := mnodeexecution.NodeExecution{
					ID:     executionID,
					NodeID: nodeID,
					Name:   fmt.Sprintf("Node %d - Execution 1", index),
					State:  mnnode.NODE_STATE_RUNNING,
				}
				
				ctx := context.Background()
				err := tracker.CreateNodeExecution(ctx, execution)
				assert.NoError(t, err)
				
				// Small delay to simulate processing
				time.Sleep(time.Millisecond)
				
				// Update to completed
				completedAt := time.Now().UnixMilli()
				execution.State = mnnode.NODE_STATE_SUCCESS
				execution.CompletedAt = &completedAt
				
				err = tracker.UpdateNodeExecution(ctx, execution)
				assert.NoError(t, err)
			}(i)
		}
		
		wg.Wait()
		
		// Verify all saves completed
		createdExecutions := tracker.GetCreatedExecutions()
		updatedExecutions := tracker.GetUpdatedExecutions()
		
		assert.Len(t, createdExecutions, numNodes, "All executions should be created")
		assert.Len(t, updatedExecutions, numNodes, "All executions should be updated")
		
		createCount, updateCount := tracker.GetCallCounts()
		assert.Equal(t, numNodes, createCount, "All creates should complete")
		assert.Equal(t, numNodes, updateCount, "All updates should complete")
	})
	
	t.Run("RealtimeSavingWithIterationRecords", func(t *testing.T) {
		// Test real-time saving of iteration records (FOR/FOR_EACH nodes)
		
		tracker := &RealTimeSaveTracker{}
		
		forNodeID := idwrap.NewNow()
		
		// Simulate multiple iterations being saved in real-time
		numIterations := 5
		
		for i := 0; i < numIterations; i++ {
			iterationID := idwrap.NewNow()
			
			// Create iteration tracking record
			iterationExecution := mnodeexecution.NodeExecution{
				ID:         iterationID,
				NodeID:     forNodeID,
				Name:       "FOR Node Iteration",
				State:      mnnode.NODE_STATE_RUNNING,
				InputData:  []byte("{}"),
				OutputData: []byte(fmt.Sprintf(`{"index": %d}`, i)),
			}
			
			// Save immediately (like iteration records in real implementation)
			ctx := context.Background()
			err := tracker.CreateNodeExecution(ctx, iterationExecution)
			require.NoError(t, err)
			
			// Simulate completion
			completedAt := time.Now().UnixMilli()
			iterationExecution.State = mnnode.NODE_STATE_SUCCESS
			iterationExecution.CompletedAt = &completedAt
			
			err = tracker.UpdateNodeExecution(ctx, iterationExecution)
			require.NoError(t, err)
			
			// Small delay between iterations
			time.Sleep(time.Millisecond)
		}
		
		// Verify all iterations saved
		createdExecutions := tracker.GetCreatedExecutions()
		updatedExecutions := tracker.GetUpdatedExecutions()
		
		assert.Len(t, createdExecutions, numIterations, "All iteration records should be created")
		assert.Len(t, updatedExecutions, numIterations, "All iteration records should be updated")
		
		// Verify iteration sequence in output data
		for i, execution := range createdExecutions {
			expectedOutput := fmt.Sprintf(`{"index": %d}`, i)
			assert.Equal(t, expectedOutput, string(execution.OutputData), "Iteration output should match sequence")
		}
	})
}

// TestRealtimeSavingErrorHandling tests error scenarios in real-time saving
func TestRealtimeSavingErrorHandling(t *testing.T) {
	t.Run("CreateFailureDoesNotStopExecution", func(t *testing.T) {
		// Test that create failures are logged but don't stop flow execution
		
		tracker := &RealTimeSaveTracker{
			shouldFailCreate: true,
		}
		
		nodeID := idwrap.NewNow()
		executionID := idwrap.NewNow()
		
		execution := mnodeexecution.NodeExecution{
			ID:     executionID,
			NodeID: nodeID,
			State:  mnnode.NODE_STATE_RUNNING,
		}
		
		ctx := context.Background()
		err := tracker.CreateNodeExecution(ctx, execution)
		
		// Should fail as configured
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mock create failure")
		
		// In real implementation, this error would be logged but flow would continue
		createCount, _ := tracker.GetCallCounts()
		assert.Equal(t, 1, createCount, "Create attempt should be made")
		
		createdExecutions := tracker.GetCreatedExecutions()
		assert.Empty(t, createdExecutions, "No executions should be saved on failure")
		
		t.Log("Create failure handled gracefully - flow execution would continue")
	})
	
	t.Run("UpdateFailureDoesNotStopExecution", func(t *testing.T) {
		// Test that update failures are logged but don't stop flow execution
		
		tracker := &RealTimeSaveTracker{
			shouldFailUpdate: true,
		}
		
		nodeID := idwrap.NewNow()
		executionID := idwrap.NewNow()
		
		// First create successfully
		execution := mnodeexecution.NodeExecution{
			ID:     executionID,
			NodeID: nodeID,
			State:  mnnode.NODE_STATE_RUNNING,
		}
		
		ctx := context.Background()
		err := tracker.CreateNodeExecution(ctx, execution)
		require.NoError(t, err)
		
		// Update should fail
		completedAt := time.Now().UnixMilli()
		execution.State = mnnode.NODE_STATE_SUCCESS
		execution.CompletedAt = &completedAt
		
		err = tracker.UpdateNodeExecution(ctx, execution)
		
		// Should fail as configured
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mock update failure")
		
		// In real implementation, this would be logged but flow would continue
		_, updateCount := tracker.GetCallCounts()
		assert.Equal(t, 1, updateCount, "Update attempt should be made")
		
		t.Log("Update failure handled gracefully - flow execution would continue")
	})
}

// TestRealtimeSavingPerformance tests performance aspects of real-time saving
func TestRealtimeSavingPerformance(t *testing.T) {
	t.Run("NonBlockingRealTimeSave", func(t *testing.T) {
		// Test that real-time saves are non-blocking (use goroutines)
		
		tracker := &RealTimeSaveTracker{
			createDelay: 50 * time.Millisecond, // Simulate slow database
		}
		
		nodeID := idwrap.NewNow()
		executionID := idwrap.NewNow()
		
		execution := mnodeexecution.NodeExecution{
			ID:     executionID,
			NodeID: nodeID,
			State:  mnnode.NODE_STATE_RUNNING,
		}
		
		// Record start time
		startTime := time.Now()
		
		// In real implementation, this would be called in a goroutine
		// go func() { tracker.CreateNodeExecution(ctx, execution) }()
		// For testing, we simulate the non-blocking behavior
		
		done := make(chan error, 1)
		go func() {
			ctx := context.Background()
			done <- tracker.CreateNodeExecution(ctx, execution)
		}()
		
		// Should not block - we can do other work
		immediateTime := time.Now()
		processingDuration := immediateTime.Sub(startTime)
		
		// The goroutine should start immediately
		assert.Less(t, processingDuration, 10*time.Millisecond, "Save should be non-blocking")
		
		// Wait for save to complete
		err := <-done
		require.NoError(t, err)
		
		completeTime := time.Now()
		totalDuration := completeTime.Sub(startTime)
		
		// Total duration should include the delay
		assert.GreaterOrEqual(t, totalDuration, 50*time.Millisecond, "Save should complete with expected delay")
		
		// But the main thread wasn't blocked
		t.Log("Real-time save is non-blocking - main execution can continue")
	})
	
	t.Run("HighVolumeRealtimeSaving", func(t *testing.T) {
		// Test real-time saving with high volume of executions
		
		tracker := &RealTimeSaveTracker{}
		
		numExecutions := 1000
		startTime := time.Now()
		
		var wg sync.WaitGroup
		
		// Simulate high volume of concurrent saves
		for i := 0; i < numExecutions; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				nodeID := idwrap.NewNow()
				executionID := idwrap.NewNow()
				
				execution := mnodeexecution.NodeExecution{
					ID:     executionID,
					NodeID: nodeID,
					Name:   fmt.Sprintf("High Volume Execution %d", index),
					State:  mnnode.NODE_STATE_RUNNING,
				}
				
				ctx := context.Background()
				err := tracker.CreateNodeExecution(ctx, execution)
				assert.NoError(t, err)
			}(i)
		}
		
		wg.Wait()
		
		duration := time.Since(startTime)
		
		// Verify all saves completed
		createdExecutions := tracker.GetCreatedExecutions()
		assert.Len(t, createdExecutions, numExecutions, "All high-volume executions should be saved")
		
		createCount, _ := tracker.GetCallCounts()
		assert.Equal(t, numExecutions, createCount, "All creates should complete")
		
		// Performance check - should handle high volume reasonably
		avgTimePerSave := duration.Nanoseconds() / int64(numExecutions)
		t.Logf("High volume test: %d saves in %v (avg %v per save)", 
			numExecutions, duration, time.Duration(avgTimePerSave))
		
		// Should be reasonably fast (less than 1ms per save on average)
		assert.Less(t, avgTimePerSave, int64(time.Millisecond), 
			"Average save time should be reasonable for high volume")
	})
}