package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/oklog/ulid/v2"
	_ "github.com/mattn/go-sqlite3"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/internal/ctx"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/muuid"
	"the-dev-tools/server/pkg/service/sexampleheader"
	pb "the-dev-tools/spec/dist/buf/connect/the_dev_tools/spec/v1"
)

func main() {
	// Connect to development database
	dbPath := "development.db"
	
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s", dbPath))
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create context
	ctx := context.Background()
	queries := gen.New(db)

	// Get the existing example ID from the database
	var exampleID idwrap.IDWrap
	rows, err := db.Query("SELECT id FROM item_api_example LIMIT 1")
	if err != nil {
		log.Fatalf("Failed to query examples: %v", err)
	}
	defer rows.Close()
	
	if rows.Next() {
		var idBytes []byte
		if err := rows.Scan(&idBytes); err != nil {
			log.Fatalf("Failed to scan example ID: %v", err)
		}
		exampleID = idwrap.IDWrap{ID: idBytes}
	} else {
		log.Fatalf("No examples found in database")
	}

	fmt.Printf("Using example ID: %x\n", exampleID.Bytes())

	// Clear existing headers for clean test
	fmt.Println("\nClearing existing headers...")
	_, err = db.Exec("DELETE FROM example_header WHERE example_id = ?", exampleID.Bytes())
	if err != nil {
		log.Printf("Failed to clear headers: %v", err)
	}

	// Now test header operations through RPC layer
	fmt.Println("\n=== Testing Header Ordering in Development DB ===\n")

	// Create service context
	headerService := sexampleheader.NewHeaderService(queries)
	serviceCtx := &ctx.ServiceContext{
		AccountID:     muuid.NewID(ulid.Make()),
		WorkspaceID:   muuid.NewID(ulid.Make()),
		UserID:        muuid.NewID(ulid.Make()),
		HeaderService: headerService,
	}
	
	reqHandler := &rrequest.RequestHandler{}

	// Step 1: Create headers
	fmt.Println("Step 1: Creating 4 headers...")
	headers := []string{"Header-1", "Header-2", "Header-3", "Header-4"}
	var createdHeaders []*pb.ExampleHeader
	
	for _, headerKey := range headers {
		header := &pb.ExampleHeader{
			ExampleId:  exampleID.String(),
			HeaderKey:  headerKey,
			Value:      fmt.Sprintf("Value for %s", headerKey),
			Enable:     true,
			Description: fmt.Sprintf("Description for %s", headerKey),
		}
		
		resp, err := reqHandler.HeaderCreate(serviceCtx, &pb.HeaderCreateRequest{
			Header: header,
		})
		if err != nil {
			log.Printf("Failed to create header %s: %v", headerKey, err)
			continue
		}
		createdHeaders = append(createdHeaders, resp.Header)
		fmt.Printf("  Created: %s (ID: %s)\n", headerKey, resp.Header.Id[:8]+"...")
	}

	// Step 2: List headers to verify initial order
	fmt.Println("\nStep 2: Listing headers - Initial order:")
	listResp, err := reqHandler.HeaderList(serviceCtx, &pb.HeaderListRequest{
		ExampleId: exampleID.String(),
	})
	if err != nil {
		log.Fatalf("Failed to list headers: %v", err)
	}

	fmt.Println("  Current order:")
	for i, h := range listResp.Headers {
		fmt.Printf("    Position %d: %s\n", i+1, h.HeaderKey)
	}

	// Step 3: Move Header-4 after Header-1 (should result in 1,4,2,3)
	if len(createdHeaders) >= 4 {
		fmt.Println("\nStep 3: Moving Header-4 after Header-1...")
		
		header4ID, _ := idwrap.ParseIDWrap(createdHeaders[3].Id)
		header1ID, _ := idwrap.ParseIDWrap(createdHeaders[0].Id)
		
		// Use HeaderService directly for move operation
		err := headerService.MoveHeader(ctx, header4ID, &header1ID, nil, exampleID)
		if err != nil {
			log.Printf("Failed to move header: %v", err)
		} else {
			fmt.Println("  ✓ Move successful")
		}
	}

	// Step 4: List headers again to verify new order
	fmt.Println("\nStep 4: Listing headers - After move:")
	listResp2, err := reqHandler.HeaderList(serviceCtx, &pb.HeaderListRequest{
		ExampleId: exampleID.String(),
	})
	if err != nil {
		log.Fatalf("Failed to list headers after move: %v", err)
	}

	fmt.Println("  New order:")
	actualOrder := []string{}
	for i, h := range listResp2.Headers {
		fmt.Printf("    Position %d: %s\n", i+1, h.HeaderKey)
		actualOrder = append(actualOrder, h.HeaderKey)
	}

	// Verify the order is correct
	expectedOrder := []string{"Header-1", "Header-4", "Header-2", "Header-3"}
	success := true
	for i := range expectedOrder {
		if i >= len(actualOrder) || actualOrder[i] != expectedOrder[i] {
			success = false
			break
		}
	}

	if success {
		fmt.Println("\n✅ SUCCESS: Headers are correctly ordered as 1,4,2,3!")
	} else {
		fmt.Println("\n❌ FAILURE: Headers are not in expected order")
		fmt.Printf("   Expected: %v\n", expectedOrder)
		fmt.Printf("   Got:      %v\n", actualOrder)
	}

	// Step 5: Show database state
	fmt.Println("\n=== Database State (Linked List Structure) ===")
	rows2, err := db.Query(`
		WITH RECURSIVE ordered_headers AS (
			SELECT id, header_key, prev, next, 1 as position
			FROM example_header 
			WHERE example_id = ? AND prev IS NULL
			
			UNION ALL
			
			SELECT eh.id, eh.header_key, eh.prev, eh.next, oh.position + 1
			FROM example_header eh
			INNER JOIN ordered_headers oh ON eh.prev = oh.id
			WHERE eh.example_id = ?
		)
		SELECT position, header_key, 
		       CASE WHEN prev IS NULL THEN 'NULL' ELSE hex(prev) END as prev_hex,
		       CASE WHEN next IS NULL THEN 'NULL' ELSE hex(next) END as next_hex
		FROM ordered_headers
		ORDER BY position
	`, exampleID.Bytes(), exampleID.Bytes())
	
	if err != nil {
		log.Printf("Failed to query database: %v", err)
	} else {
		defer rows2.Close()
		fmt.Println("  Position | Header    | Prev ID     | Next ID")
		fmt.Println("  ---------|-----------|-------------|-------------")
		for rows2.Next() {
			var pos int
			var key, prev, next string
			rows2.Scan(&pos, &key, &prev, &next)
			
			prevDisplay := "NULL"
			if prev != "NULL" && len(prev) > 8 {
				prevDisplay = prev[:8] + "..."
			}
			
			nextDisplay := "NULL"  
			if next != "NULL" && len(next) > 8 {
				nextDisplay = next[:8] + "..."
			}
			
			fmt.Printf("  %-9d| %-10s| %-12s| %s\n", pos, key, prevDisplay, nextDisplay)
		}
	}
}