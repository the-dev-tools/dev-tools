package migrate

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Migration)

	// ErrDuplicateID is returned when registering the same migration twice.
	ErrDuplicateID = errors.New("migrate: duplicate migration id")
)

// Register adds a migration to the in-process registry.
// It enforces ULID ordering, required hooks, and cursor integrity.
func Register(m Migration) error {
	if err := validateMigration(m); err != nil {
		return err
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[m.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateID, m.ID)
	}

	registry[m.ID] = m
	return nil
}

// List returns the registered migrations ordered by ID.
func List() []Migration {
	registryMu.RLock()
	defer registryMu.RUnlock()

	out := make([]Migration, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}

// ResetForTesting clears the registry. Intended for use in tests only.
func ResetForTesting() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]Migration)
}

func validateMigration(m Migration) error {
	if m.ID == "" {
		return errors.New("migrate: migration id must not be empty")
	}

	if _, err := idwrap.NewText(m.ID); err != nil {
		return fmt.Errorf("migrate: migration id must be ULID string: %w", err)
	}

	if m.Checksum == "" {
		return errors.New("migrate: checksum must not be empty")
	}

	if m.Apply == nil {
		return errors.New("migrate: Apply hook must be provided")
	}

	if m.ChunkSize < 0 {
		return errors.New("migrate: chunk size cannot be negative")
	}

	if m.Cursor != nil {
		if m.Cursor.Load == nil || m.Cursor.Save == nil {
			return errors.New("migrate: cursor Load and Save must both be provided when cursor helpers are configured")
		}
	}

	return nil
}
