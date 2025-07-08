# Base vs Delta Separation Fix Summary

## Problem
When importing simplified YAML workflows, base request templates were incorrectly getting flow-specific variable references (like `{{ request_0.response.body.token }}`) instead of keeping their static template values (like hardcoded JWT tokens).

## Root Cause
The `processRequestStep` function was mixing template values with step overrides when creating base headers, queries, and bodies. This meant that base examples (which should represent the template) were getting contaminated with flow-specific overrides.

## Solution
Redesigned the processing logic to maintain clear separation between:
1. **Base values**: Always use template values for base examples
2. **Delta values**: Use step override values for delta examples

### Key Changes

1. **Separated Override Collection** (lines 380-421)
   - Collect step overrides in separate variables (`stepHeaderOverrides`, `stepQueryOverrides`, `stepBodyOverride`)
   - Don't mix them with template data during initial processing

2. **Headers Processing** (lines 497-644)
   - Process template headers first, always using template values for base
   - Create deltas only for headers that are overridden in the step
   - Handle both template overrides and new header additions

3. **Query Parameters Processing** (lines 646-802)
   - Same pattern as headers: template values for base, overrides for delta
   - Proper handling of both map formats

4. **Body Processing** (lines 804-871)
   - Template body goes to base example
   - Override body goes to delta example
   - Always create delta bodies (required by flow execution)

## Results

### Before Fix
```yaml
requests:
  - name: api_call
    headers:
      Authorization: Bearer {{ request_0.response.body.token }}  # WRONG: Variable in template
```

### After Fix
```yaml
requests:
  - name: api_call
    headers:
      Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...  # CORRECT: Static token

flows:
  - steps:
      - request:
          name: api_call_override
          use_request: api_call
          headers:
            Authorization: Bearer {{ request_0.response.body.token }}  # CORRECT: Variable in override
```

## Tests Added

1. **base_delta_separation_test.go**
   - Tests template with header override
   - Tests template with body override
   - Tests no template with variables

2. **base_delta_roundtrip_test.go**
   - Tests import/export roundtrip preserves separation
   - Tests hardcoded JWT tokens are preserved in templates

## Impact
- Import now correctly preserves the separation between base templates and flow-specific overrides
- Export shows clean base templates without variable pollution
- Flow execution works correctly with proper delta values
- Maintains backward compatibility with existing workflows