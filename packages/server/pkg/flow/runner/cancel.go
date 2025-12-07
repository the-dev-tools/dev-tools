//nolint:revive // exported
package runner

import (
    "context"
    "errors"
)

// ErrFlowCanceledByThrow marks an intentional cancellation triggered by a node (e.g., via a user throw).
// When a loop node propagates this error, the runner should mark the loop as CANCELED, not FAILURE.
var ErrFlowCanceledByThrow = errors.New("flow canceled by throw")

// IsCancellationError returns true if the error represents a cancellation (explicit throw or context cancellation).
func IsCancellationError(err error) bool {
    if err == nil {
        return false
    }
    return errors.Is(err, ErrFlowCanceledByThrow) || errors.Is(err, context.Canceled)
}

