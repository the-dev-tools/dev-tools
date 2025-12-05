// Package norawsql provides a Go analyzer that detects raw SQL query method calls.
// It ensures all database operations go through sqlc generated code in packages/db/pkg.
package norawsql

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer detects raw SQL query/exec calls on *sql.DB, *sql.Tx, *sql.Conn, and *sql.Stmt.
var Analyzer = &analysis.Analyzer{
	Name:     "norawsql",
	Doc:      "Detects raw SQL query method calls. Use sqlc generated code instead.",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// forbiddenMethods are the database/sql methods that execute raw SQL.
var forbiddenMethods = map[string]bool{
	"Query":             true,
	"QueryContext":      true,
	"QueryRow":          true,
	"QueryRowContext":   true,
	"Exec":              true,
	"ExecContext":       true,
	"Prepare":           true,
	"PrepareContext":    true,
}

// sqlTypes are the database/sql types we want to check method calls on.
var sqlTypes = map[string]bool{
	"*database/sql.DB":   true,
	"*database/sql.Tx":   true,
	"*database/sql.Conn": true,
	"*database/sql.Stmt": true,
}

// allowedPackages are package path patterns where raw SQL is permitted.
var allowedPackages = []string{
	"packages/db",    // DB drivers and sqlc generated code
	"/db/",           // Alternate path format
	"/migrate",       // Migration runner needs raw SQL for DDL
	"/dbtest",        // DB test utilities
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Skip packages where raw SQL is allowed
	pkgPath := pass.Pkg.Path()
	for _, allowed := range allowedPackages {
		if strings.Contains(pkgPath, allowed) {
			return nil, nil
		}
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)

		// We're looking for method calls like db.Query(...) or tx.Exec(...)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}

		methodName := sel.Sel.Name
		if !forbiddenMethods[methodName] {
			return
		}

		// Get the type of the receiver (the thing before the dot)
		tv, ok := pass.TypesInfo.Types[sel.X]
		if !ok {
			return
		}

		typeStr := tv.Type.String()

		// Check if this is a call on a sql.DB, sql.Tx, sql.Conn, or sql.Stmt
		if sqlTypes[typeStr] {
			pass.Reportf(call.Pos(),
				"raw SQL method %s() is forbidden; use sqlc generated queries from packages/db/pkg/sqlc/gen instead",
				methodName)
			return
		}

		// Also check the underlying type for interfaces or type aliases
		if ptr, ok := tv.Type.(*types.Pointer); ok {
			if named, ok := ptr.Elem().(*types.Named); ok {
				obj := named.Obj()
				if obj != nil && obj.Pkg() != nil {
					fullType := "*" + obj.Pkg().Path() + "." + obj.Name()
					if sqlTypes[fullType] {
						pass.Reportf(call.Pos(),
							"raw SQL method %s() is forbidden; use sqlc generated queries from packages/db/pkg/sqlc/gen instead",
							methodName)
					}
				}
			}
		}
	})

	return nil, nil
}
