package movable

// CollectionListType represents different list types within collections
type CollectionListType int

const (
	CollectionListTypeUnspecified CollectionListType = iota
	CollectionListTypeFolders         // Folders within a collection
	CollectionListTypeEndpoints       // Endpoints within a folder
	CollectionListTypeExamples        // Examples within an endpoint
	CollectionListTypeCollections     // Collections within a workspace
)

func (c CollectionListType) String() string {
	switch c {
	case CollectionListTypeFolders:
		return "folders"
	case CollectionListTypeEndpoints:
		return "endpoints"
	case CollectionListTypeExamples:
		return "examples"
	case CollectionListTypeCollections:
		return "collections"
	default:
		return "unspecified"
	}
}

func (c CollectionListType) Value() int {
	return int(c)
}

// RequestListType represents different list types for request components
type RequestListType int

const (
	RequestListTypeUnspecified RequestListType = iota
	RequestListTypeHeaders          // HTTP headers
	RequestListTypeHeadersDeltas    // Header deltas
	RequestListTypeQueries          // Query parameters
	RequestListTypeQueriesDeltas    // Query parameter deltas
	RequestListTypeBodyForm         // Form body fields
	RequestListTypeBodyFormDeltas   // Form body field deltas
	RequestListTypeBodyUrlEncoded   // URL-encoded body fields
	RequestListTypeBodyUrlEncodedDeltas // URL-encoded body field deltas
)

func (r RequestListType) String() string {
	switch r {
	case RequestListTypeHeaders:
		return "headers"
	case RequestListTypeHeadersDeltas:
		return "headers_deltas"
	case RequestListTypeQueries:
		return "queries"
	case RequestListTypeQueriesDeltas:
		return "queries_deltas"
	case RequestListTypeBodyForm:
		return "body_form"
	case RequestListTypeBodyFormDeltas:
		return "body_form_deltas"
	case RequestListTypeBodyUrlEncoded:
		return "body_url_encoded"
	case RequestListTypeBodyUrlEncodedDeltas:
		return "body_url_encoded_deltas"
	default:
		return "unspecified"
	}
}

func (r RequestListType) Value() int {
	return int(r)
}

// FlowListType represents different list types for flow components
type FlowListType int

const (
	FlowListTypeUnspecified FlowListType = iota
	FlowListTypeNodes       // Flow nodes within a flow
	FlowListTypeEdges       // Flow edges within a flow
	FlowListTypeVariables   // Flow variables within a flow
)

func (f FlowListType) String() string {
	switch f {
	case FlowListTypeNodes:
		return "nodes"
	case FlowListTypeEdges:
		return "edges"
	case FlowListTypeVariables:
		return "variables"
	default:
		return "unspecified"
	}
}

func (f FlowListType) Value() int {
	return int(f)
}

// WorkspaceListType represents different list types within workspaces
type WorkspaceListType int

const (
	WorkspaceListTypeUnspecified WorkspaceListType = iota
	WorkspaceListTypeWorkspaces    // Workspaces (if hierarchical)
	WorkspaceListTypeEnvironments  // Environments within a workspace
	WorkspaceListTypeVariables     // Global variables within a workspace
	WorkspaceListTypeTags          // Tags within a workspace
)

func (w WorkspaceListType) String() string {
	switch w {
	case WorkspaceListTypeWorkspaces:
		return "workspaces"
	case WorkspaceListTypeEnvironments:
		return "environments"
	case WorkspaceListTypeVariables:
		return "variables"
	case WorkspaceListTypeTags:
		return "tags"
	default:
		return "unspecified"
	}
}

func (w WorkspaceListType) Value() int {
	return int(w)
}

// GetListTypeFromString returns the appropriate ListType from a string identifier
func GetListTypeFromString(listTypeStr string) ListType {
	// Collection list types
	switch listTypeStr {
	case "folders":
		return CollectionListTypeFolders
	case "endpoints":
		return CollectionListTypeEndpoints
	case "examples":
		return CollectionListTypeExamples
	case "collections":
		return CollectionListTypeCollections
	}
	
	// Request list types
	switch listTypeStr {
	case "headers":
		return RequestListTypeHeaders
	case "headers_deltas":
		return RequestListTypeHeadersDeltas
	case "queries":
		return RequestListTypeQueries
	case "queries_deltas":
		return RequestListTypeQueriesDeltas
	case "body_form":
		return RequestListTypeBodyForm
	case "body_form_deltas":
		return RequestListTypeBodyFormDeltas
	case "body_url_encoded":
		return RequestListTypeBodyUrlEncoded
	case "body_url_encoded_deltas":
		return RequestListTypeBodyUrlEncodedDeltas
	}
	
	// Flow list types
	switch listTypeStr {
	case "nodes":
		return FlowListTypeNodes
	case "edges":
		return FlowListTypeEdges
	case "variables":
		return FlowListTypeVariables
	}
	
	// Workspace list types
	switch listTypeStr {
	case "workspaces":
		return WorkspaceListTypeWorkspaces
	case "environments":
		return WorkspaceListTypeEnvironments
	case "variables":
		return WorkspaceListTypeVariables
	case "tags":
		return WorkspaceListTypeTags
	}
	
	return nil
}