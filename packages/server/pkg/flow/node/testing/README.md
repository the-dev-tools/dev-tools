# Flow Node Testing Framework

This package provides a comprehensive, idiomatic Go testing framework for flow nodes. It offers simple, explicit helper functions and test suites for validating node behavior across different execution patterns.

## 🎯 Philosophy

This framework follows Go best practices:

- **Explicit over implicit** - Clear function names, no magic registration
- **Simple over complex** - Helper functions instead of framework classes
- **Composable over rigid** - Flexible test assembly
- **Clear intent** - Every function has an obvious purpose

## 🚀 Quick Start

### Basic Usage

```go
package mynode_test

import (
    "testing"
    "the-dev-tools/server/pkg/flow/node/testing"
)

func TestMyNode(t *testing.T) {
    // Create your node
    myNode := mynode.New(idwrap.NewNow(), "TestNode")

    // Test basic success
    opts := testing.DefaultTestNodeOptions()
    testing.TestNodeSuccess(t, myNode, opts)

    // Test error handling
    testing.TestNodeError(t, myNode, opts, func(req *node.FlowNodeRequest) {
        req.Timeout = 1 * time.Nanosecond // Force timeout
    })

    // Test async execution
    testing.TestNodeAsync(t, myNode, opts)
}
```

### Using Test Suites

```go
func TestAllNodeTypes(t *testing.T) {
    // Get all available node test configurations
    nodeTests := testing.AllNodeTests()

    for nodeName, suite := range nodeSuites {
        t.Run(nodeName, func(t *testing.T) {
            testNode := suite.Factory()
            testing.RunNodeTests(t, testNode, suite.TestCases)
        })
    }
}
```

## 📋 Core Components

### 1. Helper Functions (`helpers.go`)

#### Basic Test Functions

```go
// Test successful node execution
func TestNodeSuccess(t *testing.T, testNode node.FlowNode, opts TestNodeOptions)

// Test error handling with custom error condition
func TestNodeError(t *testing.T, testNode node.FlowNode, opts TestNodeOptions, errorFunc func(*node.FlowNodeRequest))

// Test timeout behavior
func TestNodeTimeout(t *testing.T, testNode node.FlowNode, opts TestNodeOptions)

// Test asynchronous execution
func TestNodeAsync(t *testing.T, testNode node.FlowNode, opts TestNodeOptions)
```

#### Configuration

```go
type TestNodeOptions struct {
    // Request configuration
    VarMap      map[string]any           // Variables for node execution
    EdgeMap     edge.EdgesMap           // Edge connections
    Timeout     time.Duration           // Execution timeout
    ExecutionID idwrap.IDWrap           // Execution identifier

    // Test behavior
    ExpectStatusEvents bool             // Whether node should emit status events
}

// Get sensible defaults
func DefaultTestNodeOptions() TestNodeOptions
```

#### Test Execution

```go
type NodeTestCase struct {
    Name     string
    TestFunc func(t *testing.T, ctx *TestContext, testNode node.FlowNode)
}

// Run multiple test cases for a node
func RunNodeTests(t *testing.T, testNode node.FlowNode, testCases []NodeTestCase)
```

### 2. Node Suites (`nodes.go`)

#### Predefined Node Creators

```go
// Get test configuration for FOR nodes
func FORNodeTests() NodeTests

// Get test configuration for FOREACH nodes
func FOREACHNodeTests() NodeTests

// Get test configuration for IF nodes
func IFNodeTests() NodeTests

// Get test configuration for NOOP nodes
func NOOPNodeTests() NodeTests

// Get all available node test configurations
func AllNodeTests() map[string]NodeTests
```

#### Node Test Configuration Structure

```go
type NodeTests struct {
    CreateNode  NodeCreator           // Creates node instances
    TestCases   []NodeTestCase        // Test cases to run
    BaseOptions TestNodeOptions       // Base configuration
}

type NodeCreator func() node.FlowNode
```

## 🔧 Advanced Usage

### Custom Node Tests

```go
func TestCustomNodeBehavior(t *testing.T) {
    // Create custom test case
    customTest := testing.NodeTestCase{
        Name: "Custom Behavior",
        TestFunc: func(t *testing.T, ctx *testing.TestContext, testNode node.FlowNode) {
            // Your custom test logic here
            opts := testing.DefaultTestNodeOptions()
            opts.VarMap = map[string]any{
                "customVar": "customValue",
            }

            testing.TestNodeSuccess(t, testNode, opts)

            // Additional custom assertions
            myNode := testNode.(*mynode.MyNode)
            assert.Equal(t, "expected", myNode.GetCustomProperty())
        },
    }

    // Run the custom test
    myNode := mynode.New(idwrap.NewNow(), "TestNode")
    testing.RunNodeTests(t, myNode, []testing.NodeTestCase{customTest})
}
```

### Testing Status-Emitting Nodes

```go
func TestStatusEmittingNode(t *testing.T) {
    node := getStatusEmittingNode()

    opts := testing.DefaultTestNodeOptions()
    opts.ExpectStatusEvents = true  // Important: enable status validation

    testing.TestNodeSuccess(t, node, opts)

    // The framework will automatically:
    // 1. Collect status events
    // 2. Validate status sequences
    // 3. Check execution patterns
}
```

### Error Injection Testing

```go
func TestErrorScenarios(t *testing.T) {
    node := mynode.New(idwrap.NewNow(), "TestNode")
    opts := testing.DefaultTestNodeOptions()

    // Test timeout errors
    testing.TestNodeError(t, node, opts, func(req *node.FlowNodeRequest) {
        req.Timeout = 1 * time.Nanosecond
    })

    // Test invalid variable data
    testing.TestNodeError(t, node, opts, func(req *node.FlowNodeRequest) {
        req.VarMap["invalid"] = struct{} // Invalid type
    })

    // Test missing required edges
    testing.TestNodeError(t, node, opts, func(req *node.FlowNodeRequest) {
        req.EdgeSourceMap = edge.EdgesMap{} // Empty edges
    })
}
```

### Adding New Node Types

#### 1. Create Node Factory

```go
// In nodes.go or your test file
func MyCustomNodeTests() testing.NodeTests {
    return testing.NodeTests{
        CreateNode: func() node.FlowNode {
            return mycustom.New(
                idwrap.NewNow(),
                "TestCustom",
                // ... constructor parameters
            )
        },
        BaseOptions: testing.TestNodeOptions{
            ExpectStatusEvents: false, // Set based on your node
            Timeout: 5 * time.Second,
        },
        TestCases: []testing.NodeTestCase{
            {
                Name: "Basic Success",
                TestFunc: func(t *testing.T, ctx *testing.TestContext, testNode node.FlowNode) {
                    opts := testing.DefaultTestNodeOptions()
                    testing.TestNodeSuccess(t, testNode, opts)
                },
            },
            {
                Name: "Custom Behavior",
                TestFunc: func(t *testing.T, ctx *testing.TestContext, testNode node.FlowNode) {
                    // Node-specific test logic
                    customNode := testNode.(*mycustom.NodeCustom)
                    assert.NotNil(t, customNode.GetCustomFeature())
                },
            },
        },
    }
}
```

#### 2. Register in All Tests (Optional)

```go
func AllNodeTests() map[string]testing.NodeTests {
    return map[string]testing.NodeTests{
        "FOR":     FORNodeTests(),
        "FOREACH": FOREACHNodeTests(),
        "IF":      IFNodeTests(),
        "NOOP":    NOOPNodeTests(),
        "CUSTOM":  MyCustomNodeTests(), // Add your node
    }
}
```

## 🧪 Test Patterns

### 1. Basic Success Test

```go
func TestNodeBasicSuccess(t *testing.T) {
    node := createNode()
    opts := testing.DefaultTestNodeOptions()

    testing.TestNodeSuccess(t, node, opts)
}
```

### 2. Error Handling Test

```go
func TestNodeErrorHandling(t *testing.T) {
    node := createNode()
    opts := testing.DefaultTestNodeOptions()

    testing.TestNodeError(t, node, opts, func(req *node.FlowNodeRequest) {
        // Inject error condition
        req.Timeout = 1 * time.Nanosecond
    })
}
```

### 3. Async Execution Test

```go
func TestNodeAsyncExecution(t *testing.T) {
    node := createNode()
    opts := testing.DefaultTestNodeOptions()

    testing.TestNodeAsync(t, node, opts)
}
```

### 4. Comprehensive Test Suite

```go
func TestNodeComprehensive(t *testing.T) {
    node := createNode()

    testCases := []testing.NodeTestCase{
        {Name: "Success", TestFunc: successTest},
        {Name: "Error", TestFunc: errorTest},
        {Name: "Timeout", TestFunc: timeoutTest},
        {Name: "Async", TestFunc: asyncTest},
        {Name: "Custom", TestFunc: customTest},
    }

    testing.RunNodeTests(t, node, testCases)
}
```

## 📊 Status Event Testing

### For Status-Emitting Nodes (FOR, etc.)

```go
func TestStatusEmittingNode(t *testing.T) {
    node := forNode.New(idwrap.NewNow(), "TestFOR", 3, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

    opts := testing.DefaultTestNodeOptions()
    opts.ExpectStatusEvents = true  // Enable status validation
    opts.EdgeMap = edge.EdgesMap{
        node.GetID(): {
            edge.HandleLoop: {}, // Empty loop
        },
    }

    testing.TestNodeSuccess(t, node, opts)

    // Framework automatically validates:
    // - Status sequence correctness
    // - Execution ID reuse patterns
    // - Iteration context consistency
    // - Error state transitions
}
```

### For Simple Nodes (NOOP, IF, etc.)

```go
func TestSimpleNode(t *testing.T) {
    node := noop.New(idwrap.NewNow(), "TestNOOP")

    opts := testing.DefaultTestNodeOptions()
    opts.ExpectStatusEvents = false  // Simple nodes don't emit status

    testing.TestNodeSuccess(t, node, opts)

    // Framework focuses on:
    // - Successful execution
    // - Proper error handling
    // - Async behavior
}
```

## 🛠️ Integration with Existing Tests

### Migrating Manual Tests

```go
// Before (manual test)
func TestMyNodeManual(t *testing.T) {
    ctx := context.Background()
    node := mynode.New(idwrap.NewNow(), "test")

    req := &node.FlowNodeRequest{
        VarMap: make(map[string]any),
        // ... manual setup
    }

    result := node.RunSync(ctx, req)
    if result.Err != nil {
        t.Errorf("Node failed: %v", result.Err)
    }
    // ... manual validation
}

// After (using framework)
func TestMyNodeFramework(t *testing.T) {
    node := mynode.New(idwrap.NewNow(), "test")
    opts := testing.DefaultTestNodeOptions()

    testing.TestNodeSuccess(t, node, opts)
    // Framework handles setup, execution, validation, cleanup
}
```

### Combining with Existing Infrastructure

```go
func TestNodeWithExistingInfrastructure(t *testing.T) {
    // Use existing test setup
    db := setupTestDB(t)
    defer db.Close()

    // Create node with existing dependencies
    node := mynode.New(idwrap.NewNow(), "test", db)

    // Use framework for testing
    opts := testing.DefaultTestNodeOptions()
    opts.VarMap = map[string]any{
        "dbConnection": db,
    }

    testing.TestNodeSuccess(t, node, opts)
}
```

## 🔍 Best Practices

### 1. Test Organization

```go
// Group related tests
func TestMyNode_Scenarios(t *testing.T) {
    t.Run("Success", func(t *testing.T) {
        testing.TestNodeSuccess(t, node, opts)
    })

    t.Run("Error", func(t *testing.T) {
        testing.TestNodeError(t, node, opts, errorFunc)
    })

    t.Run("Async", func(t *testing.T) {
        testing.TestNodeAsync(t, node, opts)
    })
}
```

### 2. Configuration Reuse

```go
func TestMyNode(t *testing.T) {
    // Base configuration
    baseOpts := testing.DefaultTestNodeOptions()
    baseOpts.Timeout = 5 * time.Second
    baseOpts.VarMap = map[string]any{
        "commonVar": "commonValue",
    }

    // Test variations
    t.Run("Scenario1", func(t *testing.T) {
        opts := baseOpts
        opts.VarMap["scenarioVar"] = "value1"
        testing.TestNodeSuccess(t, node, opts)
    })
}
```

### 3. Custom Assertions

```go
func TestNodeWithCustomAssertions(t *testing.T) {
    node := createNode()

    customTest := testing.NodeTestCase{
        Name: "Custom Assertions",
        TestFunc: func(t *testing.T, ctx *testing.TestContext, testNode node.FlowNode) {
            // Use framework for basic testing
            opts := testing.DefaultTestNodeOptions()
            testing.TestNodeSuccess(t, testNode, opts)

            // Add custom assertions
            myNode := testNode.(*mynode.MyNode)
            assert.Equal(t, "expected", myNode.GetState())
            assert.True(t, myNode.IsInitialized())
        },
    }

    testing.RunNodeTests(t, node, []testing.NodeTestCase{customTest})
}
```

## 📈 Performance Considerations

### Benchmark Testing

```go
func BenchmarkMyNode(b *testing.B) {
    node := mynode.New(idwrap.NewNow(), "BenchmarkNode")
    opts := testing.DefaultTestNodeOptions()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        testing.TestNodeSuccess(&testing.T{}, node, opts)
    }
}
```

### Concurrent Testing

```go
func TestNodeConcurrency(t *testing.T) {
    node := mynode.New(idwrap.NewNow(), "ConcurrentNode")
    opts := testing.DefaultTestNodeOptions()

    // Test concurrent execution
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            testing.TestNodeSuccess(&testing.T{}, node, opts)
        }()
    }
    wg.Wait()
}
```

## 🐛 Troubleshooting

### Common Issues

#### 1. "Expected status events but got none"

```go
// Fix: Set ExpectStatusEvents correctly
opts := testing.DefaultTestNodeOptions()
opts.ExpectStatusEvents = true  // For FOR nodes
opts.ExpectStatusEvents = false // For NOOP, IF, etc.
```

#### 2. "Node factory returned nil"

```go
// Fix: Ensure factory returns valid node
func GetMyNodeSuite() testing.NodeTestSuite {
    return testing.NodeTests{
        CreateNode: func() node.FlowNode {
            // Make sure this returns a valid node
            return mynode.New(idwrap.NewNow(), "TestNode")
        },
    }
}
```

#### 3. Test timeouts

```go
// Fix: Increase timeout in options
opts := testing.DefaultTestNodeOptions()
opts.Timeout = 30 * time.Second  // Increase for slow nodes
```

### Debug Mode

```go
func TestNodeDebug(t *testing.T) {
    node := createNode()

    // Enable detailed logging
    opts := testing.DefaultTestNodeOptions()
    opts.Timeout = 30 * time.Second

    // Create test context with debug options
    ctx := testing.NewTestContext(t, testing.TestContextOptions{
        Timeout: opts.Timeout,
        Debug:   true,  // Enable debug logging
    })
    defer ctx.Cleanup()

    // Manual execution with debugging
    req := ctx.CreateNodeRequest(node.GetID(), node.GetName(), testing.NodeRequestOptions{
        VarMap:        opts.VarMap,
        EdgeSourceMap: opts.EdgeMap,
        ExecutionID:   opts.ExecutionID,
        Timeout:       opts.Timeout,
    })

    result := node.RunSync(ctx.Context(), req)
    t.Logf("Node result: %+v", result)
    t.Logf("Statuses collected: %d", len(ctx.Collector().GetAll()))
}
```

## 📚 Reference

### Function Index

#### Core Helpers

- `TestNodeSuccess()` - Test successful execution
- `TestNodeError()` - Test error handling
- `TestNodeTimeout()` - Test timeout behavior
- `TestNodeAsync()` - Test async execution
- `RunNodeTests()` - Run multiple test cases

#### Configuration

- `DefaultTestNodeOptions()` - Get default configuration
- `TestNodeOptions` - Configuration struct

#### Node Factories

- `FORNodeTests()` - FOR node test configuration
- `FOREACHNodeTests()` - FOREACH node test configuration
- `IFNodeTests()` - IF node test configuration
- `NOOPNodeTests()` - NOOP node test configuration
- `AllNodeTests()` - All node test configurations

### Type Definitions

```go
type NodeTestCase struct {
    Name     string
    TestFunc func(t *testing.T, ctx *TestContext, testNode node.FlowNode)
}

type NodeTests struct {
    CreateNode  NodeCreator
    TestCases   []NodeTestCase
    BaseOptions TestNodeOptions
}

type TestNodeOptions struct {
    VarMap             map[string]any
    EdgeMap            edge.EdgesMap
    Timeout            time.Duration
    ExecutionID        idwrap.IDWrap
    ExpectStatusEvents bool
}
```

## 🤝 Contributing

When adding new node types or test patterns:

1. **Follow existing patterns** - Use the same structure as existing node suites
2. **Keep it simple** - Prefer explicit helper functions over complex abstractions
3. **Document behavior** - Add comments explaining node-specific test logic
4. **Test the tests** - Ensure your test cases actually validate the intended behavior
5. **Handle both sync and async** - Test both execution patterns when applicable

## 📄 License

This testing framework follows the same license as the main project.
