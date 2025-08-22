package sworkspace

// TODO: This file will contain the WorkspaceMovableRepository implementation
// once Phase 1-2 database schema is completed. For now, it's stubbed out
// to allow the rest of the RPC layer to compile and function.

// The repository will implement movable.MovableRepository for Workspaces
// Key difference from FlowVariable: UserID acts as the parent scope instead of a direct FK
// 
// When implemented, it should provide:
// - NewWorkspaceMovableRepository(queries *gen.Queries) *WorkspaceMovableRepository
// - TX support for transactions
// - UpdatePosition for single workspace position updates  
// - UpdatePositions for batch position updates
// - GetListOrdered for retrieving workspaces in user-defined order
//
// Implementation will require:
// - Database schema with prev/next columns in workspaces table
// - GetWorkspacesByUserIDOrdered SQL query
// - UpdateWorkspaceOrder SQL query
// - Proper linked-list ordering support with user-scoped isolation