# Testing Strategy: Normal Unit Tests vs Flow Node Framework

## 🎯 The Core Question

**"When should I use normal unit tests vs the flow node testing framework?"**

This is **not** an either/or choice - they serve different purposes and complement each other. Think of it as having two specialized tools in your testing toolbox.

---

## 📊 Quick Decision Matrix

| **Scenario**                          | **Use Normal Unit Test** | **Use Flow Node Framework** |
| ------------------------------------- | ------------------------ | --------------------------- |
| Testing business logic functions      | ✅                       | ❌                          |
| Testing utility/helper functions      | ✅                       | ❌                          |
| Testing flow node execution           | ❌                       | ✅                          |
| Testing node status events            | ❌                       | ✅                          |
| Testing edge/variable handling        | ❌                       | ✅                          |
| Testing data transformations          | ✅                       | ❌                          |
| Testing async node behavior           | ❌                       | ✅                          |
| Testing error conditions in nodes     | ❌                       | ✅                          |
| Testing algorithm implementations     | ✅                       | ❌                          |
| Testing node configuration validation | ❌                       | ✅                          |

---

## 🏗️ Normal Unit Tests: What They're For

### Purpose

Test **specific pieces of functionality** in isolation with direct assertions.

### When to Use

```go
// ✅ GOOD: Testing business logic
func TestConditionEvaluator_EvaluateExpression(t *testing.T) {
    evaluator := NewConditionEvaluator()

    result, err := evaluator.Evaluate("1 == 1")
    require.NoError(t, err)
    require.True(t, result)
}

// ✅ GOOD: Testing utility functions
func TestVariableHelper_SanitizeName(t *testing.T) {
    result := SanitizeVariableName("invalid-name!")
    require.Equal(t, "invalid_name", result)
}

// ✅ GOOD: Testing data transformations
func TestDataTransformer_ConvertToMap(t *testing.T) {
    input := []string{"a", "b", "c"}
    result := ConvertToMap(input)
    require.Equal(t, map[string]bool{"a": true, "b": true, "c": true}, result)
}
```

### Characteristics

- **Direct assertions**: `require.Equal(t, expected, actual)`
- **Isolated functionality**: Test one thing at a time
- **Fast execution**: No complex setup
- **Full control**: You create all inputs and verify all outputs
- **Business logic focus**: Algorithms, calculations, transformations

---

## 🔄 Flow Node Framework: What It's For

### Purpose

Test **flow node execution patterns** including status events, edge handling, async behavior, and integration scenarios.

### When to Use

```go
// ✅ GOOD: Testing node execution with framework
func TestIFNode_Framework(t *testing.T) {
    creator := func() node.FlowNode {
        return nif.New(idwrap.NewNow(), "test", condition)
    }

    opts := nodetesting.DefaultTestNodeOptions()
    opts.EdgeMap = edge.EdgesMap{
        creator().GetID(): {
            edge.HandleThen: {idwrap.NewNow()},
            edge.HandleElse: {idwrap.NewNow()},
        },
    }

    nodetesting.TestNodeSuccess(t, creator(), opts)
}

// ✅ GOOD: Testing error scenarios
func TestFORNode_TimeoutHandling(t *testing.T) {
    node := createFORNode()
    opts := nodetesting.DefaultTestNodeOptions()

    nodetesting.TestNodeError(t, node, opts, func(req *node.FlowNodeRequest) {
        req.Timeout = 1 * time.Nanosecond // Force timeout
    })
}

// ✅ GOOD: Testing status events for complex nodes
func TestFORNode_StatusEvents(t *testing.T) {
    node := createFORNode()
    opts := nodetesting.DefaultTestNodeOptions()
    opts.ExpectStatusEvents = true // Enable status validation

    nodetesting.TestNodeSuccess(t, node, opts)
    // Framework automatically validates status sequence
}
```

### Characteristics

- **Execution patterns**: How nodes run, not just what they return
- **Status event handling**: Collection and validation of node status
- **Edge/variable integration**: Proper handling of flow control
- **Async behavior**: Testing both sync and async execution
- **Reduced boilerplate**: Framework handles complex setup

---

## 🔄 Real-World Examples

### Example 1: IF Node Testing

#### Normal Unit Test (Business Logic)

```go
// Test the condition evaluation logic directly
func TestConditionLogic_Evaluate(t *testing.T) {
    tests := []struct {
        name     string
        expr     string
        vars     map[string]any
        expected bool
    }{
        {"True condition", "1 == 1", nil, true},
        {"False condition", "1 == 2", nil, false},
        {"Variable reference", "count > 0", map[string]any{"count": 5}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := EvaluateCondition(tt.expr, tt.vars)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

#### Framework Test (Node Execution)

```go
// Test the IF node execution through framework
func TestIFNode_Execution(t *testing.T) {
    creator := func() node.FlowNode {
        return nif.New(idwrap.NewNow(), "test", mcondition.Condition{
            Comparisons: mcondition.Comparison{Expression: "count > 0"},
        })
    }

    testCases := []nodetesting.NodeTestCase{
        {
            Name: "True condition takes then branch",
            TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
                opts := nodetesting.DefaultTestNodeOptions()
                opts.VarMap = map[string]any{"count": 5}
                opts.EdgeMap = edge.EdgesMap{
                    testNode.GetID(): {
                        edge.HandleThen: {idwrap.NewNow()},
                        edge.HandleElse: {idwrap.NewNow()},
                    },
                }

                nodetesting.TestNodeSuccess(t, testNode, opts)
            },
        },
        {
            Name: "False condition takes else branch",
            TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
                opts := nodetesting.DefaultTestNodeOptions()
                opts.VarMap = map[string]any{"count": 0}
                opts.EdgeMap = edge.EdgesMap{
                    testNode.GetID(): {
                        edge.HandleThen: {idwrap.NewNow()},
                        edge.HandleElse: {idwrap.NewNow()},
                    },
                }

                nodetesting.TestNodeSuccess(t, testNode, opts)
            },
        },
    }

    nodetesting.RunNodeTests(t, creator(), testCases)
}
```

### Example 2: FOR Node Testing

#### Normal Unit Test (Loop Logic)

```go
// Test the loop counting logic directly
func TestLoopCounter_CalculateIterations(t *testing.T) {
    tests := []struct {
        name     string
        start    int
        end      int
        step     int
        expected int
    }{
        {"Positive range", 0, 5, 1, 5},
        {"Negative range", 5, 0, -1, 5},
        {"Step greater than 1", 0, 10, 2, 5},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := CalculateIterations(tt.start, tt.end, tt.step)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

#### Framework Test (Node Execution + Status Events)

```go
// Test the FOR node execution and status events
func TestFORNode_StatusEventSequence(t *testing.T) {
    creator := func() node.FlowNode {
        return nfor.New(idwrap.NewNow(), "test", 0, 3, 1, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
    }

    opts := nodetesting.DefaultTestNodeOptions()
    opts.EdgeMap = edge.EdgesMap{
        creator().GetID(): {
            edge.HandleLoop: {}, // Empty loop for testing
        },
    }
    opts.ExpectStatusEvents = true // Enable status validation

    nodetesting.TestNodeSuccess(t, creator(), opts)

    // Framework automatically validates:
    // - Status sequence: RUNNING → COMPLETED
    // - Iteration context consistency
    // - Execution ID reuse patterns
    // - Proper status transitions
}
```

---

## 🎯 When to Choose Which

### Choose Normal Unit Tests When:

1. **Testing Pure Functions**

   ```go
   func TestMathUtils_Add(t *testing.T) {
       result := Add(2, 3)
       require.Equal(t, 5, result)
   }
   ```

2. **Testing Business Logic**

   ```go
   func TestValidator_ValidateEmail(t *testing.T) {
       err := ValidateEmail("user@example.com")
       require.NoError(t, err)
   }
   ```

3. **Testing Data Transformations**

   ```go
   func TestConverter_MapToStruct(t *testing.T) {
       result := MapToStruct(inputMap)
       require.Equal(t, expectedStruct, result)
   }
   ```

4. **Testing Algorithm Implementation**
   ```go
   func TestSorter_QuickSort(t *testing.T) {
       result := QuickSort([]int{3, 1, 2})
       require.Equal(t, []int{1, 2, 3}, result)
   }
   ```

### Choose Flow Node Framework When:

1. **Testing Node Execution**

   ```go
   func TestMyNode_Execution(t *testing.T) {
       nodetesting.TestNodeSuccess(t, myNode, opts)
   }
   ```

2. **Testing Error Handling**

   ```go
   func TestMyNode_ErrorScenarios(t *testing.T) {
       nodetesting.TestNodeError(t, myNode, opts, errorFunc)
   }
   ```

3. **Testing Status Events**

   ```go
   func TestMyNode_StatusEvents(t *testing.T) {
       opts.ExpectStatusEvents = true
       nodetesting.TestNodeSuccess(t, myNode, opts)
   }
   ```

4. **Testing Async Behavior**
   ```go
   func TestMyNode_AsyncExecution(t *testing.T) {
       nodetesting.TestNodeAsync(t, myNode, opts)
   }
   ```

---

## 🚫 Anti-Patterns to Avoid

### Don't Use Framework For:

```go
// ❌ BAD: Using framework for simple business logic
func TestMath_Add_Framework(t *testing.T) {
    // This is overkill for testing 2 + 2 = 4
    creator := func() node.FlowNode { return mathNode.New() }
    opts := nodetesting.DefaultTestNodeOptions()
    nodetesting.TestNodeSuccess(t, creator(), opts)
}
```

### Don't Use Normal Tests For:

```go
// ❌ BAD: Manual node testing when framework exists
func TestMyNode_Manual(t *testing.T) {
    // 50+ lines of boilerplate that framework handles
    id := idwrap.NewNow()
    node := mynode.New(id, "test")
    edgeMap := edge.EdgesMap{...}
    req := &node.FlowNodeRequest{...}
    result := node.RunSync(ctx, req)
    // ... manual validation ...
}
```

---

## 📈 Benefits Summary

### Normal Unit Tests

- **Precision**: Test exactly what you want
- **Speed**: Fast execution, minimal setup
- **Clarity**: Direct cause-and-effect
- **Control**: Complete control over inputs/outputs
- **Best for**: Business logic, algorithms, utilities

### Flow Node Framework

- **Consistency**: Standardized patterns across all nodes
- **Coverage**: Built-in testing for common scenarios
- **Integration**: Handles complex node interactions
- **Maintenance**: Reduced boilerplate, easier updates
- **Best for**: Node execution, status events, edge cases

---

## 🎯 The Golden Rule

**Use normal unit tests for business logic, use the flow node framework for flow node execution.**

If you're asking "Does this test need to know about edges, status events, or node execution patterns?" → **Use Framework**

If you're asking "Does this test need to verify a calculation, transformation, or business rule?" → **Use Normal Unit Test**

---

## 📚 Related Documentation

- **Framework README**: `packages/server/pkg/flow/node/testing/README.md`
- **Migration Guide**: `packages/server/pkg/flow/node/testing/MIGRATION.md`
- **Server Testing Guide**: `packages/server/testing.md`
- **Example Tests**: `packages/server/pkg/flow/node/nif/nif_framework_test.go`

---

## 🤝 Contributing

When adding new tests:

1. **Ask the question**: "Am I testing business logic or node execution?"
2. **Choose the right tool**: Normal unit test vs framework
3. **Follow patterns**: Use existing examples as templates
4. **Document decisions**: Add comments explaining complex test scenarios

Remember: Both approaches are valuable and necessary. The key is using each for what it's best at.
