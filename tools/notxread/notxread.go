// Package notxread provides a Go analyzer that detects read operations on non-TX-bound
// services inside SQLite write transactions. This prevents deadlocks in file-based SQLite.
//
// In file-based SQLite, reading via a non-TX connection while holding a write TX causes
// deadlock. The correct pattern is either:
//  1. Read BEFORE starting the transaction
//  2. Use a TX-bound service via .TX(tx).Get()
package notxread

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer detects non-TX-bound read operations inside write transactions.
var Analyzer = &analysis.Analyzer{
	Name:     "notxread",
	Doc:      "Detects read operations on non-TX-bound services inside write transactions (SQLite deadlock prevention)",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// readMethodPrefixes are method name prefixes that indicate read operations.
var readMethodPrefixes = []string{
	"Get",
	"List",
	"Find",
	"Read",
	"Fetch",
	"Load",
	"Query",
	"Search",
	"Lookup",
	"Select",
}

// allowedPackages are package path patterns where TX read checks are skipped.
var allowedPackages = []string{
	"_test",      // Test files (covered by file name, but also path)
	"/sqlc/",     // SQLC generated code
	"/migrate",   // Migration code
	"/dbtest",    // DB test utilities
}

// txState tracks transaction state within a function.
type txState struct {
	beginTxPos  token.Pos         // Position of BeginTx call
	commitPos   token.Pos         // Position of Commit/Rollback call (0 if not found)
	txBoundVars map[string]bool   // Variables created from .TX(tx) calls
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Skip packages where TX checks are not needed
	pkgPath := pass.Pkg.Path()
	for _, allowed := range allowedPackages {
		if strings.Contains(pkgPath, allowed) {
			return nil, nil
		}
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// First pass: analyze each function for TX patterns
	funcFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
	}

	insp.Preorder(funcFilter, func(n ast.Node) {
		var body *ast.BlockStmt

		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body == nil {
				return
			}
			body = fn.Body
		case *ast.FuncLit:
			body = fn.Body
		default:
			return
		}

		analyzeFunction(pass, body)
	})

	return nil, nil
}

// analyzeFunction analyzes a single function body for TX read violations.
func analyzeFunction(pass *analysis.Pass, body *ast.BlockStmt) {
	state := &txState{
		txBoundVars: make(map[string]bool),
	}

	// First, find BeginTx position and track TX-bound variables
	// Also find the LAST Commit/Rollback (not the first, because defer Rollback comes early)
	var lastCommitPos token.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		// Track assignments for TX-bound variables first
		if assign, ok := n.(*ast.AssignStmt); ok {
			trackTXBoundAssignment(pass, assign, state)
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name

		// Track BeginTx
		if methodName == "BeginTx" {
			state.beginTxPos = call.Pos()
		}

		// Track Commit (but not Rollback in defer, which comes early in source order)
		// Only track Commit as the TX boundary end
		if methodName == "Commit" && state.beginTxPos != 0 {
			lastCommitPos = call.Pos()
		}

		return true
	})

	// Use the last Commit position
	state.commitPos = lastCommitPos

	// If no BeginTx found, nothing to check
	if state.beginTxPos == 0 {
		return
	}

	// Second pass: find TX-bound variables and read calls
	ast.Inspect(body, func(n ast.Node) bool {
		// Track assignment statements for TX-bound variables
		if assign, ok := n.(*ast.AssignStmt); ok {
			trackTXBoundAssignment(pass, assign, state)
		}

		// Check call expressions
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Skip calls before BeginTx or after Commit
		if call.Pos() <= state.beginTxPos {
			return true
		}
		if state.commitPos != 0 && call.Pos() >= state.commitPos {
			return true
		}

		checkReadCall(pass, call, state)
		return true
	})
}

// trackTXBoundAssignment tracks variables created from .TX(tx) calls.
func trackTXBoundAssignment(pass *analysis.Pass, assign *ast.AssignStmt, state *txState) {
	// Look for patterns like: txService := service.TX(tx)
	for i, rhs := range assign.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			continue
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		// Check if method name is "TX"
		if sel.Sel.Name != "TX" {
			continue
		}

		// Get the LHS variable name
		if i < len(assign.Lhs) {
			if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
				state.txBoundVars[ident.Name] = true
			}
		}
	}
}

// checkReadCall checks if a call is a read operation on a non-TX-bound service.
func checkReadCall(pass *analysis.Pass, call *ast.CallExpr, state *txState) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	methodName := sel.Sel.Name

	// Check if this is a read method
	if !isReadMethod(methodName) {
		return
	}

	// Get the receiver
	receiverIdent := getReceiverIdent(sel.X)
	if receiverIdent == nil {
		return
	}

	receiverName := receiverIdent.Name

	// Skip if receiver is TX-bound
	if state.txBoundVars[receiverName] {
		return
	}

	// Get receiver type to check for Writer types
	tv, ok := pass.TypesInfo.Types[sel.X]
	if ok && isWriterType(tv.Type) {
		return
	}

	// Skip certain receiver names that are commonly safe
	if isSafeReceiverName(receiverName) {
		return
	}

	pass.Reportf(call.Pos(),
		"non-TX-bound read %s() inside transaction may cause SQLite deadlock; move read before BeginTx or use %s.TX(tx).%s()",
		methodName, receiverName, methodName)
}

// isReadMethod checks if a method name indicates a read operation.
func isReadMethod(name string) bool {
	for _, prefix := range readMethodPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// getReceiverIdent extracts the identifier from a receiver expression.
func getReceiverIdent(expr ast.Expr) *ast.Ident {
	switch x := expr.(type) {
	case *ast.Ident:
		return x
	case *ast.SelectorExpr:
		// For chained calls like imp.workspaceService.Get()
		// Return the field selector (workspaceService)
		return x.Sel
	case *ast.CallExpr:
		// For chained calls like service.TX(tx).Get()
		// This is a TX-bound call, handled elsewhere
		return nil
	}
	return nil
}

// isWriterType checks if the type is a Writer type (which only has write methods).
func isWriterType(t types.Type) bool {
	typeStr := t.String()
	return strings.Contains(typeStr, "Writer")
}

// isSafeReceiverName checks if the receiver name is commonly safe (not a service).
func isSafeReceiverName(name string) bool {
	// Skip common safe variables and package names
	safeNames := map[string]bool{
		"ctx":         true,
		"context":     true,
		"tx":          true,
		"db":          true,
		"conn":        true,
		"row":         true,
		"rows":        true,
		"stmt":        true,
		"err":         true,
		"result":      true,
		"results":     true,
		"resp":        true,
		"req":         true,
		"request":     true,
		"response":    true,
		"file":        true,
		"buf":         true,
		"buffer":      true,
		"data":        true,
		"bytes":       true,
		"str":         true,
		"string":      true,
		"slice":       true,
		"map":         true,
		"arr":         true,
		"array":       true,
		"list":        true,
		"item":        true,
		"items":       true,
		"val":         true,
		"value":       true,
		"key":         true,
		"keys":        true,
		"id":          true,
		"ids":         true,
		"name":        true,
		"path":        true,
		"url":         true,
		"uri":         true,
		"body":        true,
		"header":      true,
		"headers":     true,
		"query":       true,
		"params":      true,
		"args":        true,
		"opts":        true,
		"options":     true,
		"config":      true,
		"cfg":         true,
		"settings":    true,
		"env":         true,
		"log":         true,
		"logger":      true,
		"client":      true,
		"server":      true,
		"writer":      true,
		"reader":      true,
		"io":          true,
		"os":          true,
		"fmt":         true,
		"json":        true,
		"xml":         true,
		"yaml":        true,
		"toml":        true,
		"proto":       true,
		"pb":          true,
		"grpc":        true,
		"http":        true,
		"rpc":         true,
		"mwauth":      true, // Auth middleware (reads from context)
		"ioworkspace": true, // Workspace IO helpers (not DB reads)
		"idwrap":      true, // ID wrapper helpers
		"patch":       true, // Patch helpers
		"converter":   true, // Converter helpers
		"model":       true, // Model helpers
		"util":        true, // Utility helpers
		"helper":      true, // Helper packages
		"helpers":     true, // Helper packages
		"time":        true, // Time package
		"strings":     true, // Strings package
		"reflect":     true, // Reflect package
		"sync":        true, // Sync package
		"errors":      true, // Errors package
		"rand":        true, // Random package
		"math":        true, // Math package
		"sort":        true, // Sort package
		"slices":      true, // Slices package
		"maps":        true, // Maps package
	}
	return safeNames[name]
}
