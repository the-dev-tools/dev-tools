package txread

import (
	"context"
	"database/sql"
)

// ========== COMPLEX PATTERNS THE LINTER CAN DETECT ==========

// StructFieldAccess tests detection on struct field receivers (like imp.workspaceService)
type ImportHandler struct {
	userService      *UserService
	workspaceReader  *WorkspaceReader
}

func (h *ImportHandler) BadStructFieldRead(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// BAD: Read via struct field inside TX
	_, err = h.userService.Get(ctx, 1) // want "non-TX-bound read Get\\(\\) inside transaction"
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (h *ImportHandler) GoodStructFieldWithTX(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// GOOD: TX-bound via struct field
	txService := h.userService.TX(tx)
	_, err = txService.Get(ctx, 1)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// MultipleReadsInsideTx tests multiple read calls
func MultipleReadsInsideTx(ctx context.Context, db *sql.DB, userService *UserService, workspaceReader *WorkspaceReader) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// BAD: Multiple non-TX reads
	_, err = userService.Get(ctx, 1) // want "non-TX-bound read Get\\(\\) inside transaction"
	if err != nil {
		return err
	}

	_, err = workspaceReader.Get(ctx, 1) // want "non-TX-bound read Get\\(\\) inside transaction"
	if err != nil {
		return err
	}

	_, err = userService.List(ctx) // want "non-TX-bound read List\\(\\) inside transaction"
	if err != nil {
		return err
	}

	return tx.Commit()
}

// MixedTXAndNonTX tests mix of TX-bound and non-TX-bound reads
func MixedTXAndNonTX(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// GOOD: TX-bound
	txService := userService.TX(tx)
	_, err = txService.Get(ctx, 1)
	if err != nil {
		return err
	}

	// BAD: Non-TX-bound after TX-bound call
	_, err = userService.Get(ctx, 2) // want "non-TX-bound read Get\\(\\) inside transaction"
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ConditionalRead tests reads inside conditionals
func ConditionalRead(ctx context.Context, db *sql.DB, userService *UserService, shouldRead bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if shouldRead {
		// BAD: Still inside TX even in conditional
		_, err = userService.Get(ctx, 1) // want "non-TX-bound read Get\\(\\) inside transaction"
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoopRead tests reads inside loops
func LoopRead(ctx context.Context, db *sql.DB, userService *UserService, ids []int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, id := range ids {
		// BAD: Read in loop inside TX
		_, err = userService.Get(ctx, id) // want "non-TX-bound read Get\\(\\) inside transaction"
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// EarlyReturnPath tests early return scenarios
func EarlyReturnPath(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// BAD: Read before early return check
	user, err := userService.Get(ctx, 1) // want "non-TX-bound read Get\\(\\) inside transaction"
	if err != nil {
		return err
	}

	if user == "" {
		return nil // Early return
	}

	return tx.Commit()
}

// ========== PATTERNS THE LINTER CORRECTLY ALLOWS ==========

// ChainedTXCall tests method chaining with TX
func ChainedTXCall(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// GOOD: Chained TX call - the result of .TX(tx) is immediately used
	// Note: This pattern returns nil from getReceiverIdent because it's a CallExpr
	_, err = userService.TX(tx).Get(ctx, 1)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ReadBeforeAndAfterTX tests reads before and after TX
func ReadBeforeAndAfterTX(ctx context.Context, db *sql.DB, userService *UserService) error {
	// GOOD: Read before TX
	user1, err := userService.Get(ctx, 1)
	if err != nil {
		return err
	}
	_ = user1

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Only writes inside TX
	txService := userService.TX(tx)
	err = txService.Create(ctx, "new")
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// GOOD: Read after commit
	user2, err := userService.Get(ctx, 2)
	if err != nil {
		return err
	}
	_ = user2

	return nil
}

// WriterTypeInTx tests that Writer types are not flagged
func WriterTypeInTx(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	writer := &WorkspaceWriter{tx: tx}
	// GOOD: Writer types are allowed (they only have write methods)
	err = writer.Create(ctx, "new")
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ========== LIMITATIONS (patterns the linter cannot fully handle) ==========

// Note: The following patterns demonstrate current limitations.
// These are documented here as edge cases the simple AST-based analyzer
// may not handle in all scenarios.

// ClosureCapture - closures that capture TX state
// The linter DOES detect reads in closures defined inside TX blocks
func ClosureCapture(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// The linter detects this because the closure is defined inside the TX block
	doRead := func() error {
		_, err := userService.Get(ctx, 1) // want "non-TX-bound read Get\\(\\) inside transaction"
		return err
	}
	_ = doRead

	return tx.Commit()
}

// InterproceduralFlow - TX state across function calls
// The linter does NOT track TX state across function boundaries
func InterproceduralFlow(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// LIMITATION: The linter doesn't know helper() is called inside TX
	helper(ctx, userService)

	return tx.Commit()
}

func helper(ctx context.Context, userService *UserService) {
	// This read is inside a TX (when called from InterproceduralFlow)
	// but the linter cannot detect this
	userService.Get(ctx, 1) // NOT detected (separate function)
}

// DynamicTXBound - TX-bound services passed as interface
// The linter tracks variable names, not types through interfaces
func DynamicTXBound(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// The analyzer tracks "txService" as TX-bound
	txService := userService.TX(tx)

	// If passed through interface, tracking is lost
	var svc interface{} = txService
	_ = svc
	// Can't check methods on interface{} without type info

	return tx.Commit()
}
