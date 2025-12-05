package ioworkspace

import (
	"context"
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/sfile"
)

// importFiles imports files from the bundle.
func (s *IOWorkspaceService) importFiles(ctx context.Context, fileService *sfile.FileService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, file := range bundle.Files {
		oldID := file.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			file.ID = idwrap.NewNow()
		}

		// Update workspace ID
		file.WorkspaceID = opts.WorkspaceID

		// Update parent folder ID
		if opts.ParentFolderID != nil {
			file.ParentID = opts.ParentFolderID
		} else if file.ParentID != nil {
			// Remap parent ID if it exists in file mapping
			if newParentID, ok := result.FileIDMap[*file.ParentID]; ok {
				file.ParentID = &newParentID
			}
		}

		// Update content ID references (HTTP or Flow)
		if file.ContentID != nil {
			if newContentID, ok := result.HTTPIDMap[*file.ContentID]; ok {
				file.ContentID = &newContentID
			} else if newContentID, ok := result.FlowIDMap[*file.ContentID]; ok {
				file.ContentID = &newContentID
			}
		}

		// Adjust order if needed
		if opts.StartOrder > 0 {
			file.Order = opts.StartOrder + file.Order
		}

		// Create file
		if err := fileService.CreateFile(ctx, &file); err != nil {
			return fmt.Errorf("failed to create file %s: %w", file.Name, err)
		}

		// Track ID mapping
		result.FileIDMap[oldID] = file.ID
		result.FilesCreated++
	}
	return nil
}
