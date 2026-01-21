using DevTools;

namespace Api.Flow;

@TanStackDB.collection
model Flow {
  @primaryKey flowId: Id;
  @foreignKey @visibility(Lifecycle.Create) workspaceId: Id;
  name: string;
  @visibility(Lifecycle.Read) duration?: int32;
  @visibility(Lifecycle.Read) running: boolean;
}

model FlowDuplicateRequest {
  flowId: Id;
}

op FlowDuplicate(...FlowDuplicateRequest): {};

model FlowRunRequest {
  flowId: Id;
}

op FlowRun(...FlowRunRequest): {};

model FlowStopRequest {
  flowId: Id;
}

op FlowStop(...FlowStopRequest): {};

@TanStackDB.collection(#{ isReadOnly: true })
model FlowVersion {
  @primaryKey flowVersionId: Id;
  @foreignKey flowId: Id;
}

@TanStackDB.collection
model FlowVariable {
  @primaryKey flowVariableId: Id;
  ...CommonTableFields<Flow>;
}

enum HandleKind {
  Then,
  Else,
  Loop,
}

@TanStackDB.collection
model Edge {
  @primaryKey edgeId: Id;
  @foreignKey flowId: Id;
  @foreignKey sourceId: Id;
  @foreignKey targetId: Id;
  sourceHandle: HandleKind;
  @visibility(Lifecycle.Read) state: FlowItemState;
}

enum NodeKind {
  ManualStart,
  Http,
  Condition,
  For,
  ForEach,
  Js,
}

enum FlowItemState {
  Running,
  Success,
  Failure,
  Canceled,
}

model Position {
  x: float32;
  y: float32;
}

@TanStackDB.collection
model Node {
  @primaryKey nodeId: Id;
  @foreignKey flowId: Id;
  kind: NodeKind;
  name: string;
  position: Position;
  @visibility(Lifecycle.Read) state: FlowItemState;
  @visibility(Lifecycle.Read) info?: string;
}

@TanStackDB.collection
model NodeHttp {
  @primaryKey nodeId: Id;
  @foreignKey httpId: Id;
  @foreignKey deltaHttpId?: Id;
}

enum ErrorHandling {
  Ignore,
  Break,
}

@TanStackDB.collection
model NodeFor {
  @primaryKey nodeId: Id;
  iterations: int32;
  condition: string;
  errorHandling: ErrorHandling;
}

@TanStackDB.collection
model NodeForEach {
  @primaryKey nodeId: Id;
  path: string;
  condition: string;
  errorHandling: ErrorHandling;
}

@TanStackDB.collection
model NodeCondition {
  @primaryKey nodeId: Id;
  condition: string;
}

@TanStackDB.collection
model NodeJs {
  @primaryKey nodeId: Id;
  code: string;
}

// =============================================================================
// AI Tool Definitions - Mutation Tools
// =============================================================================

@doc("Create a new JavaScript node in the workflow. JS nodes can transform data, make calculations, or perform custom logic.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Create JavaScript Node" })
model CreateJsNode {
  @doc("The ULID of the workflow to add the node to")
  flowId: Id;
  name: string;
  code: string;
  position?: Position;
}

@doc("Create a new HTTP request node that makes an API call.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Create HTTP Node" })
model CreateHttpNode {
  @doc("The ULID of the workflow to add the node to")
  flowId: Id;
  name: string;
  @doc("The ULID of the HTTP request definition to use")
  httpId: Id;
  position?: Position;
}

@doc("Create a condition node that routes flow based on a boolean expression. Has THEN and ELSE output handles.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Create Condition Node" })
model CreateConditionNode {
  @doc("The ULID of the workflow to add the node to")
  flowId: Id;
  name: string;
  condition: string;
  position?: Position;
}

@doc("Create a for-loop node that iterates a fixed number of times.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Create For Loop Node" })
model CreateForNode {
  @doc("The ULID of the workflow to add the node to")
  flowId: Id;
  name: string;
  @doc("Number of iterations to perform")
  iterations: int32;
  @doc("Optional condition to continue loop using expr-lang syntax (e.g., \"i < 10\"). Use == for equality (NOT ===)")
  condition: string;
  errorHandling: ErrorHandling;
  position?: Position;
}

@doc("Create a forEach node that iterates over an array or object.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Create ForEach Loop Node" })
model CreateForEachNode {
  @doc("The ULID of the workflow to add the node to")
  flowId: Id;
  name: string;
  @doc("Path to the array/object to iterate (e.g., \"input.items\")")
  path: string;
  @doc("Optional condition to continue iteration using expr-lang syntax. Use == for equality (NOT ===)")
  condition: string;
  errorHandling: ErrorHandling;
  position?: Position;
}

@doc("Update the JavaScript code of a JS node.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Update Node Code" })
model UpdateNodeCode {
  @doc("The ULID of the JS node to update")
  nodeId: Id;
  code: string;
}

@doc("Update general node properties like name or position.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Update Node Config" })
model UpdateNodeConfig {
  @doc("The ULID of the node to update")
  nodeId: Id;
  @doc("New display name (optional)")
  name?: string;
  position?: Position;
}

@doc("Create an edge connection between two nodes. IMPORTANT: For sequential flows (Manual Start, JS, HTTP nodes), do NOT specify sourceHandle - omit it entirely. Only use sourceHandle for Condition nodes (then/else) and Loop nodes (loop/then).")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Connect Nodes" })
model ConnectNodes {
  @doc("The ULID of the workflow")
  flowId: Id;
  @doc("The ULID of the source node")
  sourceId: Id;
  @doc("The ULID of the target node")
  targetId: Id;
  @doc("Output handle for branching nodes ONLY. Use \"then\"/\"else\" for Condition nodes, \"loop\"/\"then\" for For/ForEach nodes. OMIT this parameter for Manual Start, JS, and HTTP nodes.")
  sourceHandle?: HandleKind;
}

@doc("Remove an edge connection between nodes.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Disconnect Nodes" })
model DisconnectNodes {
  @doc("The ULID of the edge to remove")
  edgeId: Id;
}

@doc("Delete a node from the workflow. Also removes all connected edges.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Delete Node" })
model DeleteNode {
  @doc("The ULID of the node to delete")
  nodeId: Id;
}

@doc("Create a new workflow variable that can be referenced in node expressions.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Create Variable" })
model CreateVariable {
  @doc("The ULID of the workflow")
  flowId: Id;
  @doc("Variable name (used to reference it in expressions)")
  key: string;
  @doc("Variable value")
  value: string;
  @doc("Description of what the variable is for (optional)")
  description?: string;
  @doc("Whether the variable is active (default: true)")
  enabled?: boolean;
}

@doc("Update an existing workflow variable.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Mutation, title: "Update Variable" })
model UpdateVariable {
  @doc("The ULID of the variable to update")
  flowVariableId: Id;
  @doc("New variable name (optional)")
  key?: string;
  @doc("New variable value (optional)")
  value?: string;
  @doc("New description (optional)")
  description?: string;
  @doc("Whether the variable is active (optional)")
  enabled?: boolean;
}

// =============================================================================
// AI Tool Definitions - Exploration Tools
// =============================================================================

@doc("Get the complete workflow graph including all nodes and edges. Use this to understand the current structure of the workflow.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Exploration, title: "Get Workflow Graph" })
model GetWorkflowGraph {
  @doc("The ULID of the workflow to retrieve")
  flowId: Id;
}

@doc("Get detailed information about a specific node including its configuration, code (for JS nodes), and connections.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Exploration, title: "Get Node Details" })
model GetNodeDetails {
  @doc("The ULID of the node to inspect")
  nodeId: Id;
}

@doc("Read a markdown template that provides guidance on implementing a specific type of node or pattern.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Exploration, title: "Get Node Template" })
model GetNodeTemplate {
  @doc("The name of the template to retrieve (e.g., \"http-aggregator\", \"js-transformer\")")
  templateName: string;
}

@doc("Search for available templates by keyword or pattern.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Exploration, title: "Search Templates" })
model SearchTemplates {
  @doc("Search query to find relevant templates")
  query: string;
}

@doc("Get the history of past workflow executions.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Exploration, title: "Get Execution History" })
model GetExecutionHistory {
  @doc("The ULID of the workflow")
  flowId: Id;
  @doc("Maximum number of executions to return (default: 10)")
  limit?: int32;
}

@doc("Get the latest execution logs. Returns only the most recent execution per node to avoid showing full history.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Exploration, title: "Get Execution Logs" })
model GetExecutionLogs {
  @doc("Filter to only show executions for nodes in this workflow")
  flowId?: Id;
  @doc("Maximum number of node executions to return (default: 10)")
  limit?: int32;
  @doc("Optional: specific execution ID to get logs for")
  executionId?: Id;
}

@doc("Search for API documentation by name, description, or keywords. Returns lightweight metadata for matching APIs. Use getApiDocs to load full documentation for a specific API.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Exploration, title: "Search API Docs" })
model SearchApiDocs {
  @doc("Search query - API name, description keywords, or use case (e.g., \"send message\", \"payment\", \"telegram bot\")")
  query: string;
  @doc("Category of the API")
  category?: string;
  @doc("Maximum results to return (default: 5)")
  limit?: int32;
}

@doc("Load full documentation for a specific API. Call this after searchApiDocs to get complete endpoint details, authentication info, and examples.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Exploration, title: "Get API Docs" })
model GetApiDocs {
  @doc("API identifier from search results (e.g., \"slack\", \"stripe\", \"telegram\")")
  apiId: string;
  @doc("Force refresh from source, bypassing cache")
  forceRefresh?: boolean;
  @doc("Optional filter to focus on specific endpoint (e.g., \"chat.postMessage\", \"sendMessage\")")
  endpoint?: string;
}

// =============================================================================
// AI Tool Definitions - Execution Tools
// =============================================================================

@doc("Execute the workflow from the start node. Returns execution status.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Execution, title: "Run Workflow" })
model RunWorkflow {
  @doc("The ULID of the workflow to run")
  flowId: Id;
}

@doc("Stop a running workflow execution.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Execution, title: "Stop Workflow" })
model StopWorkflow {
  @doc("The ULID of the workflow to stop")
  flowId: Id;
}

@doc("Validate the workflow for errors, missing connections, or configuration issues. Use this before running to catch problems.")
@AITools.aiTool(#{ category: AITools.ToolCategory.Execution, title: "Validate Workflow" })
model ValidateWorkflow {
  @doc("The ULID of the workflow to validate")
  flowId: Id;
}

@TanStackDB.collection(#{ isReadOnly: true })
model NodeExecution {
  @primaryKey nodeExecutionId: Id;
  @foreignKey nodeId: Id;
  name: string;
  state: FlowItemState;
  error?: string;
  input?: Protobuf.WellKnown.Json;
  output?: Protobuf.WellKnown.Json;
  httpResponseId?: Id;
  completedAt?: Protobuf.WellKnown.Timestamp;
}
