# Delta Raw Body Fix Summary

## Problem
When importing simplified YAML workflows, the flow execution was failing with error:
```
{"error":{"code":"internal","message":"delta raw body not found"}}
```

This happened because the import logic was only creating delta raw bodies when there were actual body overrides (changes or variables), but the flow execution in `rflow.go` expects ALL delta examples to have corresponding raw bodies.

## Root Cause
In `/packages/server/internal/api/rflow/rflow.go` lines 676-679, the flow execution code tries to fetch delta raw bodies for ALL delta examples:
```go
rawBodyDelta, err := c.brs.GetBodyRawByExampleID(ctx, deltaExample.ID)
if err != nil {
    return connect.NewError(connect.CodeInternal, errors.New("delta raw body not found"))
}
```

## Solution
Updated the import logic in `workflowsimple.go` to ALWAYS create delta raw bodies when creating delta examples, regardless of whether there are actual body overrides:

1. Removed the conditional logic that only created delta bodies when `bodyNeedsDelta` was true
2. Now always creates delta raw bodies for consistency with flow execution expectations
3. This matches the pattern used in HAR import (`thar.go`)

## Changes Made

### 1. `/packages/server/pkg/io/workflow/workflowsimple/workflowsimple.go`
- Lines 707-777: Removed `bodyNeedsDelta` logic
- Always create delta raw body when processing request steps
- Added comments explaining why delta bodies are always needed

### 2. `/packages/server/pkg/io/workflow/workflowsimple/delta_test.go`
- Updated test expectations to expect delta bodies for all requests
- Changed `bodies: 0` to `bodies: 1` in all test cases

### 3. Added new test: `/packages/server/pkg/io/workflow/workflowsimple/delta_rawbody_test.go`
- Verifies that delta raw bodies are always created
- Tests various scenarios: with body, without body, with template override

## Result
- Import now creates delta raw bodies for ALL delta examples
- Flow execution no longer fails with "delta raw body not found" error
- All tests pass
- The system is consistent with how HAR import works