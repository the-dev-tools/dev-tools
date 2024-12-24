package runner

import (
	"context"
	"errors"
	"the-dev-tools/backend/pkg/idwrap"
)

var (
	ErrFlowRunnerNotImplemented = errors.New("FlowRunner not implemented")
	ErrNodeNotFound             = errors.New("Next Node not found")
)

type FlowRunner interface {
	Run(context.Context, chan FlowStatus) error
}

type FlowStatus struct {
	CurrentNodeID idwrap.IDWrap
}
