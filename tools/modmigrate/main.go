// Package main provides a tool to migrate Go module paths from non-idiomatic
// paths (e.g., "the-dev-tools/server") to idiomatic GitHub paths
// (e.g., "github.com/the-dev-tools/dev-tools/packages/server").
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// pathMap defines the mapping from old module paths to new idiomatic paths.
var pathMap = map[string]string{
	"the-dev-tools/cli":      "github.com/the-dev-tools/dev-tools/apps/cli",
	"the-dev-tools/db":       "github.com/the-dev-tools/dev-tools/packages/db",
	"the-dev-tools/server":   "github.com/the-dev-tools/dev-tools/packages/server",
	"the-dev-tools/spec":     "github.com/the-dev-tools/dev-tools/packages/spec",
	"the-dev-tools/norawsql": "github.com/the-dev-tools/dev-tools/tools/norawsql",
	"the-dev-tools/notxread": "github.com/the-dev-tools/dev-tools/tools/notxread",
	"benchmark":              "github.com/the-dev-tools/dev-tools/tools/benchmark",
	"tools":                  "github.com/the-dev-tools/dev-tools/tools/go-tool",
}

var (
	rootDir = flag.String("root", "", "Root directory of the project (defaults to current directory)")
	dryRun  = flag.Bool("dry-run", false, "Print changes without applying them")
	verbose = flag.Bool("verbose", false, "Print verbose output")
)

func main() {
	flag.Parse()

	root := *rootDir
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Make root absolute
	root, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting absolute path: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Migrating Go module paths in: %s\n", root)
	if *dryRun {
		fmt.Println("DRY RUN - no changes will be made")
	}

	// Process go.mod files
	goModFiles, err := findGoModFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding go.mod files: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d go.mod files\n", len(goModFiles))
	for _, f := range goModFiles {
		if err := processGoMod(f); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", f, err)
			os.Exit(1)
		}
	}

	// Process Go source files
	goFiles, err := findGoFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding .go files: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d .go files\n", len(goFiles))
	modified := 0
	for _, f := range goFiles {
		changed, err := processGoFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", f, err)
			os.Exit(1)
		}
		if changed {
			modified++
		}
	}

	fmt.Printf("\nMigration complete!\n")
	fmt.Printf("  - go.mod files updated: %d\n", len(goModFiles))
	fmt.Printf("  - .go files modified: %d\n", modified)
}

func findGoModFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip vendor directories
		if info.IsDir() && info.Name() == "vendor" {
			return filepath.SkipDir
		}
		// Skip node_modules
		if info.IsDir() && info.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if info.Name() == "go.mod" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func findGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip vendor directories
		if info.IsDir() && info.Name() == "vendor" {
			return filepath.SkipDir
		}
		// Skip node_modules
		if info.IsDir() && info.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if strings.HasSuffix(info.Name(), ".go") && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func processGoMod(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	changed := false

	// Update module path
	if newPath, ok := pathMap[f.Module.Mod.Path]; ok {
		if *verbose {
			fmt.Printf("  %s: module %s -> %s\n", path, f.Module.Mod.Path, newPath)
		}
		if err := f.AddModuleStmt(newPath); err != nil {
			return fmt.Errorf("updating module path: %w", err)
		}
		changed = true
	}

	// Update require statements
	for _, req := range f.Require {
		if newPath := replaceImportPath(req.Mod.Path); newPath != req.Mod.Path {
			if *verbose {
				fmt.Printf("  %s: require %s -> %s\n", path, req.Mod.Path, newPath)
			}
			if err := f.AddRequire(newPath, req.Mod.Version); err != nil {
				return fmt.Errorf("updating require: %w", err)
			}
			if err := f.DropRequire(req.Mod.Path); err != nil {
				return fmt.Errorf("dropping old require: %w", err)
			}
			changed = true
		}
	}

	// Update replace statements
	for _, rep := range f.Replace {
		oldPathChanged := false
		newOldPath := rep.Old.Path
		if np := replaceImportPath(rep.Old.Path); np != rep.Old.Path {
			newOldPath = np
			oldPathChanged = true
		}

		newNewPath := rep.New.Path
		newPathChanged := false
		if np := replaceImportPath(rep.New.Path); np != rep.New.Path {
			newNewPath = np
			newPathChanged = true
		}

		if oldPathChanged || newPathChanged {
			if *verbose {
				fmt.Printf("  %s: replace %s => %s -> %s => %s\n",
					path, rep.Old.Path, rep.New.Path, newOldPath, newNewPath)
			}
			// Drop old replace first
			if err := f.DropReplace(rep.Old.Path, rep.Old.Version); err != nil {
				return fmt.Errorf("dropping old replace: %w", err)
			}
			// Add new replace
			if err := f.AddReplace(newOldPath, rep.Old.Version, newNewPath, rep.New.Version); err != nil {
				return fmt.Errorf("adding new replace: %w", err)
			}
			changed = true
		}
	}

	// Update tool statements (Go 1.24+ feature)
	// modfile doesn't have direct support for tool directives, so we handle them via raw syntax
	for _, stmt := range f.Syntax.Stmt {
		if line, ok := stmt.(*modfile.Line); ok {
			if len(line.Token) >= 2 && line.Token[0] == "tool" {
				toolPath := line.Token[1]
				if newPath := replaceImportPath(toolPath); newPath != toolPath {
					if *verbose {
						fmt.Printf("  %s: tool %s -> %s\n", path, toolPath, newPath)
					}
					line.Token[1] = newPath
					changed = true
				}
			}
		}
		// Handle tool blocks
		if block, ok := stmt.(*modfile.LineBlock); ok {
			if len(block.Token) == 1 && block.Token[0] == "tool" {
				for _, line := range block.Line {
					if len(line.Token) >= 1 {
						toolPath := line.Token[0]
						if newPath := replaceImportPath(toolPath); newPath != toolPath {
							if *verbose {
								fmt.Printf("  %s: tool %s -> %s\n", path, toolPath, newPath)
							}
							line.Token[0] = newPath
							changed = true
						}
					}
				}
			}
		}
	}

	if !changed {
		if *verbose {
			fmt.Printf("  %s: no changes needed\n", path)
		}
		return nil
	}

	// Format and write back
	newData, err := f.Format()
	if err != nil {
		return fmt.Errorf("formatting go.mod: %w", err)
	}

	if *dryRun {
		fmt.Printf("  Would update: %s\n", path)
		return nil
	}

	if err := os.WriteFile(path, newData, 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	fmt.Printf("  Updated: %s\n", path)
	return nil
}

func processGoFile(path string) (bool, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parsing file: %w", err)
	}

	changed := false

	// Update imports
	for _, imp := range node.Imports {
		if imp.Path == nil {
			continue
		}
		// Remove quotes from path value
		oldPath := strings.Trim(imp.Path.Value, `"`)
		newPath := replaceImportPath(oldPath)

		if newPath != oldPath {
			if *verbose {
				fmt.Printf("  %s: import %q -> %q\n", path, oldPath, newPath)
			}
			imp.Path.Value = fmt.Sprintf(`"%s"`, newPath)
			changed = true
		}
	}

	if !changed {
		return false, nil
	}

	// Format the modified AST
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return false, fmt.Errorf("formatting: %w", err)
	}

	if *dryRun {
		fmt.Printf("  Would update: %s\n", path)
		return true, nil
	}

	// Write back
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return false, fmt.Errorf("writing file: %w", err)
	}

	if *verbose {
		fmt.Printf("  Updated: %s\n", path)
	}

	return true, nil
}

// replaceImportPath checks if the given import path matches any of our old
// module paths and returns the new path. If no match, returns the original.
func replaceImportPath(importPath string) string {
	// Check for exact matches first
	if newPath, ok := pathMap[importPath]; ok {
		return newPath
	}

	// Check for prefix matches (subpackages)
	for oldPrefix, newPrefix := range pathMap {
		if strings.HasPrefix(importPath, oldPrefix+"/") {
			suffix := strings.TrimPrefix(importPath, oldPrefix)
			return newPrefix + suffix
		}
	}

	return importPath
}

// Utility function for testing the path replacement
func init() {
	// Verify all mappings are valid
	for old, new := range pathMap {
		if old == "" || new == "" {
			panic(fmt.Sprintf("invalid path mapping: %q -> %q", old, new))
		}
	}
}
