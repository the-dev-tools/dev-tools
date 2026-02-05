// Package notxread provides a Go analyzer that detects read operations on non-TX-bound
// services inside SQLite write transactions. This prevents deadlocks in file-based SQLite.
//
// SQLite Deadlock Scenario:
//
//	Connection Pool (s.DB)
//	    │
//	    ├── Connection 1: BeginTx() → holds EXCLUSIVE lock
//	    │       └── waiting for read to complete...
//	    │
//	    └── Connection 2: s.credReader.Get() → needs SHARED lock
//	            └── waiting for EXCLUSIVE lock to release...
//	    = DEADLOCK
//
// The correct pattern is either:
//  1. Read BEFORE starting the transaction (outside TX scope)
//  2. Use a TX-bound service via .TX(tx).Method() (same connection, no lock contention)
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
	"_test",       // Test files
	"/sqlc/",      // SQLC generated code
	"/migrate",    // Migration code
	"/migrations", // Migration files
	"/dbtest",     // DB test utilities
}

// servicePackagePatterns identify types from service packages that use DB connections.
var servicePackagePatterns = []string{
	"/service/",  // pkg/service/* packages
	"/shttp/",    // HTTP service
	"/sflow/",    // Flow service
	"/suser/",    // User service
	"/senv/",     // Environment service
	"/sfile/",    // File service
	"/stag/",     // Tag service
	"/sworkspace/", // Workspace service
	"/scredential/", // Credential service
}

// txState tracks transaction state within a function.
type txState struct {
	beginTxPos   token.Pos       // Position of BeginTx call
	commitPos    token.Pos       // Position of Commit call (0 if not found)
	dbReceiver   string          // The receiver on which DB.BeginTx was called (e.g., "s" from "s.DB.BeginTx")
	txVarName    string          // The variable name holding the transaction (e.g., "tx")
	txBoundVars  map[string]bool // Variables created from .TX(tx) calls
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

	// Analyze each function for TX patterns
	funcFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
	}

	insp.Preorder(funcFilter, func(n ast.Node) {
		// Skip test files by checking the file name
		// But don't skip testdata files (used for linter testing)
		pos := pass.Fset.Position(n.Pos())
		if strings.HasSuffix(pos.Filename, "_test.go") && !strings.Contains(pos.Filename, "testdata") {
			return
		}

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

	// First pass: find BeginTx, track DB source, TX variable, and Commit position
	ast.Inspect(body, func(n ast.Node) bool {
		// Track TX variable assignment: tx, err := s.DB.BeginTx(ctx, nil)
		if assign, ok := n.(*ast.AssignStmt); ok {
			trackBeginTxAssignment(assign, state)
			trackTXBoundAssignment(assign, state)
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

		// Track BeginTx and its DB source
		if methodName == "BeginTx" && state.beginTxPos == 0 {
			state.beginTxPos = call.Pos()
			state.dbReceiver = extractDBReceiver(sel.X)
		}

		// Track Commit (last one, not defer Rollback)
		if methodName == "Commit" && state.beginTxPos != 0 {
			state.commitPos = call.Pos()
		}

		return true
	})

	// If no BeginTx found, nothing to check
	if state.beginTxPos == 0 {
		return
	}

	// Second pass: find read calls that violate TX safety
	ast.Inspect(body, func(n ast.Node) bool {
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

// trackBeginTxAssignment tracks the TX variable from BeginTx assignment.
// Pattern: tx, err := s.DB.BeginTx(ctx, nil)
func trackBeginTxAssignment(assign *ast.AssignStmt, state *txState) {
	for i, rhs := range assign.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			continue
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		if sel.Sel.Name == "BeginTx" {
			// Get the TX variable name (first LHS)
			if i < len(assign.Lhs) {
				if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
					state.txVarName = ident.Name
				}
			}
		}
	}
}

// trackTXBoundAssignment tracks variables created from .TX(tx) or .WithTx(tx) calls.
// Pattern: serviceTx := s.service.TX(tx) or queriesTx := queries.WithTx(tx)
func trackTXBoundAssignment(assign *ast.AssignStmt, state *txState) {
	for i, rhs := range assign.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			continue
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		// Check if method name is "TX" (service pattern) or "WithTx" (sqlc pattern)
		methodName := sel.Sel.Name
		if methodName != "TX" && methodName != "WithTx" {
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

// extractDBReceiver extracts the root receiver from a DB access expression.
// For s.DB.BeginTx(), returns "s"
// For db.BeginTx(), returns "db"
func extractDBReceiver(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		// For s.DB, recurse to get "s"
		return extractDBReceiver(x.X)
	}
	return ""
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

	// Check if this is a chained TX call: service.TX(tx).Get()
	if isChainedTXCall(sel.X) {
		return
	}

	// Get receiver info
	receiverInfo := analyzeReceiver(pass, sel.X)
	if receiverInfo == nil {
		return
	}

	// Skip if receiver is TX-bound variable
	if state.txBoundVars[receiverInfo.varName] {
		return
	}

	// Skip if not a service type (based on type info)
	if !receiverInfo.isServiceType {
		return
	}

	// Flag: reading from a non-TX-bound service inside a transaction
	// Two cases:
	// 1. Struct method pattern: s.DB.BeginTx() and s.credReader.Get() - same root receiver
	// 2. Local variable pattern: db.BeginTx() and userService.Get() - assume same DB pool
	//
	// If we can match root receivers and they differ, it might be safe (different DB).
	// But if we can't determine, or they match, flag it.
	shouldFlag := false

	if state.dbReceiver == "" {
		// Couldn't determine DB receiver, be conservative and flag
		shouldFlag = true
	} else if receiverInfo.rootReceiver == state.dbReceiver {
		// Same root receiver (e.g., both on "s")
		shouldFlag = true
	} else if receiverInfo.rootReceiver != "" && receiverInfo.rootReceiver != state.dbReceiver {
		// Different root receivers - might be different DBs, but still flag
		// because in this codebase, typically all services share the same DB
		// Local variables like userService, workspaceReader should still be flagged
		shouldFlag = true
	}

	if shouldFlag {
		// Suggest the right binding method based on type
		bindMethod := "TX(tx)"
		if receiverInfo.isSqlcQueries {
			bindMethod = "WithTx(tx)"
		}
		pass.Reportf(call.Pos(),
			"non-TX-bound read %s() inside transaction may cause SQLite deadlock; move read before BeginTx or use %s.%s.%s()",
			methodName, receiverInfo.varName, bindMethod, methodName)
	}
}

// receiverInfo holds analyzed information about a method receiver.
type receiverInfo struct {
	varName       string // The immediate variable name (e.g., "credReader" from "s.credReader")
	rootReceiver  string // The root receiver (e.g., "s" from "s.credReader")
	isServiceType bool   // Whether the type appears to be a service type
	isSqlcQueries bool   // Whether the type is sqlc generated Queries
}

// analyzeReceiver analyzes a receiver expression and returns info about it.
func analyzeReceiver(pass *analysis.Pass, expr ast.Expr) *receiverInfo {
	info := &receiverInfo{}

	switch x := expr.(type) {
	case *ast.Ident:
		info.varName = x.Name
		info.rootReceiver = x.Name
	case *ast.SelectorExpr:
		// For s.credReader, varName is "credReader", rootReceiver is "s"
		info.varName = x.Sel.Name
		info.rootReceiver = extractDBReceiver(x.X)
	case *ast.CallExpr:
		// Chained call like service.TX(tx).Get() - handled elsewhere
		return nil
	default:
		return nil
	}

	// Check if it's a service type using type information
	tv, ok := pass.TypesInfo.Types[expr]
	if ok {
		info.isServiceType = isServiceType(tv.Type)
		info.isSqlcQueries = isSqlcQueriesType(tv.Type)
	} else {
		// Fallback: check variable name patterns
		info.isServiceType = looksLikeServiceName(info.varName)
		info.isSqlcQueries = info.varName == "queries" || strings.HasSuffix(info.varName, "Queries")
	}

	return info
}

// isChainedTXCall checks if the expression is a chained TX call: service.TX(tx) or queries.WithTx(tx)
func isChainedTXCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check for both TX (service pattern) and WithTx (sqlc pattern)
	return sel.Sel.Name == "TX" || sel.Sel.Name == "WithTx"
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

// isServiceType checks if the type is a service type that uses DB connections.
func isServiceType(t types.Type) bool {
	if t == nil {
		return false
	}

	typeStr := t.String()

	// Check if type is from a service package
	for _, pattern := range servicePackagePatterns {
		if strings.Contains(typeStr, pattern) {
			return true
		}
	}

	// Check for sqlc generated Queries type
	// Pattern: *gen.Queries or *sqlc/gen.Queries
	if isSqlcQueriesType(t) {
		return true
	}

	// Check for common service type patterns by type name
	// Types ending in "Reader", "Service" are likely DB-connected services
	typeName := extractTypeName(t)
	if strings.HasSuffix(typeName, "Reader") {
		return true
	}
	if strings.HasSuffix(typeName, "Service") {
		return true
	}

	// Check for types that have a TX method - they're definitely services
	if hasTXMethod(t) {
		return true
	}

	// Check for types that have a WithTx method (sqlc pattern)
	if hasWithTxMethod(t) {
		return true
	}

	return false
}

// isSqlcQueriesType checks if the type is a sqlc generated Queries type.
func isSqlcQueriesType(t types.Type) bool {
	typeName := extractTypeName(t)
	if typeName != "Queries" {
		return false
	}

	// Check the package path contains sqlc/gen or similar
	typeStr := t.String()
	return strings.Contains(typeStr, "sqlc") || strings.Contains(typeStr, "/gen.")
}

// extractTypeName gets the simple type name from a types.Type.
func extractTypeName(t types.Type) string {
	// Handle pointer types
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	// Get named type
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}

	return ""
}

// hasTXMethod checks if the type has a TX method, indicating it's a service.
func hasTXMethod(t types.Type) bool {
	// Get the method set
	methods := types.NewMethodSet(t)

	for i := 0; i < methods.Len(); i++ {
		if methods.At(i).Obj().Name() == "TX" {
			return true
		}
	}

	return false
}

// hasWithTxMethod checks if the type has a WithTx method (sqlc Queries pattern).
func hasWithTxMethod(t types.Type) bool {
	methods := types.NewMethodSet(t)

	for i := 0; i < methods.Len(); i++ {
		if methods.At(i).Obj().Name() == "WithTx" {
			return true
		}
	}

	return false
}

// looksLikeServiceName checks if a variable name looks like a service.
// This is a fallback when type info is not available.
func looksLikeServiceName(name string) bool {
	// Service-like suffixes
	serviceSuffixes := []string{
		"Reader",
		"Service",
		"Repo",
		"Repository",
		"Store",
		"DAO",
	}

	for _, suffix := range serviceSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
		// Also check lowercase: credReader, fileService
		if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
			return true
		}
	}

	// Common service variable patterns
	servicePatterns := []string{
		"credReader",
		"fileService",
		"flowService",
		"httpService",
		"userService",
		"workspaceService",
		"envService",
		"tagService",
	}

	for _, pattern := range servicePatterns {
		if name == pattern {
			return true
		}
	}

	return false
}
