# Collection Items Move Implementation Plan (Improved with Testing)

## Date: 2025-01-21
## Based on: Lessons learned from previous implementation attempts

## Overview

This plan implements move functionality for collection items RPC with **compilation and testing after every phase** to catch issues early. Focus on **discovering and building upon existing functionality** rather than creating new systems from scratch.

## Key Insights from Previous Work

1. **Current State**: We have basic collection item listing but no move functionality
2. **Database**: `collection_items` table does NOT exist (confirmed in reset state)
3. **Architecture**: Currently uses separate `item_folder` and `item_api` tables with separate linked lists
4. **Problem**: Folders and endpoints can't be mixed in ordering (separate linked lists)
5. **Solution Approach**: **SIMPLIFIED** - `collection_items` table becomes the primary ordering mechanism with legacy tables referencing it (not dual-write)

## Requirements (Re-confirmed)

- ✅ Use golang-pro and sql-pro agents exclusively (NO direct implementation)
- ✅ Focus on backend only (no frontend changes)
- ✅ Comprehensive testing at RPC, service, and repository layers
- ✅ No migration needed (product not released yet)
- ✅ Enable mixed folder/endpoint ordering for drag-and-drop
- ✅ **NEW**: Compile and test after every phase

## Implementation Strategy

### Phase 1: Database Foundation (Day 1) - COMPILE & TEST

#### 1.1 Create Unified Collection Items Table
**Agent**: sql-pro
**Task**: Design and create `collection_items` table as **PRIMARY** ordering table
**Requirements**:
- Support both folders (type=0) and endpoints (type=1)
- Include linked list pointers (prev/next) - **ONLY place for linked list**
- Store common data (id, name, collection_id, parent_folder_id, type)
- Store type-specific data (url, method for endpoints; NULL for folders)
- Add proper indexes for performance
- **Remove prev/next from item_folder and item_api tables**

**Expected Deliverable**: SQL schema with collection_items as primary table

**Immediate Validation**: 
- ✅ Schema compiles without errors
- ✅ Can create/drop table successfully
- ✅ Indexes exist and are properly named

#### 1.2 Create SQLC Queries
**Agent**: sql-pro  
**Task**: Create SQLC queries for unified operations
**Required Queries**:
- `GetCollectionItemsInOrder` (recursive CTE for linked list traversal)
- `InsertCollectionItem`
- `UpdateCollectionItemOrder` (for move operations)
- `DeleteCollectionItem`

**Expected Deliverable**: SQLC query definitions

**Immediate Validation**: 
- ✅ SQLC queries compile without syntax errors
- ✅ All query parameters properly named and typed
- ✅ Recursive CTE query returns correct result structure

#### 1.3 Generate Go Code & Compile Test
**Agent**: golang-pro
**Task**: Generate Go code from SQLC definitions and verify compilation
**Commands**:
```bash
# Run in development environment
nix develop -c bash -c "
cd packages/db && sqlc generate
cd ../server && go build ./...
echo 'Compilation successful!'
"
```

**Expected Deliverable**: Generated Go structs and methods + successful compilation

**Phase 1 Exit Criteria**: 🛑 **MUST PASS BEFORE PHASE 2**
- ✅ Database schema compiles and can be applied
- ✅ SQLC generation succeeds without errors
- ✅ Server code compiles successfully with new generated code
- ✅ Basic database operations work (manual test with sqlite3)
- ✅ Run: `nix develop -c pnpm nx run server:test` (existing tests still pass)

---

### Phase 2: Repository Layer (Day 1) - COMPILE & TEST

#### 2.1 Create Movable Repository
**Agent**: golang-pro
**Task**: Implement `CollectionItemsMovableRepository`
**Requirements**:
- Implement `movable.MovableRepository` interface
- Handle linked list operations (insert, move, delete)
- Support transaction contexts
- Provide batch update capabilities

**Files to Create/Modify**:
- `packages/server/pkg/service/scollectionitem/repository.go`

**Immediate Validation**:
```bash
nix develop -c bash -c "
cd packages/server && go build ./pkg/service/scollectionitem/...
echo 'Repository compilation successful!'
"
```

#### 2.2 Create Repository Tests
**Agent**: golang-pro
**Task**: Create comprehensive repository tests
**Test Coverage**:
- Basic CRUD operations
- Linked list integrity
- Move operations (before/after/first/last)
- Transaction rollback scenarios

**Files to Create**:
- `packages/server/pkg/service/scollectionitem/repository_test.go`

**Immediate Validation**:
```bash
nix develop -c bash -c "
cd packages/server && go test ./pkg/service/scollectionitem/... -v
echo 'Repository tests successful!'
"
```

**Phase 2 Exit Criteria**: 🛑 **MUST PASS BEFORE PHASE 3**
- ✅ Repository compiles without errors
- ✅ Repository tests pass with >90% coverage
- ✅ Interface properly implemented (no missing methods)
- ✅ All existing server tests still pass: `nix develop -c pnpm nx run server:test`

---

### Phase 3: Service Layer (Day 2) - COMPILE & TEST

#### 3.1 Create Collection Item Service  
**Agent**: golang-pro
**Task**: Implement `CollectionItemService` with **SIMPLIFIED** reference pattern
**Key Methods**:
- `ListCollectionItems()` - read from collection_items table only
- `MoveCollectionItem()` - update prev/next in collection_items only
- `CreateFolderTX()` - create collection_item first, then folder with collection_item_id FK
- `CreateEndpointTX()` - create collection_item first, then endpoint with collection_item_id FK

**Architecture**: **SIMPLIFIED** - Collection items table is primary, legacy tables reference it
1. Create collection_item entry (with ordering data)
2. Create legacy table entry (folder/endpoint) with collection_item_id FK
3. **No dual-write** - collection_items contains all ordering logic
4. Legacy tables only store type-specific data + reference to collection_item

**Files to Create/Modify**:
- `packages/server/pkg/service/scollectionitem/service.go`

**Immediate Validation**:
```bash
nix develop -c bash -c "
cd packages/server && go build ./pkg/service/scollectionitem/...
echo 'Service compilation successful!'
"
```

#### 3.2 Create Service Tests
**Agent**: golang-pro
**Task**: Create comprehensive service tests
**Test Coverage**:
- Collection item creation with proper FK references
- Move operations across types (folder → endpoint → folder)
- Transaction integrity
- Error handling
- Data consistency between collection_items and legacy tables

**Files to Create**:
- `packages/server/pkg/service/scollectionitem/service_test.go`

**Immediate Validation**:
```bash
nix develop -c bash -c "
cd packages/server && go test ./pkg/service/scollectionitem/... -v
echo 'Service tests successful!'
"
```

**Phase 3 Exit Criteria**: 🛑 **MUST PASS BEFORE PHASE 4**
- ✅ Service compiles without errors
- ✅ Service tests pass with >90% coverage
- ✅ FK reference consistency verified (legacy tables → collection_items)
- ✅ All existing server tests still pass: `nix develop -c pnpm nx run server:test`

---

### Phase 4: RPC Integration (Day 2) - COMPILE & TEST

#### 4.1 Update Collection Item RPC
**Agent**: golang-pro
**Task**: Implement move functionality in RPC layer
**Current State**: `CollectionItemMove` returns empty response (TODO comment)
**Required Changes**:
- Parse and validate move request
- Call service layer for move operations
- Handle permission checking
- Return proper response

**Files to Modify**:
- `packages/server/internal/api/rcollectionitem/collectionitem.go`

**Immediate Validation**:
```bash
nix develop -c bash -c "
cd packages/server && go build ./internal/api/rcollectionitem/...
echo 'RPC compilation successful!'
"
```

#### 4.2 Update Creation Handlers (**REQUIRED**)
**Agent**: golang-pro
**Task**: Update folder/endpoint creation to use collection item service
**Changes Required**:
- Folder creation: Call `CollectionItemService.CreateFolderTX()` instead of direct folder service
- Endpoint creation: Call `CollectionItemService.CreateEndpointTX()` instead of direct endpoint service
- This ensures collection_item is created first, then legacy table with FK reference
**Files to Modify**:
- `packages/server/internal/api/ritemfolder/ritemfolder.go`
- `packages/server/internal/api/ritemapi/ritemapi.go`

**Note**: This is **REQUIRED** since collection_items must be created before legacy items

#### 4.3 Create RPC Tests
**Agent**: golang-pro
**Task**: Create comprehensive RPC tests
**Test Coverage**:
- Move request validation
- Permission checking
- Error scenarios
- Integration with service layer

**Files to Create**:
- `packages/server/internal/api/rcollectionitem/collectionitem_move_test.go`

**Immediate Validation**:
```bash
nix develop -c bash -c "
cd packages/server && go test ./internal/api/rcollectionitem/... -v
echo 'RPC tests successful!'
"
```

#### 4.4 Integration Smoke Test
**Command**:
```bash
nix develop -c bash -c "
cd packages/server && go run cmd/server/server.go &
SERVER_PID=$!
sleep 2
# Test that server starts without crashing
kill $SERVER_PID
echo 'Server integration smoke test passed!'
"
```

**Phase 4 Exit Criteria**: 🛑 **MUST PASS BEFORE PHASE 5**
- ✅ RPC layer compiles without errors
- ✅ RPC tests pass with >80% coverage
- ✅ Server starts and responds to requests
- ✅ All existing server tests still pass: `nix develop -c pnpm nx run server:test`

---

### Phase 5: End-to-End Testing (Day 3) - COMPREHENSIVE TEST

#### 5.1 Create Integration Tests
**Agent**: golang-pro
**Task**: Create end-to-end integration tests
**Test Scenarios**:
- Create folder → Create endpoint → List (mixed order) → Move
- Cross-folder moves
- Empty collection scenarios
- Large collection performance (100+ items)

**Files to Create**:
- `packages/server/internal/api/rcollectionitem/integration_test.go`

**Validation**:
```bash
nix develop -c bash -c "
cd packages/server && go test ./internal/api/rcollectionitem/... -run Integration -v
echo 'Integration tests successful!'
"
```

#### 5.2 Performance & Load Testing
**Agent**: golang-pro
**Task**: Test with realistic loads
**Test Focus**:
- 100+ items in collection
- Concurrent move operations
- Memory usage patterns
- Query performance

**Validation**:
```bash
nix develop -c bash -c "
cd packages/server && go test ./internal/api/rcollectionitem/... -run Performance -v
echo 'Performance tests successful!'
"
```

#### 5.3 Data Consistency Validation
**Agent**: sql-pro
**Task**: Create validation queries and tests
**Checks**:
- Items exist in both tables after creation
- Linked list integrity in unified table
- No orphaned records

**Phase 5 Exit Criteria**: 🛑 **MUST PASS FOR COMPLETION**
- ✅ All integration tests pass
- ✅ Performance acceptable (< 100ms for list/move with 100 items)
- ✅ Data consistency maintained under load
- ✅ Full test suite passes: `nix develop -c pnpm nx run server:test`

---

### Phase 6: Documentation & Final Validation (Day 0.5)

#### 6.1 Update CLAUDE.md
**Task**: Document unified collection items system
**Content**:
- Architecture overview (dual-table approach)
- Move functionality usage
- Testing guidelines
- Performance characteristics

#### 6.2 Final System Test
**Commands**:
```bash
# Complete build and test
nix develop -c bash -c "
pnpm nx run-many --targets=lint,typecheck,build,test
echo 'Full system validation passed!'
"
```

**Final Acceptance Criteria**:
- ✅ All code compiles without warnings
- ✅ All tests pass (unit, integration, performance)
- ✅ No breaking changes to existing functionality
- ✅ Documentation updated and accurate

## Testing Strategy - After Every Phase

### Compilation Validation
Every phase must compile successfully before proceeding:
```bash
nix develop -c bash -c "cd packages/server && go build ./..."
```

### Test Validation
Every phase must pass its tests before proceeding:
```bash
nix develop -c bash -c "cd packages/server && go test ./... -v"
```

### Integration Validation
Existing functionality must continue working:
```bash
nix develop -c pnpm nx run server:test
```

### Rollback Strategy
If any phase fails compilation or testing:
1. **Stop immediately** - don't continue to next phase
2. **Debug the issue** with the responsible agent
3. **Fix or rollback** the changes from that phase
4. **Re-test** before proceeding

## Sub-Agent Task Format

Each task specifies:
- **Objective**: Clear goal and expected outcome
- **Files**: Specific paths to create/modify
- **Requirements**: Technical specifications
- **Expected Deliverable**: Concrete artifact to be produced
- **Immediate Validation**: Compile/test commands to run
- **Exit Criteria**: Must pass before next phase

## Success Criteria

- ✅ Mixed folder/endpoint ordering works correctly
- ✅ Drag-and-drop move operations function properly
- ✅ Data consistency maintained between tables
- ✅ All tests pass (unit, integration, performance)
- ✅ Performance acceptable for 100+ items
- ✅ No breaking changes to existing functionality
- ✅ **System compiles and tests pass after every phase**

## Risk Mitigation

### Early Detection
- **Compile after every change** - catch syntax errors immediately
- **Test after every implementation** - catch logic errors early
- **Integration test** - ensure no breaking changes

### Rollback Strategy
- **Git commits after each successful phase** - easy rollback points
- **Incremental changes** - smaller, safer modifications
- **Validation gates** - cannot proceed with broken code

## Estimated Timeline

- **Phase 1**: Database Foundation - 1 day (includes compilation/testing)
- **Phase 2**: Repository Layer - 0.5 day (includes compilation/testing)
- **Phase 3**: Service Layer - 1 day (includes compilation/testing)
- **Phase 4**: RPC Integration - 0.5 day (includes compilation/testing)
- **Phase 5**: End-to-End Testing - 1 day (comprehensive testing)
- **Phase 6**: Documentation - 0.5 day (final validation)

**Total**: 4.5 days with built-in validation

## Next Steps

1. **Start with Phase 1.1**: sql-pro designs unified collection items table
2. **Validate immediately**: Test schema compilation and SQLC generation
3. **Exit criteria**: Must pass all Phase 1 validations before Phase 2
4. **Iterate safely**: Fix issues in current phase before advancing

**🔴 CRITICAL**: No phase can be skipped. Each phase's exit criteria must be met before proceeding to ensure a stable, working implementation throughout the process.

## CollectionItemService Architecture Explanation

### Simplified Architecture (Updated)

Instead of the previous dual-write approach, we're using a **reference-based architecture**:

#### Database Structure:
```sql
-- PRIMARY ordering table (contains linked list)
CREATE TABLE collection_items (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  parent_folder_id BLOB,
  name TEXT NOT NULL,
  item_type INTEGER NOT NULL, -- 0=folder, 1=endpoint
  url TEXT,        -- NULL for folders, populated for endpoints
  method TEXT,     -- NULL for folders, populated for endpoints  
  prev BLOB,       -- Linked list pointer (ONLY here)
  next BLOB,       -- Linked list pointer (ONLY here)
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Legacy tables reference collection_items (NO prev/next)
CREATE TABLE item_folder (
  id BLOB NOT NULL PRIMARY KEY,
  collection_item_id BLOB NOT NULL, -- FK to collection_items.id
  collection_id BLOB NOT NULL,
  name TEXT NOT NULL,
  -- Remove: prev, next (moved to collection_items)
  FOREIGN KEY (collection_item_id) REFERENCES collection_items(id)
);

CREATE TABLE item_api (
  id BLOB NOT NULL PRIMARY KEY,
  collection_item_id BLOB NOT NULL, -- FK to collection_items.id
  collection_id BLOB NOT NULL,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  method TEXT NOT NULL,
  -- Remove: prev, next (moved to collection_items)
  FOREIGN KEY (collection_item_id) REFERENCES collection_items(id)
);
```

#### Service Layer Flow:

**Creating a Folder:**
1. Create `collection_items` entry (type=0, name, prev/next positioning)
2. Create `item_folder` entry with `collection_item_id` FK
3. Single transaction ensures consistency

**Creating an Endpoint:**
1. Create `collection_items` entry (type=1, name, url, method, prev/next positioning)
2. Create `item_api` entry with `collection_item_id` FK  
3. Single transaction ensures consistency

**Listing Items:**
1. Query `collection_items` with recursive CTE for ordering
2. JOIN with legacy tables for additional data if needed
3. Return mixed folder/endpoint list in correct order

**Moving Items:**
1. Update only `prev/next` in `collection_items` table
2. No changes to legacy tables needed
3. Works across types (folder → endpoint → folder)

### Benefits of This Approach:
- ✅ **Single source of truth** for ordering (collection_items)
- ✅ **Simplified move operations** (only update one table)
- ✅ **No dual-write complexity** or consistency issues
- ✅ **Clean separation** - ordering vs. type-specific data
- ✅ **Easy to extend** - new item types just need FK reference
- ✅ **Backward compatible** - legacy tables still contain their data