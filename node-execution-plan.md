# NodeExecution Enhancement Plan

## Overview
Implement enhancements to the NodeExecution system based on Slack conversation requirements.

## Tasks

### 1. Add "name" field to NodeExecution
- **Database**: Add `name TEXT NOT NULL` column to `node_execution` table in schema.sql
- **Model**: Add `Name string` field to `mnodeexecution.NodeExecution` struct
- **Service**: Update `ConvertNodeExecutionToDB` and `ConvertNodeExecutionToModel` to handle name field
- **RPC**: The TypeSpec already has the name field defined, so no changes needed there
- **Flow Execution**: In `rflow.go`, generate execution names like "Execution 1", "Execution 2" based on node execution count

### 2. Verify execution ordering (Already Implemented ✓)
- Current SQL query uses `ORDER BY completed_at DESC` which returns newest first
- No changes needed

### 3. Add responseId support for REQUEST nodes
- **Database**: Add `response_id BLOB` column to `node_execution` table
- **Model**: Add `ResponseID *idwrap.IDWrap` field to `mnodeexecution.NodeExecution`
- **Service**: Update conversion functions to handle responseId
- **Flow Execution**: Set responseId when creating NodeExecution for REQUEST nodes
- **RPC**: TypeSpec already has responseId field defined

### 4. Remove OUTPUT_KIND
- **Database**: Remove `output_kind` column from schema
- **Model**: Remove `OutputKind` field from `mnodeexecution.NodeExecution`
- **Service**: Remove OutputKind handling from conversion functions
- **Flow Execution**: Remove OutputKind assignment logic

## Implementation Order
1. Database schema changes (direct edits to schema.sql)
2. Regenerate sqlc code: `cd packages/db/pkg/sqlc && sqlc generate`
3. Model updates
4. Service layer updates
5. Flow execution logic updates
6. Testing and verification

## Files to be Modified
- `/packages/db/pkg/sqlc/schema.sql`
- `/packages/db/pkg/sqlc/query.sql`
- `/packages/server/pkg/model/mnodeexecution/mnodeexecution.go`
- `/packages/server/pkg/service/snodeexecution/snodeexecution.go`
- `/packages/server/internal/api/rflow/rflow.go`
- `/packages/server/pkg/translate/tnodeexecution/tnodeexecution.go`

## Generated Files (after sqlc generate)
- `/packages/db/pkg/sqlc/gen/models.go`
- `/packages/db/pkg/sqlc/gen/query.sql.go`
- `/packages/db/pkg/sqlc/gen/db.go`