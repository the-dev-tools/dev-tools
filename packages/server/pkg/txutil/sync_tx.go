package txutil

import (
	"context"
	"database/sql"
)

// SyncTxInsert wraps a SQL transaction and tracks items to publish sync events
// after successful commit. This ensures sync events are never forgotten.
//
// Usage:
//
//	tx, _ := db.BeginTx(ctx, nil)
//	syncTx := txutil.NewInsertTx[mhttp.HTTP](tx)
//	defer devtoolsdb.TxnRollback(tx)
//
//	for _, item := range items {
//	    service.Create(ctx, item)
//	    syncTx.Track(item)
//	}
//
//	err := syncTx.CommitAndPublish(ctx, publishInsertEvent)
type SyncTxInsert[T any] struct {
	tx      *sql.Tx
	tracked []T
}

// NewInsertTx creates a new transaction wrapper for insert operations
func NewInsertTx[T any](tx *sql.Tx) *SyncTxInsert[T] {
	return &SyncTxInsert[T]{
		tx:      tx,
		tracked: make([]T, 0),
	}
}

// Track adds an item to be published after successful commit
func (s *SyncTxInsert[T]) Track(item T) {
	s.tracked = append(s.tracked, item)
}

// CommitAndPublish commits the transaction and publishes all tracked items.
// If commit fails, no events are published.
func (s *SyncTxInsert[T]) CommitAndPublish(
	ctx context.Context,
	publishFn func(T),
) error {
	if err := s.tx.Commit(); err != nil {
		return err
	}

	// Only publish if commit succeeded
	for _, item := range s.tracked {
		publishFn(item)
	}

	return nil
}

// UpdateEvent represents an update operation with the updated item and its delta patch
type UpdateEvent[T any, P any] struct {
	Item  T
	Patch P
}

// SyncTxUpdate wraps a SQL transaction and tracks update events to publish
// after successful commit.
//
// Usage:
//
//	tx, _ := db.BeginTx(ctx, nil)
//	syncTx := txutil.NewUpdateTx[mhttp.HTTP, patch.HTTPDeltaPatch](tx)
//	defer devtoolsdb.TxnRollback(tx)
//
//	for _, update := range updates {
//	    service.Update(ctx, update.Item)
//	    syncTx.Track(update.Item, update.Patch)
//	}
//
//	err := syncTx.CommitAndPublish(ctx, publishUpdateEvent)
type SyncTxUpdate[T any, P any] struct {
	tx      *sql.Tx
	tracked []UpdateEvent[T, P]
}

// NewUpdateTx creates a new transaction wrapper for update operations
func NewUpdateTx[T any, P any](tx *sql.Tx) *SyncTxUpdate[T, P] {
	return &SyncTxUpdate[T, P]{
		tx:      tx,
		tracked: make([]UpdateEvent[T, P], 0),
	}
}

// Track adds an update event (item + patch) to be published after successful commit
func (s *SyncTxUpdate[T, P]) Track(item T, patch P) {
	s.tracked = append(s.tracked, UpdateEvent[T, P]{
		Item:  item,
		Patch: patch,
	})
}

// CommitAndPublish commits the transaction and publishes all tracked update events.
// If commit fails, no events are published.
// The publishFn receives both the item and its patch.
func (s *SyncTxUpdate[T, P]) CommitAndPublish(
	ctx context.Context,
	publishFn func(T, P),
) error {
	if err := s.tx.Commit(); err != nil {
		return err
	}

	// Only publish if commit succeeded
	for _, event := range s.tracked {
		publishFn(event.Item, event.Patch)
	}

	return nil
}

// DeleteEvent represents a delete operation with the ID and workspace context
type DeleteEvent[ID any] struct {
	ID          ID
	WorkspaceID ID
	IsDelta     bool
}

// SyncTxDelete wraps a SQL transaction and tracks delete events to publish
// after successful commit.
//
// Usage:
//
//	tx, _ := db.BeginTx(ctx, nil)
//	syncTx := txutil.NewDeleteTx[idwrap.IDWrap](tx)
//	defer devtoolsdb.TxnRollback(tx)
//
//	for _, id := range ids {
//	    service.Delete(ctx, id)
//	    syncTx.Track(id, workspaceID, isDelta)
//	}
//
//	err := syncTx.CommitAndPublish(ctx, publishDeleteEvent)
type SyncTxDelete[ID any] struct {
	tx      *sql.Tx
	tracked []DeleteEvent[ID]
}

// NewDeleteTx creates a new transaction wrapper for delete operations
func NewDeleteTx[ID any](tx *sql.Tx) *SyncTxDelete[ID] {
	return &SyncTxDelete[ID]{
		tx:      tx,
		tracked: make([]DeleteEvent[ID], 0),
	}
}

// Track adds a delete event to be published after successful commit
func (s *SyncTxDelete[ID]) Track(id ID, workspaceID ID, isDelta bool) {
	s.tracked = append(s.tracked, DeleteEvent[ID]{
		ID:          id,
		WorkspaceID: workspaceID,
		IsDelta:     isDelta,
	})
}

// CommitAndPublish commits the transaction and publishes all tracked delete events.
// If commit fails, no events are published.
// The publishFn receives the ID, workspaceID, and isDelta flag.
func (s *SyncTxDelete[ID]) CommitAndPublish(
	ctx context.Context,
	publishFn func(ID, ID, bool),
) error {
	if err := s.tx.Commit(); err != nil {
		return err
	}

	// Only publish if commit succeeded
	for _, event := range s.tracked {
		publishFn(event.ID, event.WorkspaceID, event.IsDelta)
	}

	return nil
}
