package ioworkspace

import (
	"context"
	"fmt"
	devtoolsdb "the-dev-tools/db"
	"time"
)

// ImportIntoWorkspaceImproved imports data into an existing workspace with better error handling and logging.
// Unlike ImportWorkspace, this does not create a new workspace.
func (s *IOWorkspaceService) ImportIntoWorkspaceImproved(ctx context.Context, data WorkspaceData) error {
	startTime := time.Now()
	fmt.Printf("ImportIntoWorkspace: Starting import at %s\n", startTime.Format(time.RFC3339))
	
	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Skip workspace creation - we're importing into an existing workspace
	// Just verify the workspace exists
	fmt.Printf("ImportIntoWorkspace: Verifying workspace exists (ID: %s)\n", data.Workspace.ID)
	_, err = s.workspaceService.TX(tx).Get(ctx, data.Workspace.ID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	// services
	txCollectionService := s.collectionService.TX(tx)
	txFolderService := s.folderservice.TX(tx)
	txEndpointService := s.endpointService.TX(tx)
	txExampleService := s.exampleService.TX(tx)
	txExampleHeaderService := s.exampleHeaderService.TX(tx)
	txExampleQueryService := s.exampleQueryService.TX(tx)
	txExampleAssertService := s.exampleAssertService.TX(tx)
	txRawBodyService := s.rawBodyService.TX(tx)
	txFormBodyService := s.formBodyService.TX(tx)
	txUrlBodyService := s.urlBodyService.TX(tx)
	txResponseService := s.responseService.TX(tx)
	txResponseHeaderService := s.responseHeaderService.TX(tx)
	txResponseAssertService := s.responseAssertService.TX(tx)

	// flow
	txFlowService := s.flowService.TX(tx)
	txFlowNodeService := s.flowNodeService.TX(tx)
	txFlowEdgeService := s.flowEdgeService.TX(tx)
	txFlowVariableService := s.flowVariableService.TX(tx)

	tdFlowRequestService := s.flowRequestService.TX(tx)
	txFlowConditionService := s.flowConditionService.TX(tx)
	txFlowNoopService := s.flowNoopService.TX(tx)
	txFlowForService := s.flowForService.TX(tx)
	txFlowForEachService := s.flowForEachService.TX(tx)
	txFlowJSService := s.flowJSService.TX(tx)

	// Collections
	fmt.Printf("ImportIntoWorkspace: Creating %d collections\n", len(data.Collections))
	for i, collection := range data.Collections {
		err = txCollectionService.CreateCollection(ctx, &collection)
		if err != nil {
			return fmt.Errorf("failed to create collection %d/%d (%s): %w", i+1, len(data.Collections), collection.Name, err)
		}
	}

	// Folders
	fmt.Printf("ImportIntoWorkspace: Creating %d folders\n", len(data.Folders))
	if len(data.Folders) > 0 {
		err = txFolderService.CreateItemFolderBulk(ctx, data.Folders)
		if err != nil {
			return fmt.Errorf("failed to create folders: %w", err)
		}
	}

	// Endpoints
	fmt.Printf("ImportIntoWorkspace: Creating %d endpoints\n", len(data.Endpoints))
	if len(data.Endpoints) > 0 {
		err = txEndpointService.CreateItemApiBulk(ctx, data.Endpoints)
		if err != nil {
			return fmt.Errorf("failed to create endpoints: %w", err)
		}
	}

	// Examples
	fmt.Printf("ImportIntoWorkspace: Creating %d examples\n", len(data.Examples))
	if len(data.Examples) > 0 {
		err = txExampleService.CreateApiExampleBulk(ctx, data.Examples)
		if err != nil {
			return fmt.Errorf("failed to create examples: %w", err)
		}
	}

	// Example Headers
	fmt.Printf("ImportIntoWorkspace: Creating %d example headers\n", len(data.ExampleHeaders))
	if len(data.ExampleHeaders) > 0 {
		err = txExampleHeaderService.CreateBulkHeader(ctx, data.ExampleHeaders)
		if err != nil {
			return fmt.Errorf("failed to create example headers: %w", err)
		}
	}

	// Example Queries
	fmt.Printf("ImportIntoWorkspace: Creating %d example queries\n", len(data.ExampleQueries))
	if len(data.ExampleQueries) > 0 {
		err = txExampleQueryService.CreateBulkQuery(ctx, data.ExampleQueries)
		if err != nil {
			return fmt.Errorf("failed to create example queries: %w", err)
		}
	}

	// Example Asserts
	fmt.Printf("ImportIntoWorkspace: Creating %d example asserts\n", len(data.ExampleAsserts))
	if len(data.ExampleAsserts) > 0 {
		err = txExampleAssertService.CreateAssertBulk(ctx, data.ExampleAsserts)
		if err != nil {
			return fmt.Errorf("failed to create example asserts: %w", err)
		}
	}

	// Raw Bodies
	fmt.Printf("ImportIntoWorkspace: Creating %d raw bodies\n", len(data.Rawbodies))
	if len(data.Rawbodies) > 0 {
		err = txRawBodyService.CreateBulkBodyRaw(ctx, data.Rawbodies)
		if err != nil {
			return fmt.Errorf("failed to create raw bodies: %w", err)
		}
	}

	// Form Bodies
	fmt.Printf("ImportIntoWorkspace: Creating %d form bodies\n", len(data.FormBodies))
	if len(data.FormBodies) > 0 {
		err = txFormBodyService.CreateBulkBodyForm(ctx, data.FormBodies)
		if err != nil {
			return fmt.Errorf("failed to create form bodies: %w", err)
		}
	}

	// URL Bodies
	fmt.Printf("ImportIntoWorkspace: Creating %d URL bodies\n", len(data.UrlBodies))
	if len(data.UrlBodies) > 0 {
		err = txUrlBodyService.CreateBulkBodyURLEncoded(ctx, data.UrlBodies)
		if err != nil {
			return fmt.Errorf("failed to create URL bodies: %w", err)
		}
	}

	// Example Responses
	fmt.Printf("ImportIntoWorkspace: Creating %d example responses\n", len(data.ExampleResponses))
	if len(data.ExampleResponses) > 0 {
		err = txResponseService.CreateExampleRespBulk(ctx, data.ExampleResponses)
		if err != nil {
			return fmt.Errorf("failed to create example responses: %w", err)
		}
	}

	// Example Response Headers
	fmt.Printf("ImportIntoWorkspace: Creating %d example response headers\n", len(data.ExampleResponseHeaders))
	if len(data.ExampleResponseHeaders) > 0 {
		err = txResponseHeaderService.CreateExampleRespHeaderBulk(ctx, data.ExampleResponseHeaders)
		if err != nil {
			return fmt.Errorf("failed to create example response headers: %w", err)
		}
	}

	// Response Assert Results
	fmt.Printf("ImportIntoWorkspace: Creating %d response assert results\n", len(data.ExampleResponseAsserts))
	if len(data.ExampleResponseAsserts) > 0 {
		err = txResponseAssertService.CreateAssertResultBulk(ctx, data.ExampleResponseAsserts)
		if err != nil {
			return fmt.Errorf("failed to create response assert results: %w", err)
		}
	}

	// Flows
	fmt.Printf("ImportIntoWorkspace: Creating %d flows\n", len(data.Flows))
	for i, flow := range data.Flows {
		f := flow // Create a copy to avoid taking address of loop variable
		err = txFlowService.CreateFlow(ctx, f)
		if err != nil {
			return fmt.Errorf("failed to create flow %d/%d (%s): %w", i+1, len(data.Flows), flow.Name, err)
		}
	}

	// Flow Variables
	fmt.Printf("ImportIntoWorkspace: Creating %d flow variables\n", len(data.FlowVariables))
	for i, flowVariable := range data.FlowVariables {
		fv := flowVariable // Create a copy to avoid taking address of loop variable
		err = txFlowVariableService.CreateFlowVariable(ctx, fv)
		if err != nil {
			return fmt.Errorf("failed to create flow variable %d/%d: %w", i+1, len(data.FlowVariables), err)
		}
	}

	// Flow Nodes
	fmt.Printf("ImportIntoWorkspace: Creating %d flow nodes\n", len(data.FlowNodes))
	if len(data.FlowNodes) > 0 {
		err = txFlowNodeService.CreateNodeBulk(ctx, data.FlowNodes)
		if err != nil {
			return fmt.Errorf("failed to create flow nodes: %w", err)
		}
	}

	// Flow Request Nodes
	fmt.Printf("ImportIntoWorkspace: Creating %d flow request nodes\n", len(data.FlowRequestNodes))
	if len(data.FlowRequestNodes) > 0 {
		err = tdFlowRequestService.CreateNodeRequestBulk(ctx, data.FlowRequestNodes)
		if err != nil {
			return fmt.Errorf("failed to create flow request nodes: %w", err)
		}
	}

	// Flow Condition Nodes
	fmt.Printf("ImportIntoWorkspace: Creating %d flow condition nodes\n", len(data.FlowConditionNodes))
	if len(data.FlowConditionNodes) > 0 {
		err = txFlowConditionService.CreateNodeIfBulk(ctx, data.FlowConditionNodes)
		if err != nil {
			return fmt.Errorf("failed to create flow condition nodes: %w", err)
		}
	}

	// Flow Noop Nodes
	fmt.Printf("ImportIntoWorkspace: Creating %d flow noop nodes\n", len(data.FlowNoopNodes))
	if len(data.FlowNoopNodes) > 0 {
		err = txFlowNoopService.CreateNodeNoopBulk(ctx, data.FlowNoopNodes)
		if err != nil {
			return fmt.Errorf("failed to create flow noop nodes: %w", err)
		}
	}

	// Flow For Nodes
	fmt.Printf("ImportIntoWorkspace: Creating %d flow for nodes\n", len(data.FlowForNodes))
	if len(data.FlowForNodes) > 0 {
		err = txFlowForService.CreateNodeForBulk(ctx, data.FlowForNodes)
		if err != nil {
			return fmt.Errorf("failed to create flow for nodes: %w", err)
		}
	}

	// Flow JS Nodes
	fmt.Printf("ImportIntoWorkspace: Creating %d flow JS nodes\n", len(data.FlowJSNodes))
	if len(data.FlowJSNodes) > 0 {
		err = txFlowJSService.CreateNodeJSBulk(ctx, data.FlowJSNodes)
		if err != nil {
			return fmt.Errorf("failed to create flow JS nodes: %w", err)
		}
	}

	// Flow Edges
	fmt.Printf("ImportIntoWorkspace: Creating %d flow edges\n", len(data.FlowEdges))
	if len(data.FlowEdges) > 0 {
		err = txFlowEdgeService.CreateEdgeBulk(ctx, data.FlowEdges)
		if err != nil {
			return fmt.Errorf("failed to create flow edges: %w", err)
		}
	}

	// Flow ForEach Nodes
	fmt.Printf("ImportIntoWorkspace: Creating %d flow foreach nodes\n", len(data.FlowForEachNodes))
	if len(data.FlowForEachNodes) > 0 {
		err = txFlowForEachService.CreateNodeForEachBulk(ctx, data.FlowForEachNodes)
		if err != nil {
			return fmt.Errorf("failed to create flow foreach nodes: %w", err)
		}
	}

	fmt.Printf("ImportIntoWorkspace: Committing transaction\n")
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("ImportIntoWorkspace: Import completed successfully in %s\n", duration)
	return nil
}