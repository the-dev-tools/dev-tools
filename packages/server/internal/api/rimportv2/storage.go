package rimportv2

import (
	"context"
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

// DefaultImporter implements the Importer interface using existing modern services
// It coordinates HAR processing and storage operations
type DefaultImporter struct {
	httpService   *shttp.HTTPService
	flowService   *sflow.FlowService
	fileService   *sfile.FileService
	harTranslator *DefaultHARTranslator
}

// NewImporter creates a new DefaultImporter with service dependencies
func NewImporter(httpService *shttp.HTTPService, flowService *sflow.FlowService, fileService *sfile.FileService) *DefaultImporter {
	return &DefaultImporter{
		httpService:   httpService,
		flowService:   flowService,
		fileService:   fileService,
		harTranslator: NewHARTranslator(),
	}
}

// ImportAndStore processes HAR data and returns resolved models
func (imp *DefaultImporter) ImportAndStore(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	return imp.harTranslator.ConvertHAR(ctx, data, workspaceID)
}

// StoreHTTPEntities stores HTTP request entities using the modern HTTP service
func (imp *DefaultImporter) StoreHTTPEntities(ctx context.Context, httpReqs []*mhttp.HTTP) error {
	if len(httpReqs) == 0 {
		return nil
	}

	for _, httpReq := range httpReqs {
		if err := imp.httpService.Create(ctx, httpReq); err != nil {
			return fmt.Errorf("failed to store HTTP request: %w", err)
		}
	}

	return nil
}

// StoreFiles stores file entities using the modern file service
func (imp *DefaultImporter) StoreFiles(ctx context.Context, files []*mfile.File) error {
	if len(files) == 0 {
		return nil
	}

	for _, file := range files {
		if err := imp.fileService.CreateFile(ctx, file); err != nil {
			return fmt.Errorf("failed to store file: %w", err)
		}
	}

	return nil
}

// StoreFlow stores the flow entity using the modern flow service
func (imp *DefaultImporter) StoreFlow(ctx context.Context, flow *mflow.Flow) error {
	if flow == nil {
		return nil
	}

	return imp.flowService.CreateFlow(ctx, *flow)
}

// StoreImportResults performs a coordinated storage of all import results
func (imp *DefaultImporter) StoreImportResults(ctx context.Context, results *ImportResults) error {
	if results == nil {
		return nil
	}

	// Store files first (they may be referenced by HTTP entities)
	if len(results.Files) > 0 {
		for _, file := range results.Files {
			if err := imp.fileService.CreateFile(ctx, file); err != nil {
				return fmt.Errorf("failed to store files: %w", err)
			}
		}
	}

	// Store HTTP entities
	if len(results.HTTPReqs) > 0 {
		for _, httpReq := range results.HTTPReqs {
			if err := imp.httpService.Create(ctx, httpReq); err != nil {
				return fmt.Errorf("failed to store HTTP entities: %w", err)
			}
		}
	}

	// Store flow
	if results.Flow != nil {
		if err := imp.flowService.CreateFlow(ctx, *results.Flow); err != nil {
			return fmt.Errorf("failed to store flow: %w", err)
		}
	}

	return nil
}
