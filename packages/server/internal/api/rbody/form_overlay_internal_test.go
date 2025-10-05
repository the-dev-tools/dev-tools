package rbody

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	overcore "the-dev-tools/server/pkg/overlay/core"
)

type formStateUpsertCall struct {
	exampleID  idwrap.IDWrap
	originID   idwrap.IDWrap
	suppressed bool
	keyPtr     *string
	valPtr     *string
	descPtr    *string
	enPtr      *bool
}

type stubFormStateStore struct {
	states  map[string]overcore.StateRow
	upserts []formStateUpsertCall
}

func (s *stubFormStateStore) Get(_ context.Context, _ idwrap.IDWrap, origin idwrap.IDWrap) (overcore.StateRow, bool, error) {
	if s.states == nil {
		return overcore.StateRow{}, false, nil
	}
	st, ok := s.states[origin.String()]
	if !ok {
		return overcore.StateRow{}, false, nil
	}
	return st, true, nil
}

func (s *stubFormStateStore) Upsert(_ context.Context, ex, origin idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error {
	s.upserts = append(s.upserts, formStateUpsertCall{exampleID: ex, originID: origin, suppressed: suppressed, keyPtr: key, valPtr: val, descPtr: desc, enPtr: enabled})
	return nil
}

func TestSeedMissingFormStateFromDeltaCreatesOverrides(t *testing.T) {
	ctx := context.Background()
	store := &stubFormStateStore{}
	deltaExampleID := idwrap.NewNow()
	originFormID := idwrap.NewNow()

	origin := mbodyform.BodyForm{
		ID:          originFormID,
		BodyKey:     "token",
		Value:       "static",
		Description: "auth token",
		Enable:      true,
	}

	delta := mbodyform.BodyForm{
		ID:            idwrap.NewNow(),
		ExampleID:     deltaExampleID,
		DeltaParentID: &originFormID,
		BodyKey:       origin.BodyKey,
		Value:         "{{token}}",
		Description:   origin.Description,
		Enable:        false,
	}

	err := seedMissingFormStateFromDelta(ctx, store, []mbodyform.BodyForm{delta}, map[idwrap.IDWrap]mbodyform.BodyForm{originFormID: origin}, deltaExampleID)
	require.NoError(t, err)
	require.Len(t, store.upserts, 1)

	call := store.upserts[0]
	require.Equal(t, deltaExampleID, call.exampleID)
	require.Equal(t, originFormID, call.originID)
	require.Nil(t, call.keyPtr)
	require.NotNil(t, call.valPtr)
	require.Equal(t, delta.Value, *call.valPtr)
	require.Nil(t, call.descPtr)
	require.NotNil(t, call.enPtr)
	require.Equal(t, delta.Enable, *call.enPtr)
}

func TestSeedMissingFormStateFromDeltaSkipsExistingOverrides(t *testing.T) {
	ctx := context.Background()
	deltaExampleID := idwrap.NewNow()
	originFormID := idwrap.NewNow()

	existingValue := "already-set"
	store := &stubFormStateStore{
		states: map[string]overcore.StateRow{
			originFormID.String(): {
				Key: &existingValue,
			},
		},
	}

	origin := mbodyform.BodyForm{ID: originFormID, BodyKey: "token", Value: "static", Enable: true}
	delta := mbodyform.BodyForm{ID: idwrap.NewNow(), ExampleID: deltaExampleID, DeltaParentID: &originFormID, BodyKey: origin.BodyKey, Value: "{{token}}", Enable: false}

	err := seedMissingFormStateFromDelta(ctx, store, []mbodyform.BodyForm{delta}, map[idwrap.IDWrap]mbodyform.BodyForm{originFormID: origin}, deltaExampleID)
	require.NoError(t, err)
	require.Empty(t, store.upserts)
}
