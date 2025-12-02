// Package ioworkspace provides types and utilities for workspace import/export operations.
//
// This package defines the core data structures used to bundle all workspace entities
// (HTTP requests, flows, files, environments, etc.) for serialization, transfer, and
// deserialization across different formats (JSON, YAML, etc.).
//
// Key Types:
//
//   - WorkspaceBundle: Complete snapshot of all workspace entities
//   - ImportOptions: Configuration for import operations
//   - ExportOptions: Configuration for export operations
//
// The WorkspaceBundle structure serves as a comprehensive container for all workspace
// data, making it easy to:
//
//   - Export entire workspaces to various formats
//   - Import workspaces from external sources
//   - Clone or backup workspace state
//   - Migrate workspaces between environments
//
// Example usage:
//
//	// Create a bundle with workspace data
//	bundle := &ioworkspace.WorkspaceBundle{
//	    Workspace: workspace,
//	    HTTPRequests: httpRequests,
//	    Flows: flows,
//	    // ... other entities
//	}
//
//	// Get entity counts for logging
//	counts := bundle.CountEntities()
//	fmt.Printf("Exporting %d HTTP requests and %d flows\n",
//	    counts["http_requests"], counts["flows"])
//
//	// Find specific entities
//	flow := bundle.GetFlowByName("Main Flow")
//	http := bundle.GetHTTPByID(httpID)
package ioworkspace
