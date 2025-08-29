package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Connect to development database
	dbPath := "development.db"
	
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s", dbPath))
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	fmt.Println("=== Testing Header Ordering System ===\n")

	// Step 1: Check current database state
	fmt.Println("Step 1: Current database state")
	showHeaderState(db)

	// Step 2: Clear existing headers and create fresh test data
	fmt.Println("\nStep 2: Creating fresh test headers...")
	
	// Get existing example ID
	exampleID := getExampleID(db)
	if exampleID == nil {
		log.Fatal("No example found in database")
	}
	
	fmt.Printf("Using example ID: %s\n", hex.EncodeToString(exampleID)[:16]+"...")
	
	// Clear existing headers
	_, err = db.Exec("DELETE FROM example_header WHERE example_id = ?", exampleID)
	if err != nil {
		log.Printf("Failed to clear headers: %v", err)
	}
	
	// Create 4 test headers in sequence
	headers := []string{"Header-1", "Header-2", "Header-3", "Header-4"}
	headerIDs := make([][]byte, len(headers))
	
	for i, headerKey := range headers {
		headerID := generateULID()
		headerIDs[i] = headerID
		
		// Insert header with proper linked list structure
		var prevID interface{}
		if i > 0 {
			prevID = headerIDs[i-1]
		}
		
		_, err = db.Exec(`
			INSERT INTO example_header (id, example_id, header_key, value, enable, description, prev, next)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
		`, headerID, exampleID, headerKey, fmt.Sprintf("Value-%s", headerKey), true, fmt.Sprintf("Desc-%s", headerKey), prevID)
		
		if err != nil {
			log.Printf("Failed to create header %s: %v", headerKey, err)
			continue
		}
		
		// Update previous header's next pointer
		if i > 0 {
			_, err = db.Exec("UPDATE example_header SET next = ? WHERE id = ?", headerID, headerIDs[i-1])
			if err != nil {
				log.Printf("Failed to update previous header's next pointer: %v", err)
			}
		}
		
		fmt.Printf("  Created %s (ID: %s)\n", headerKey, hex.EncodeToString(headerID)[:16]+"...")
	}
	
	fmt.Println("\nStep 3: Initial order verification")
	showHeaderState(db)
	
	// Step 4: Test header move - Move Header-4 after Header-1
	fmt.Println("\nStep 4: Moving Header-4 after Header-1...")
	
	header4ID := headerIDs[3] // Header-4
	header1ID := headerIDs[0] // Header-1
	
	err = moveHeaderAfter(db, header4ID, header1ID, exampleID)
	if err != nil {
		fmt.Printf("❌ Move failed: %v\n", err)
	} else {
		fmt.Println("✓ Move operation completed")
	}
	
	fmt.Println("\nStep 5: Final order verification")
	showHeaderState(db)
	
	// Step 6: Verify expected order
	expectedOrder := []string{"Header-1", "Header-4", "Header-2", "Header-3"}
	actualOrder := getHeaderOrder(db, exampleID)
	
	fmt.Println("\nStep 6: Order verification")
	fmt.Printf("Expected: %v\n", expectedOrder)
	fmt.Printf("Actual:   %v\n", actualOrder)
	
	success := len(actualOrder) == len(expectedOrder)
	if success {
		for i := range expectedOrder {
			if actualOrder[i] != expectedOrder[i] {
				success = false
				break
			}
		}
	}
	
	if success {
		fmt.Println("\n✅ SUCCESS: Header ordering is working correctly!")
	} else {
		fmt.Println("\n❌ FAILURE: Header ordering is not working as expected!")
		os.Exit(1)
	}
}

func showHeaderState(db *sql.DB) {
	rows, err := db.Query(`
		WITH RECURSIVE ordered_headers AS (
			SELECT id, header_key, example_id, prev, next, 1 as position
			FROM example_header 
			WHERE prev IS NULL
			
			UNION ALL
			
			SELECT eh.id, eh.header_key, eh.example_id, eh.prev, eh.next, oh.position + 1
			FROM example_header eh
			INNER JOIN ordered_headers oh ON eh.prev = oh.id
		)
		SELECT position, header_key, 
		       CASE WHEN prev IS NULL THEN 'NULL' ELSE hex(prev) END as prev_hex,
		       CASE WHEN next IS NULL THEN 'NULL' ELSE hex(next) END as next_hex
		FROM ordered_headers
		ORDER BY position
	`)
	
	if err != nil {
		log.Printf("Failed to query headers: %v", err)
		return
	}
	defer rows.Close()
	
	fmt.Println("  Position | Header    | Prev ID      | Next ID")
	fmt.Println("  ---------|-----------|--------------|----------")
	
	hasRows := false
	for rows.Next() {
		hasRows = true
		var pos int
		var key, prev, next string
		if err := rows.Scan(&pos, &key, &prev, &next); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		
		prevDisplay := "NULL"
		if prev != "NULL" && len(prev) > 8 {
			prevDisplay = prev[:8] + "..."
		}
		
		nextDisplay := "NULL"  
		if next != "NULL" && len(next) > 8 {
			nextDisplay = next[:8] + "..."
		}
		
		fmt.Printf("  %-9d| %-10s| %-13s| %s\n", pos, key, prevDisplay, nextDisplay)
	}
	
	if !hasRows {
		fmt.Println("  No headers found")
	}
}

func getExampleID(db *sql.DB) []byte {
	var exampleID []byte
	err := db.QueryRow("SELECT id FROM item_api_example LIMIT 1").Scan(&exampleID)
	if err != nil {
		log.Printf("Failed to get example ID: %v", err)
		return nil
	}
	return exampleID
}

func getHeaderOrder(db *sql.DB, exampleID []byte) []string {
	rows, err := db.Query(`
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
		SELECT header_key FROM ordered_headers ORDER BY position
	`, exampleID, exampleID)
	
	if err != nil {
		log.Printf("Failed to get header order: %v", err)
		return nil
	}
	defer rows.Close()
	
	var order []string
	for rows.Next() {
		var headerKey string
		if err := rows.Scan(&headerKey); err != nil {
			log.Printf("Failed to scan header key: %v", err)
			continue
		}
		order = append(order, headerKey)
	}
	
	return order
}

// moveHeaderAfter moves headerID to be positioned after targetID
func moveHeaderAfter(db *sql.DB, headerID, targetID, exampleID []byte) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	// 1. Remove header from current position
	var headerPrev, headerNext []byte
	err = tx.QueryRow("SELECT prev, next FROM example_header WHERE id = ?", headerID).Scan(&headerPrev, &headerNext)
	if err != nil {
		return fmt.Errorf("failed to get header position: %w", err)
	}
	
	// Update links to bypass the moving header
	if headerPrev != nil {
		_, err = tx.Exec("UPDATE example_header SET next = ? WHERE id = ?", headerNext, headerPrev)
		if err != nil {
			return fmt.Errorf("failed to update previous header: %w", err)
		}
	}
	
	if headerNext != nil {
		_, err = tx.Exec("UPDATE example_header SET prev = ? WHERE id = ?", headerPrev, headerNext)
		if err != nil {
			return fmt.Errorf("failed to update next header: %w", err)
		}
	}
	
	// 2. Get target's current next
	var targetNext []byte
	err = tx.QueryRow("SELECT next FROM example_header WHERE id = ?", targetID).Scan(&targetNext)
	if err != nil {
		return fmt.Errorf("failed to get target next: %w", err)
	}
	
	// 3. Insert header after target
	_, err = tx.Exec("UPDATE example_header SET prev = ?, next = ? WHERE id = ?", targetID, targetNext, headerID)
	if err != nil {
		return fmt.Errorf("failed to update moving header: %w", err)
	}
	
	// 4. Update target to point to header
	_, err = tx.Exec("UPDATE example_header SET next = ? WHERE id = ?", headerID, targetID)
	if err != nil {
		return fmt.Errorf("failed to update target header: %w", err)
	}
	
	// 5. Update target's old next to point back to header
	if targetNext != nil {
		_, err = tx.Exec("UPDATE example_header SET prev = ? WHERE id = ?", headerID, targetNext)
		if err != nil {
			return fmt.Errorf("failed to update target's old next: %w", err)
		}
	}
	
	return tx.Commit()
}

// generateULID creates a unique ID for testing using timestamp + random
func generateULID() []byte {
	id := make([]byte, 16)
	// Use timestamp for uniqueness
	now := time.Now().UnixNano()
	for i := 0; i < 8; i++ {
		id[i] = byte(now >> (i * 8))
	}
	// Add random bytes
	rand.Read(id[8:])
	return id
}