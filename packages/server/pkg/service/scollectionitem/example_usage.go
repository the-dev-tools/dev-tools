package scollectionitem

// Example usage of CollectionItemService demonstrating the simplified reference-based architecture
//
// This file shows how to use the CollectionItemService for common operations:
// 1. Creating folders and endpoints 
// 2. Listing collection items in order
// 3. Moving items within a collection
//
// Architecture Overview:
// =====================
// 
// PRIMARY TABLE: collection_items
// - Contains ALL ordering logic (prev/next linked list)
// - Stores common data (id, name, collection_id, parent_folder_id, item_type)
// - References legacy tables via foreign keys (folder_id, endpoint_id)
//
// REFERENCE TABLES: item_folder, item_api  
// - Store type-specific data only
// - Have collection_item_id FK pointing to collection_items
// - NO prev/next fields (moved to collection_items)
//
// Benefits:
// - Single source of truth for ordering
// - Simplified move operations (only update one table)
// - Mixed folder/endpoint ordering works naturally
// - Easy to extend with new item types
//
// Usage Example:
// =============
//
// ```go
// // Initialize service
// service := scollectionitem.New(queries, logger)
//
// // Create a folder (automatically positioned at end)
// folder := &mitemfolder.ItemFolder{
//     ID:           idwrap.New(ulid.Make()),
//     CollectionID: collectionID,
//     ParentID:     nil, // Root level
//     Name:         "API Documentation",
// }
// err := service.CreateFolderTX(ctx, tx, folder)
//
// // Create an endpoint (automatically positioned at end)  
// endpoint := &mitemapi.ItemApi{
//     ID:           idwrap.New(ulid.Make()),
//     CollectionID: collectionID,
//     FolderID:     nil, // Root level
//     Name:         "Get Users",
//     Url:          "/api/users",
//     Method:       "GET",
// }
// err = service.CreateEndpointTX(ctx, tx, endpoint)
//
// // List all items in collection (mixed folders/endpoints in order)
// items, err := service.ListCollectionItems(ctx, collectionID, nil)
// // Returns: [folder, endpoint] in linked list order
//
// // Move endpoint before folder
// err = service.MoveCollectionItem(ctx, endpoint.ID, &folder.ID, movable.MovePositionBefore)
// // Now order is: [endpoint, folder]
// ```
//
// Database Flow:
// =============
//
// When creating a folder:
// 1. Create collection_items entry: {id: <new>, type: 0, name: "API Documentation", folder_id: <folder_id>}
// 2. Create item_folder entry: {id: <folder_id>, collection_item_id: <new>, name: "API Documentation"}
// 3. Both reference each other, but collection_items controls ordering
//
// When creating an endpoint:
// 1. Create collection_items entry: {id: <new>, type: 1, name: "Get Users", endpoint_id: <endpoint_id>}
// 2. Create item_api entry: {id: <endpoint_id>, collection_item_id: <new>, name: "Get Users", url: "/api/users"}
// 3. Both reference each other, but collection_items controls ordering
//
// When listing items:
// 1. Query collection_items with recursive CTE for correct ordering
// 2. Returns mixed list of folders and endpoints in display order
// 3. Can JOIN with legacy tables if additional type-specific data needed
//
// When moving items:
// 1. Update prev/next pointers in collection_items table only
// 2. No changes needed in legacy tables
// 3. Works seamlessly across different item types