package mflow

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
)

// NodeExpect is a flow node's expect: block (AI checks).
type NodeExpect struct {
	FlowNodeID idwrap.IDWrap
	Expect     mexpect.Expect
}

// FlowAIChecks are a flow's AI check settings: its judge:, quality: and iterations:, with the
// file-level judge: and quality: folded in.
type FlowAIChecks struct {
	FlowID   idwrap.IDWrap
	Settings mexpect.FlowSettings
}
