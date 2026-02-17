// Package nopermskip provides a Go analyzer that ensures all Connect RPC handlers
// include a permission check. It prevents accidentally shipping unprotected endpoints.
package nopermskip

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer ensures all Connect RPC handlers include a permission check.
var Analyzer = &analysis.Analyzer{
	Name:     "nopermskip",
	Doc:      "Ensures all Connect RPC handlers include a permission check.",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// skippedPackages are package path patterns where permission checks are not required.
var skippedPackages = []string{
	"/rhealth", // HealthCheck is intentionally public
}

func run(pass *analysis.Pass) (interface{}, error) {
	pkgPath := pass.Pkg.Path()

	// Skip test packages
	if strings.HasSuffix(pkgPath, "_test") {
		return nil, nil
	}

	// Skip packages where permission checks are not required
	for _, skip := range skippedPackages {
		if strings.Contains(pkgPath, skip) {
			return nil, nil
		}
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)

		// Skip unexported functions
		if !fn.Name.IsExported() {
			return
		}

		// Skip non-methods (no receiver)
		if fn.Recv == nil {
			return
		}

		// Skip functions declared in test files
		pos := pass.Fset.Position(fn.Pos())
		if strings.HasSuffix(pos.Filename, "_test.go") {
			return
		}

		// Check if this is an RPC handler by inspecting parameter types
		if !isRPCHandler(pass, fn) {
			return
		}

		// Check for //nolint:nopermskip directive
		if hasNolintDirective(pass, fn) {
			return
		}

		// Walk the function body looking for permission gate calls
		if hasPermissionGate(fn.Body) {
			return
		}

		pass.Reportf(fn.Name.Pos(),
			"RPC handler %s missing permission check; add a permcheck.Check*Access call or //nolint:nopermskip",
			fn.Name.Name)
	})

	return nil, nil
}

// isRPCHandler checks if the function has a parameter whose type contains "connect.Request[".
func isRPCHandler(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	for _, field := range fn.Type.Params.List {
		tv, ok := pass.TypesInfo.Types[field.Type]
		if !ok {
			continue
		}
		typeStr := tv.Type.String()
		if strings.Contains(typeStr, "connect.Request[") {
			return true
		}
	}
	return false
}

// hasNolintDirective checks if the function declaration has a //nolint:nopermskip comment.
func hasNolintDirective(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	// Check doc comments (/** */ or // above the function)
	if fn.Doc != nil {
		for _, comment := range fn.Doc.List {
			if strings.Contains(comment.Text, "nolint:nopermskip") {
				return true
			}
		}
	}

	// Check line comments on the func declaration line
	for _, cg := range pass.Files {
		for _, commentGroup := range cg.Comments {
			for _, comment := range commentGroup.List {
				commentPos := pass.Fset.Position(comment.Pos())
				funcPos := pass.Fset.Position(fn.Pos())
				if commentPos.Line == funcPos.Line && strings.Contains(comment.Text, "nolint:nopermskip") {
					return true
				}
			}
		}
	}

	return false
}

// hasPermissionGate walks the function body and checks if any call expression
// matches a known permission gate pattern.
func hasPermissionGate(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}

	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		name := callName(call)
		if name == "" {
			return true
		}

		if isPermissionGateName(name) {
			found = true
			return false
		}

		return true
	})

	return found
}

// callName extracts the function/method name from a call expression.
func callName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fn.Sel.Name
	case *ast.Ident:
		return fn.Name
	}
	return ""
}

// isPermissionGateName returns true if the function name matches a known permission gate pattern.
func isPermissionGateName(name string) bool {
	// Contains "Access" — checkWorkspaceReadAccess, CheckWorkspaceDeleteAccess, etc.
	if strings.Contains(name, "Access") {
		return true
	}

	// Starts with "GetContextUser" — mwauth.GetContextUserID
	if strings.HasPrefix(name, "GetContextUser") {
		return true
	}

	// Starts with "CheckOwner" — mwauth.CheckOwnerWorkspace
	if strings.HasPrefix(name, "CheckOwner") {
		return true
	}

	// Starts with "list" and contains "User" or "Accessible"
	if strings.HasPrefix(name, "list") {
		if strings.Contains(name, "User") || strings.Contains(name, "Accessible") {
			return true
		}
	}

	// Starts with "stream" and contains "Sync"
	if strings.HasPrefix(name, "stream") && strings.Contains(name, "Sync") {
		return true
	}

	return false
}
