package runner

import (
	"errors"
	"the-dev-tools/backend/pkg/idwrap"
)

var (
	ErrFlowRunnerNotImplemented = errors.New("FlowRunner not implemented")
	ErrNodeNotFound             = errors.New("Node not found")
)

type FlowRunner interface {
	Run(chan FlowStatus) error
}

type FlowStatus struct {
	CurrentNodeID idwrap.IDWrap
}
