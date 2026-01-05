package txread

import (
	"context"
	"database/sql"
)

// UserService represents a typical service with read methods
type UserService struct {
	db *sql.DB
}

// TX returns a TX-bound service
func (s *UserService) TX(tx *sql.Tx) *UserServiceTX {
	return &UserServiceTX{tx: tx}
}

// Get reads a user by ID - this is a read operation
func (s *UserService) Get(ctx context.Context, id int) (string, error) {
	return "", nil
}

// GetByName reads a user by name - this is a read operation
func (s *UserService) GetByName(ctx context.Context, name string) (string, error) {
	return "", nil
}

// List reads all users - this is a read operation
func (s *UserService) List(ctx context.Context) ([]string, error) {
	return nil, nil
}

// Create is a write operation - should not be flagged
func (s *UserService) Create(ctx context.Context, name string) error {
	return nil
}

// UserServiceTX is a TX-bound user service
type UserServiceTX struct {
	tx *sql.Tx
}

// Get reads a user by ID using the transaction
func (s *UserServiceTX) Get(ctx context.Context, id int) (string, error) {
	return "", nil
}

// Create is a write operation
func (s *UserServiceTX) Create(ctx context.Context, name string) error {
	return nil
}

// WorkspaceReader is a reader service
type WorkspaceReader struct {
	db *sql.DB
}

// TX returns a TX-bound reader
func (r *WorkspaceReader) TX(tx *sql.Tx) *WorkspaceReaderTX {
	return &WorkspaceReaderTX{tx: tx}
}

// Get reads a workspace
func (r *WorkspaceReader) Get(ctx context.Context, id int) (string, error) {
	return "", nil
}

// WorkspaceReaderTX is a TX-bound reader
type WorkspaceReaderTX struct {
	tx *sql.Tx
}

// Get reads a workspace using the transaction
func (r *WorkspaceReaderTX) Get(ctx context.Context, id int) (string, error) {
	return "", nil
}

// WorkspaceWriter is a writer service - reads on writers should not be flagged
type WorkspaceWriter struct {
	tx *sql.Tx
}

// Create is a write operation
func (w *WorkspaceWriter) Create(ctx context.Context, name string) error {
	return nil
}

// BadReadInsideTx demonstrates the deadlock pattern - read inside TX
func BadReadInsideTx(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// BAD: Non-TX-bound read inside transaction
	_, err = userService.Get(ctx, 1) // want "non-TX-bound read Get\\(\\) inside transaction"
	if err != nil {
		return err
	}

	return tx.Commit()
}

// BadListInsideTx demonstrates the deadlock pattern with List
func BadListInsideTx(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// BAD: Non-TX-bound List inside transaction
	_, err = userService.List(ctx) // want "non-TX-bound read List\\(\\) inside transaction"
	if err != nil {
		return err
	}

	return tx.Commit()
}

// BadGetByNameInsideTx demonstrates the deadlock pattern with GetBy*
func BadGetByNameInsideTx(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// BAD: Non-TX-bound GetByName inside transaction
	_, err = userService.GetByName(ctx, "test") // want "non-TX-bound read GetByName\\(\\) inside transaction"
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GoodTXBoundRead demonstrates the correct pattern with TX-bound service
func GoodTXBoundRead(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// GOOD: TX-bound service
	txService := userService.TX(tx)
	_, err = txService.Get(ctx, 1)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GoodReadBeforeTx demonstrates the correct pattern - read before TX
func GoodReadBeforeTx(ctx context.Context, db *sql.DB, userService *UserService) error {
	// GOOD: Read before transaction
	user, err := userService.Get(ctx, 1)
	if err != nil {
		return err
	}
	_ = user

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Only writes inside transaction
	txService := userService.TX(tx)
	err = txService.Create(ctx, "new-user")
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GoodReadAfterCommit demonstrates reads after commit
func GoodReadAfterCommit(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Writes inside transaction
	txService := userService.TX(tx)
	err = txService.Create(ctx, "new-user")
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// GOOD: Read after commit
	_, err = userService.Get(ctx, 1)
	return err
}

// GoodWriteInsideTx demonstrates writes inside TX (should not be flagged)
func GoodWriteInsideTx(ctx context.Context, db *sql.DB, userService *UserService) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// GOOD: Create is a write operation, not a read
	err = userService.Create(ctx, "new-user")
	if err != nil {
		return err
	}

	return tx.Commit()
}

// BadReaderServiceInsideTx demonstrates the pattern with Reader types
func BadReaderServiceInsideTx(ctx context.Context, db *sql.DB, workspaceReader *WorkspaceReader) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// BAD: Non-TX-bound reader inside transaction
	_, err = workspaceReader.Get(ctx, 1) // want "non-TX-bound read Get\\(\\) inside transaction"
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GoodReaderWithTX demonstrates the correct pattern with Reader types
func GoodReaderWithTX(ctx context.Context, db *sql.DB, workspaceReader *WorkspaceReader) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// GOOD: TX-bound reader
	txReader := workspaceReader.TX(tx)
	_, err = txReader.Get(ctx, 1)
	if err != nil {
		return err
	}

	return tx.Commit()
}
