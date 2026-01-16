package mutation

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"

// EntityType identifies the type of entity being mutated.
// Using uint16 for compact storage and no string comparisons at runtime.
type EntityType uint16

const (
	// Workspace entities
	EntityWorkspace EntityType = iota
	EntityWorkspaceUser
	EntityEnvironment
	EntityEnvironmentValue
	EntityTag

	// HTTP entities
	EntityHTTP
	EntityHTTPHeader
	EntityHTTPParam
	EntityHTTPBodyForm
	EntityHTTPBodyURL
	EntityHTTPBodyRaw
	EntityHTTPAssert
	EntityHTTPResponse
	EntityHTTPResponseHeader
	EntityHTTPResponseAssert
	EntityHTTPVersion

	// Flow entities
	EntityFlow
	EntityFlowNode
	EntityFlowNodeHTTP
	EntityFlowNodeFor
	EntityFlowNodeForEach
	EntityFlowNodeCondition
	EntityFlowNodeJS
	EntityFlowNodeAI
	EntityFlowNodeAiProvider
	EntityFlowNodeMemory
	EntityFlowEdge
	EntityFlowVariable
	EntityFlowTag

	// File system
	EntityFile

	// Credential entities
	EntityCredential
)

// Operation identifies the type of mutation.
type Operation uint8

const (
	OpInsert Operation = iota
	OpUpdate
	OpDelete
)

// String returns the string representation of the operation.
// Used for event type in sync streaming ("insert", "update", "delete").
func (o Operation) String() string {
	switch o {
	case OpInsert:
		return "insert"
	case OpUpdate:
		return "update"
	case OpDelete:
		return "delete"
	default:
		return ""
	}
}

// Event represents a single mutation event.
// Events are collected during a mutation transaction and published on commit.
type Event struct {
	Entity      EntityType
	Op          Operation
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	ParentID    idwrap.IDWrap // For child entities - the parent ID (e.g., FlowID for nodes/edges/variables)
	IsDelta     bool
	Payload     any // For insert/update - the entity data
	Patch       any // For update - the changed fields
}
