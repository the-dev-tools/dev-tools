//nolint:revive // exported
package node

// EntryNode marks a node as a valid flow entry point (no incoming edges expected).
// The runner collects all EntryNodes and starts them concurrently.
type EntryNode interface {
	FlowNode
	IsEntryNode() bool
}

// ListenerEntry is an entry node that runs for the flow's lifetime,
// receiving events in a loop (e.g., WebSocket Connection).
// It implements LoopCoordinator so the runner doesn't apply per-node timeout.
type ListenerEntry interface {
	EntryNode
	LoopCoordinator
}

// TriggerEntry is an entry node whose external event initiates the flow run.
// Examples: Webhook (HTTP request triggers flow), Queue (message triggers flow).
// The trigger payload is written to VarMap before downstream nodes execute.
type TriggerEntry interface {
	EntryNode
	// TriggerType returns a string identifier for the trigger kind (e.g., "webhook", "queue").
	TriggerType() string
}
