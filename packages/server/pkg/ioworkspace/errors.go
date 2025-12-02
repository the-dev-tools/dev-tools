package ioworkspace

import "errors"

var (
	// ErrWorkspaceIDRequired is returned when a workspace ID is not provided
	ErrWorkspaceIDRequired = errors.New("workspace ID is required")

	// ErrInvalidMergeMode is returned when an invalid merge mode is specified
	ErrInvalidMergeMode = errors.New("invalid merge mode: must be 'skip', 'replace', or 'create_new'")

	// ErrInvalidExportFormat is returned when an invalid export format is specified
	ErrInvalidExportFormat = errors.New("invalid export format: must be 'json', 'yaml', or 'zip'")

	// ErrWorkspaceNotFound is returned when a workspace is not found
	ErrWorkspaceNotFound = errors.New("workspace not found")

	// ErrInvalidBundle is returned when a workspace bundle fails validation
	ErrInvalidBundle = errors.New("invalid workspace bundle")

	// ErrImportFailed is returned when an import operation fails
	ErrImportFailed = errors.New("import operation failed")

	// ErrExportFailed is returned when an export operation fails
	ErrExportFailed = errors.New("export operation failed")
)
