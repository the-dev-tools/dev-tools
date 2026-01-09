package rimportv2

import (
	"context"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
)

// IntegrityError represents a specific integrity violation
type IntegrityError struct {
	EntityType string        // "file", "request_node", "http"
	EntityID   idwrap.IDWrap // ID of the problematic entity
	Field      string        // Which field has the issue
	RefID      idwrap.IDWrap // The referenced ID that is missing/invalid
	Message    string
}

func (e IntegrityError) Error() string {
	return fmt.Sprintf("%s %s: %s (field=%s, ref=%s)", e.EntityType, e.EntityID, e.Message, e.Field, e.RefID)
}

// IntegrityReport contains all integrity violations found
type IntegrityReport struct {
	Errors []IntegrityError
}

func (r *IntegrityReport) HasErrors() bool {
	return len(r.Errors) > 0
}

func (r *IntegrityReport) Error() string {
	if len(r.Errors) == 0 {
		return "no errors"
	}
	return fmt.Sprintf("%d integrity errors: first error: %s", len(r.Errors), r.Errors[0].Error())
}

func (r *IntegrityReport) AddError(entityType string, entityID idwrap.IDWrap, field string, refID idwrap.IDWrap, message string) {
	r.Errors = append(r.Errors, IntegrityError{
		EntityType: entityType,
		EntityID:   entityID,
		Field:      field,
		RefID:      refID,
		Message:    message,
	})
}

// ValidateImportIntegrity checks that all imported entities are correctly linked.
// This should be called after an import to verify data consistency.
func ValidateImportIntegrity(
	ctx context.Context,
	workspaceID idwrap.IDWrap,
	fileService *sfile.FileService,
	httpService *shttp.HTTPService,
	nodeService *sflow.NodeService,
	nodeRequestService *sflow.NodeRequestService,
) (*IntegrityReport, error) {
	report := &IntegrityReport{}

	// 1. Get all files in workspace
	files, err := fileService.ListFilesByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// Build HTTP ID set for quick lookup
	httpReqs, err := httpService.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list HTTP requests: %w", err)
	}
	httpIDSet := make(map[idwrap.IDWrap]bool)
	for _, h := range httpReqs {
		httpIDSet[h.ID] = true
	}

	// Also get deltas
	deltas, err := httpService.GetDeltasByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list HTTP deltas: %w", err)
	}
	for _, d := range deltas {
		httpIDSet[d.ID] = true
	}

	// Build file ID set
	fileIDSet := make(map[idwrap.IDWrap]bool)
	for _, f := range files {
		fileIDSet[f.ID] = true
	}

	// 2. Validate each file's ContentID points to existing HTTP
	for _, file := range files {
		if file.ContentID == nil {
			continue // Folders don't have ContentID
		}

		switch file.ContentType {
		case mfile.ContentTypeHTTP, mfile.ContentTypeHTTPDelta:
			if !httpIDSet[*file.ContentID] {
				report.AddError("file", file.ID, "ContentID", *file.ContentID,
					fmt.Sprintf("file references non-existent HTTP (type=%s)", file.ContentType))
			}
		case mfile.ContentTypeFlow, mfile.ContentTypeFolder:
			// Flow files reference flow IDs - we could validate these too
			// Folders don't have ContentID
			continue
		}
	}

	// 3. Validate each file's ParentID points to existing file
	for _, file := range files {
		if file.ParentID == nil {
			continue // Root files have no parent
		}
		if !fileIDSet[*file.ParentID] {
			report.AddError("file", file.ID, "ParentID", *file.ParentID,
				"file references non-existent parent file")
		}
	}

	// 4. Validate HTTP ParentHttpID points to existing HTTP
	for _, h := range httpReqs {
		if h.ParentHttpID != nil && !httpIDSet[*h.ParentHttpID] {
			report.AddError("http", h.ID, "ParentHttpID", *h.ParentHttpID,
				"HTTP references non-existent parent HTTP")
		}
	}
	for _, d := range deltas {
		if d.ParentHttpID != nil && !httpIDSet[*d.ParentHttpID] {
			report.AddError("http_delta", d.ID, "ParentHttpID", *d.ParentHttpID,
				"HTTP delta references non-existent parent HTTP")
		}
	}

	return report, nil
}

// ValidateTranslationResult validates the in-memory translation result before storage.
// This catches issues at the translation layer before they hit the database.
func ValidateTranslationResult(result *TranslationResult) *IntegrityReport {
	report := &IntegrityReport{}

	if result == nil {
		return report
	}

	// Build ID sets from translation result
	httpIDSet := make(map[idwrap.IDWrap]bool)
	for _, h := range result.HTTPRequests {
		httpIDSet[h.ID] = true
	}

	fileIDSet := make(map[idwrap.IDWrap]bool)
	for _, f := range result.Files {
		fileIDSet[f.ID] = true
	}

	// 1. Check that all files with HTTP content type have valid ContentIDs
	for _, file := range result.Files {
		if file.ContentID == nil {
			continue
		}

		switch file.ContentType {
		case mfile.ContentTypeHTTP, mfile.ContentTypeHTTPDelta:
			if !httpIDSet[*file.ContentID] {
				report.AddError("file", file.ID, "ContentID", *file.ContentID,
					fmt.Sprintf("file references HTTP not in translation result (type=%s)", file.ContentType))
			}
		case mfile.ContentTypeFlow, mfile.ContentTypeFolder:
			continue
		}
	}

	// 2. Check RequestNodes reference valid HTTP IDs
	for _, rn := range result.RequestNodes {
		if rn.HttpID != nil && !httpIDSet[*rn.HttpID] {
			report.AddError("request_node", rn.FlowNodeID, "HttpID", *rn.HttpID,
				"request node references HTTP not in translation result")
		}
		if rn.DeltaHttpID != nil && !httpIDSet[*rn.DeltaHttpID] {
			report.AddError("request_node", rn.FlowNodeID, "DeltaHttpID", *rn.DeltaHttpID,
				"request node references delta HTTP not in translation result")
		}
	}

	// 3. Check file parent references
	for _, file := range result.Files {
		if file.ParentID != nil && !fileIDSet[*file.ParentID] {
			report.AddError("file", file.ID, "ParentID", *file.ParentID,
				"file references parent not in translation result")
		}
	}

	return report
}
