package flowlocalrunner

import (
	"context"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// buildTerminalStatus consolidates state classification, data attachment, and
// output flattening into a single function. Both strategies call this instead
// of duplicating ~50 lines each.
//
// base must have ExecutionID, NodeID, Name, IterationContext, RunDuration,
// and AuxiliaryID pre-filled. This function fills State, Error, InputData,
// and OutputData.
func buildTerminalStatus(
	base runner.FlowNodeStatus,
	nodeErr error,
	timedOut bool,
	trackedInput, trackedOutput map[string]any,
	req *node.FlowNodeRequest,
	trackData bool,
) runner.FlowNodeStatus {
	status := base

	if nodeErr != nil {
		switch {
		case timedOut || errors.Is(nodeErr, context.DeadlineExceeded):
			status.State = mflow.NODE_STATE_FAILURE
		case runner.IsCancellationError(nodeErr):
			status.State = mflow.NODE_STATE_CANCELED
		default:
			status.State = mflow.NODE_STATE_FAILURE
		}
		status.Error = nodeErr
	} else {
		status.State = mflow.NODE_STATE_SUCCESS
	}

	if trackData {
		if len(trackedOutput) > 0 {
			status.OutputData = node.DeepCopyValue(trackedOutput)
		} else {
			status.OutputData = collectSingleModeOutput(req, status.Name)
		}
		if len(trackedInput) > 0 {
			status.InputData = node.DeepCopyValue(trackedInput)
		}
	}
	status.OutputData = flattenNodeOutput(status.Name, status.OutputData)

	return status
}

func collectSingleModeOutput(req *node.FlowNodeRequest, nodeName string) any {
	if nodeName == "" {
		return nil
	}
	if data, err := node.ReadVarRaw(req, nodeName); err == nil {
		return node.DeepCopyValue(data)
	}
	return nil
}

func flattenNodeOutput(nodeName string, output any) any {
	if nodeName == "" || output == nil {
		return output
	}
	m, ok := output.(map[string]any)
	if !ok {
		return output
	}
	nested, ok := m[nodeName]
	if !ok {
		return output
	}
	nestedMap, ok := nested.(map[string]any)
	if !ok {
		return output
	}
	delete(m, nodeName)
	for k, v := range nestedMap {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return m
}
