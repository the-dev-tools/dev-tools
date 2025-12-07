package ioworkspace

import (
	"context"
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/shttp"
)

// importHTTPRequests imports HTTP requests from the bundle.
func (s *IOWorkspaceService) importHTTPRequests(ctx context.Context, httpService shttp.HTTPService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, http := range bundle.HTTPRequests {
		oldID := http.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			http.ID = idwrap.NewNow()
		}

		// Update workspace ID
		http.WorkspaceID = opts.WorkspaceID

		// Update folder ID if specified
		if opts.ParentFolderID != nil {
			http.FolderID = opts.ParentFolderID
		} else if http.FolderID != nil {
			// Remap folder ID if it exists in file mapping
			if newFolderID, ok := result.FileIDMap[*http.FolderID]; ok {
				http.FolderID = &newFolderID
			}
		}

		// Update parent HTTP ID if it exists in the mapping (for deltas)
		if http.ParentHttpID != nil {
			if newParentID, ok := result.HTTPIDMap[*http.ParentHttpID]; ok {
				http.ParentHttpID = &newParentID
			}
		}

		// Create HTTP request
		if err := httpService.Create(ctx, &http); err != nil {
			return fmt.Errorf("failed to create HTTP request %s: %w", http.Name, err)
		}

		// Track ID mapping
		result.HTTPIDMap[oldID] = http.ID
		result.HTTPRequestsCreated++
	}
	return nil
}

// importHTTPHeaders imports HTTP headers from the bundle.
func (s *IOWorkspaceService) importHTTPHeaders(ctx context.Context, headerService shttp.HttpHeaderService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, header := range bundle.HTTPHeaders {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			header.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[header.HttpID]; ok {
			header.HttpID = newHTTPID
		}

		// Remap parent header ID if it exists in the mapping
		if header.ParentHttpHeaderID != nil {
			// Note: We'd need to track header ID mappings for this to work properly
			// For now, we'll clear parent references on import
			header.ParentHttpHeaderID = nil
			header.IsDelta = false
		}

		// Create header
		if err := headerService.Create(ctx, &header); err != nil {
			return fmt.Errorf("failed to create HTTP header: %w", err)
		}

		result.HTTPHeadersCreated++
	}
	return nil
}

// importHTTPSearchParams imports HTTP search params from the bundle.
func (s *IOWorkspaceService) importHTTPSearchParams(ctx context.Context, searchParamService *shttp.HttpSearchParamService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, param := range bundle.HTTPSearchParams {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			param.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[param.HttpID]; ok {
			param.HttpID = newHTTPID
		}

		// Clear parent references (similar to headers)
		if param.ParentHttpSearchParamID != nil {
			param.ParentHttpSearchParamID = nil
			param.IsDelta = false
		}

		// Create search param
		if err := searchParamService.Create(ctx, &param); err != nil {
			return fmt.Errorf("failed to create HTTP search param: %w", err)
		}

		result.HTTPSearchParamsCreated++
	}
	return nil
}

// importHTTPBodyForms imports HTTP body forms from the bundle.
func (s *IOWorkspaceService) importHTTPBodyForms(ctx context.Context, bodyFormService *shttp.HttpBodyFormService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, bodyForm := range bundle.HTTPBodyForms {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			bodyForm.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[bodyForm.HttpID]; ok {
			bodyForm.HttpID = newHTTPID
		}

		// Clear parent references
		if bodyForm.ParentHttpBodyFormID != nil {
			bodyForm.ParentHttpBodyFormID = nil
			bodyForm.IsDelta = false
		}

		// Create body form
		if err := bodyFormService.Create(ctx, &bodyForm); err != nil {
			return fmt.Errorf("failed to create HTTP body form: %w", err)
		}

		result.HTTPBodyFormsCreated++
	}
	return nil
}

// importHTTPBodyUrlencoded imports HTTP body urlencoded from the bundle.
func (s *IOWorkspaceService) importHTTPBodyUrlencoded(ctx context.Context, bodyUrlencodedService *shttp.HttpBodyUrlEncodedService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, bodyUrlencoded := range bundle.HTTPBodyUrlencoded {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			bodyUrlencoded.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[bodyUrlencoded.HttpID]; ok {
			bodyUrlencoded.HttpID = newHTTPID
		}

		// Clear parent references
		if bodyUrlencoded.ParentHttpBodyUrlEncodedID != nil {
			bodyUrlencoded.ParentHttpBodyUrlEncodedID = nil
			bodyUrlencoded.IsDelta = false
		}

		// Create body urlencoded
		if err := bodyUrlencodedService.Create(ctx, &bodyUrlencoded); err != nil {
			return fmt.Errorf("failed to create HTTP body urlencoded: %w", err)
		}

		result.HTTPBodyUrlencodedCreated++
	}
	return nil
}

// importHTTPBodyRaw imports HTTP body raw from the bundle.
func (s *IOWorkspaceService) importHTTPBodyRaw(ctx context.Context, bodyRawService *shttp.HttpBodyRawService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	// First pass: Import base bodies
	for _, bodyRaw := range bundle.HTTPBodyRaw {
		if bodyRaw.IsDelta {
			continue
		}

		// Remap HTTP ID
		newHTTPID := bodyRaw.HttpID
		if mappedID, ok := result.HTTPIDMap[bodyRaw.HttpID]; ok {
			newHTTPID = mappedID
		}

		// Create base body
		_, err := bodyRawService.Create(ctx, newHTTPID, bodyRaw.RawData, bodyRaw.ContentType)
		if err != nil {
			return fmt.Errorf("failed to create HTTP body raw: %w", err)
		}

		result.HTTPBodyRawCreated++
	}

	// Second pass: Import delta bodies
	for _, bodyRaw := range bundle.HTTPBodyRaw {
		if !bodyRaw.IsDelta {
			continue
		}

		// Remap HTTP ID
		newHTTPID := bodyRaw.HttpID
		if mappedID, ok := result.HTTPIDMap[bodyRaw.HttpID]; ok {
			newHTTPID = mappedID
		}

		// Get delta content type
		var deltaContentType string
		if ct, ok := bodyRaw.DeltaContentType.(string); ok {
			deltaContentType = ct
		} else if ctPtr, ok := bodyRaw.DeltaContentType.(*string); ok && ctPtr != nil {
			deltaContentType = *ctPtr
		}

		// Create delta body
		_, err := bodyRawService.CreateDelta(ctx, newHTTPID, bodyRaw.DeltaRawData, deltaContentType)
		if err != nil {
			return fmt.Errorf("failed to create HTTP delta body raw: %w", err)
		}

		result.HTTPBodyRawCreated++
	}
	return nil
}

// importHTTPAsserts imports HTTP asserts from the bundle.
func (s *IOWorkspaceService) importHTTPAsserts(ctx context.Context, assertService *shttp.HttpAssertService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, assert := range bundle.HTTPAsserts {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			assert.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[assert.HttpID]; ok {
			assert.HttpID = newHTTPID
		}

		// Create assert
		if err := assertService.Create(ctx, &assert); err != nil {
			return fmt.Errorf("failed to create HTTP assert: %w", err)
		}

		result.HTTPAssertsCreated++
	}
	return nil
}
