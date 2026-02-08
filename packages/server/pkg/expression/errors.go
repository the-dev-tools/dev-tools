//nolint:revive // exported
package expression

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrNilEnv          = errors.New("cannot evaluate on nil UnifiedEnv")
	ErrKeyNotFound     = errors.New("key not found")
	ErrEmptyPath       = errors.New("empty path")
	ErrEmptyExpression = errors.New("empty expression")
)

// ExpressionError represents a structured error from expression evaluation.
type ExpressionError struct {
	Expression string // The expression that failed
	Phase      string // "compile", "run", or "resolve"
	Cause      error  // The underlying error
}

func (e *ExpressionError) Error() string {
	return fmt.Sprintf("expression %q failed during %s: %v", e.Expression, e.Phase, e.Cause)
}

func (e *ExpressionError) Unwrap() error {
	return e.Cause
}

// NewCompileError creates an error for compilation failures.
func NewCompileError(expr string, cause error) error {
	return &ExpressionError{
		Expression: expr,
		Phase:      "compile",
		Cause:      cause,
	}
}

// NewRunError creates an error for runtime evaluation failures.
func NewRunError(expr string, cause error) error {
	return &ExpressionError{
		Expression: expr,
		Phase:      "run",
		Cause:      cause,
	}
}

// NewResolveError creates an error for variable resolution failures.
func NewResolveError(path string, cause error) error {
	return &ExpressionError{
		Expression: path,
		Phase:      "resolve",
		Cause:      cause,
	}
}

// InterpolationError represents an error during {{ }} interpolation.
type InterpolationError struct {
	Input   string // The original input string
	VarRef  string // The variable reference that failed
	Cause   error  // The underlying error
}

func (e *InterpolationError) Error() string {
	return fmt.Sprintf("interpolation failed for '%s': %v", e.VarRef, e.Cause)
}

func (e *InterpolationError) Unwrap() error {
	return e.Cause
}

// FileReferenceError represents an error when reading a #file: reference.
type FileReferenceError struct {
	Path  string
	Cause error
}

func (e *FileReferenceError) Error() string {
	return fmt.Sprintf("failed to read file '%s': %v", e.Path, e.Cause)
}

func (e *FileReferenceError) Unwrap() error {
	return e.Cause
}

// EnvReferenceError represents an error when reading a #env: reference.
type EnvReferenceError struct {
	VarName string
	Cause   error
}

func (e *EnvReferenceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("environment variable '%s': %v", e.VarName, e.Cause)
	}
	return fmt.Sprintf("environment variable '%s' not found", e.VarName)
}

func (e *EnvReferenceError) Unwrap() error {
	return e.Cause
}

// SecretReferenceError represents an error when resolving a cloud secret reference.
type SecretReferenceError struct {
	Provider string // "gcp", "aws", "azure"
	Ref      string // The resource path
	Fragment string // Optional JSON fragment key
	Cause    error
}

func (e *SecretReferenceError) Error() string {
	loc := e.Ref
	if e.Fragment != "" {
		loc += "#" + e.Fragment
	}
	if e.Cause != nil {
		return fmt.Sprintf("secret reference '%s:%s' failed: %v", e.Provider, loc, e.Cause)
	}
	return fmt.Sprintf("secret reference '%s:%s' failed", e.Provider, loc)
}

func (e *SecretReferenceError) Unwrap() error {
	return e.Cause
}
