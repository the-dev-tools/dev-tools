# Flow Node Testing - Quick Start

## 🚀 5-Minute Guide

### 1. Basic Test

```go
func TestMyNode(t *testing.T) {
    node := mynode.New(idwrap.NewNow(), "TestNode")
    opts := testing.DefaultTestNodeOptions()

    testing.TestNodeSuccess(t, node, opts)
}
```

### 2. Test All Scenarios

```go
func TestMyNodeComplete(t *testing.T) {
    node := mynode.New(idwrap.NewNow(), "TestNode")
    opts := testing.DefaultTestNodeOptions()

    // Success case
    testing.TestNodeSuccess(t, node, opts)

    // Error case
    testing.TestNodeError(t, node, opts, func(req *node.FlowNodeRequest) {
        req.Timeout = 1 * time.Nanosecond // Force timeout
    })

    // Async case
    testing.TestNodeAsync(t, node, opts)
}
```

### 3. Use Predefined Suites

```go
func TestAllNodes(t *testing.T) {
    // Test all available node types
    testing.TestAllNodes(t)

    // Or test specific node type
    testing.TestFORNode(t)
    testing.TestNOOPNode(t)
}
```

### 4. Custom Test Case

```go
func TestMyNodeCustom(t *testing.T) {
    node := mynode.New(idwrap.NewNow(), "TestNode")

    customTest := testing.NodeTestCase{
        Name: "Custom Behavior",
        TestFunc: func(t *testing.T, ctx *testing.TestContext, testNode node.FlowNode) {
            // Your custom test logic
            myNode := testNode.(*mynode.MyNode)
            assert.Equal(t, "expected", myNode.GetCustomProperty())
        },
    }

    testing.RunNodeTests(t, node, []testing.NodeTestCase{customTest})
}
```

## 📋 Key Functions

| Function                   | Purpose                   |
| -------------------------- | ------------------------- |
| `TestNodeSuccess()`        | Test successful execution |
| `TestNodeError()`          | Test error handling       |
| `TestNodeTimeout()`        | Test timeout behavior     |
| `TestNodeAsync()`          | Test async execution      |
| `RunNodeTests()`           | Run multiple test cases   |
| `DefaultTestNodeOptions()` | Get default config        |

## ⚙️ Configuration

```go
opts := testing.DefaultTestNodeOptions()
opts.Timeout = 30 * time.Second           // Increase timeout
opts.ExpectStatusEvents = true           // For status-emitting nodes
opts.VarMap = map[string]any{            // Add variables
    "customVar": "customValue",
}
```

## 🎯 Node Types

| Node    | Status Events | Usage                        |
| ------- | ------------- | ---------------------------- |
| FOR     | ✅ Yes        | `testing.TestFORNode(t)`     |
| FOREACH | ✅ Yes        | `testing.TestFOREACHNode(t)` |
| IF      | ❌ No         | `testing.TestIFNode(t)`      |
| NOOP    | ❌ No         | `testing.TestNOOPNode(t)`    |

## 🔧 Add New Node

```go
func MyNodeTests() testing.NodeTests {
    return testing.NodeTests{
        CreateNode: func() node.FlowNode {
            return mynode.New(idwrap.NewNow(), "TestNode")
        },
        TestCases: []testing.NodeTestCase{
            {Name: "Success", TestFunc: successTest},
            {Name: "Error", TestFunc: errorTest},
        },
    }
}
```

That's it! You're ready to test flow nodes. 🎉

For detailed documentation, see [README.md](./README.md).
