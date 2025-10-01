package migrate

import (
	"context"
	"database/sql"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
)

func TestRegisterAndListOrdersByID(t *testing.T) {
	ResetForTesting()
	t.Cleanup(ResetForTesting)

	idA := newID()
	idB := newID()
	idC := newID()

	toRegister := []string{idC, idA, idB}

	for _, id := range toRegister {
		if err := Register(Migration{ID: id, Checksum: "sum", Apply: funcStub}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}

	migrations := List()
	if len(migrations) != len(toRegister) {
		t.Fatalf("expected %d migrations, got %d", len(toRegister), len(migrations))
	}

	for i := 1; i < len(migrations); i++ {
		if migrations[i-1].ID > migrations[i].ID {
			t.Fatalf("migrations not ordered: %s before %s", migrations[i-1].ID, migrations[i].ID)
		}
	}
}

func TestRegisterRejectsDuplicateID(t *testing.T) {
	ResetForTesting()
	t.Cleanup(ResetForTesting)

	id := newID()
	if err := Register(Migration{ID: id, Checksum: "sum", Apply: funcStub}); err != nil {
		t.Fatalf("register %s once: %v", id, err)
	}

	if err := Register(Migration{ID: id, Checksum: "sum", Apply: funcStub}); err == nil {
		t.Fatalf("expected duplicate registration error")
	}
}

func TestRegisterValidatesIDAndApply(t *testing.T) {
	ResetForTesting()
	t.Cleanup(ResetForTesting)

	tests := []struct {
		name string
		mig  Migration
	}{
		{
			name: "missing id",
			mig:  Migration{Apply: funcStub, Checksum: "sum"},
		},
		{
			name: "invalid ulid",
			mig:  Migration{ID: "not-ulid", Checksum: "sum", Apply: funcStub},
		},
		{
			name: "missing apply",
			mig:  Migration{ID: newID(), Checksum: "sum"},
		},
		{
			name: "missing checksum",
			mig:  Migration{ID: newID(), Apply: funcStub},
		},
		{
			name: "cursor missing load",
			mig: Migration{
				ID:       newID(),
				Checksum: "sum",
				Apply:    funcStub,
				Cursor: &CursorFuncs{
					Save: funcStubCursorSave,
				},
			},
		},
		{
			name: "cursor missing save",
			mig: Migration{
				ID:       newID(),
				Checksum: "sum",
				Apply:    funcStub,
				Cursor: &CursorFuncs{
					Load: funcStubCursorLoad,
				},
			},
		},
		{
			name: "negative chunk size",
			mig: Migration{
				ID:        newID(),
				Checksum:  "sum",
				Apply:     funcStub,
				ChunkSize: -1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Register(tt.mig); err == nil {
				t.Fatalf("expected error for case %s", tt.name)
			}
		})
	}
}

func funcStub(_ context.Context, _ *sql.Tx) error {
	return nil
}

func funcStubCursorLoad(context.Context, *sql.Tx) (CursorState, error) {
	return nil, nil
}

func funcStubCursorSave(context.Context, *sql.Tx, CursorState) error {
	return nil
}

func newID() string {
	return idwrap.NewNow().String()
}
