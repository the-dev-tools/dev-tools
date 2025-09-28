package assertiondelta

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
)

func mustFromHex(t *testing.T, hexStr string) idwrap.IDWrap {
	t.Helper()
	bytes, err := hex.DecodeString(hexStr)
	require.NoError(t, err)
	id, err := idwrap.NewFromBytes(bytes)
	require.NoError(t, err)
	return id
}

type memoryStore struct {
	asserts map[string]massert.Assert
}

func newMemoryStore(asserts ...massert.Assert) *memoryStore {
	store := &memoryStore{asserts: make(map[string]massert.Assert, len(asserts))}
	for _, a := range asserts {
		store.asserts[a.ID.String()] = a
	}
	return store
}

func (m *memoryStore) GetAssert(_ context.Context, id idwrap.IDWrap) (massert.Assert, error) {
	if a, ok := m.asserts[id.String()]; ok {
		return a, nil
	}
	return massert.Assert{}, ErrOriginMismatch
}

func (m *memoryStore) ListByExample(_ context.Context, example idwrap.IDWrap) ([]massert.Assert, error) {
	out := make([]massert.Assert, 0)
	for _, a := range m.asserts {
		if a.ExampleID.Compare(example) == 0 {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *memoryStore) ListByDeltaParent(_ context.Context, parent idwrap.IDWrap) ([]massert.Assert, error) {
	out := make([]massert.Assert, 0)
	for _, a := range m.asserts {
		if a.DeltaParentID != nil && a.DeltaParentID.Compare(parent) == 0 {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *memoryStore) UpdateAssert(_ context.Context, assert massert.Assert) error {
	m.asserts[assert.ID.String()] = assert
	return nil
}

func (m *memoryStore) CreateAssert(_ context.Context, assert massert.Assert) error {
	m.asserts[assert.ID.String()] = assert
	return nil
}

func (m *memoryStore) DeleteAssert(_ context.Context, id idwrap.IDWrap) error {
	delete(m.asserts, id.String())
	return nil
}

func TestApplyUpdateMirrorsDelta(t *testing.T) {
	t.Helper()

	originID := mustFromHex(t, "01999142A92177889AFC3D4C16978DB0")
	originExample := mustFromHex(t, "01999142A92177889AFC3D352E82C393")
	defaultExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D60")
	deltaExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D61")
	defaultAssertID := mustFromHex(t, "01999142A92177889AFC3D4C8860EE00")
	deltaID := mustFromHex(t, "01999142A92177889AFC3D4C8860FF87")

	parent := originID

	store := newMemoryStore(
		massert.Assert{
			ID:        originID,
			ExampleID: originExample,
			Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}},
			Enable:    true,
		},
		massert.Assert{
			ID:        defaultAssertID,
			ExampleID: defaultExample,
			Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}},
			Enable:    true,
		},
		massert.Assert{
			ID:            deltaID,
			ExampleID:     deltaExample,
			DeltaParentID: &parent,
			Condition:     mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}},
			Enable:        true,
		},
	)

	ctx := context.Background()
	result, err := ApplyUpdate(ctx, store, ApplyUpdateInput{
		Origin: ExampleMeta{ID: originExample},
		Delta: []ExampleMeta{
			{ID: defaultExample, HasVersionParent: false},
			{ID: deltaExample, HasVersionParent: true},
		},
		AssertID:  originID,
		Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 200"}},
		Enable:    true,
	})
	require.NoError(t, err)
	require.Equal(t, "response.status == 200", result.Origin.Condition.Comparisons.Expression)
	require.Len(t, result.UpdatedDeltas, 2)
	require.Empty(t, result.CreatedDeltas)
	for _, a := range result.UpdatedDeltas {
		require.Equal(t, "response.status == 200", a.Condition.Comparisons.Expression)
	}
}

func TestApplyUpdateCreatesDeltaWhenMissing(t *testing.T) {
	originID := mustFromHex(t, "01999142A92177889AFC3D4C16978DB0")
	originExample := mustFromHex(t, "01999142A92177889AFC3D352E82C393")
	deltaExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D60")

	store := newMemoryStore(massert.Assert{
		ID:        originID,
		ExampleID: originExample,
		Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}},
		Enable:    true,
	})

	ctx := context.Background()
	result, err := ApplyUpdate(ctx, store, ApplyUpdateInput{
		Origin:    ExampleMeta{ID: originExample},
		Delta:     []ExampleMeta{{ID: deltaExample, HasVersionParent: true}},
		AssertID:  originID,
		Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 200"}},
		Enable:    true,
	})
	require.NoError(t, err)
	require.Len(t, result.CreatedDeltas, 1)
	require.Equal(t, "response.status == 200", result.CreatedDeltas[0].Condition.Comparisons.Expression)
}

func TestApplyDeleteRemovesOriginAndDeltas(t *testing.T) {
    originID := mustFromHex(t, "01999142A92177889AFC3D4C16978DB0")
    originExample := mustFromHex(t, "01999142A92177889AFC3D352E82C393")
    defaultExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D60")
    deltaExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D61")
    defaultAssertID := mustFromHex(t, "01999142A92177889AFC3D4C8860EE00")
    deltaID := mustFromHex(t, "01999142A92177889AFC3D4C8860FF87")

    parent := originID

    store := newMemoryStore(
        massert.Assert{ID: originID, ExampleID: originExample},
        massert.Assert{ID: defaultAssertID, ExampleID: defaultExample},
        massert.Assert{ID: deltaID, ExampleID: deltaExample, DeltaParentID: &parent},
    )

    ctx := context.Background()
    result, err := ApplyDelete(ctx, store, ApplyDeleteInput{
        Origin:   ExampleMeta{ID: originExample},
        Delta:    []ExampleMeta{{ID: defaultExample, HasVersionParent: false}, {ID: deltaExample, HasVersionParent: true}},
        AssertID: originID,
    })
    require.NoError(t, err)
    require.ElementsMatch(t, []idwrap.IDWrap{defaultAssertID, deltaID, originID}, result.DeletedIDs)
    require.Empty(t, store.asserts)
}

func TestLoadEffectiveMergesDelta(t *testing.T) {
	originID := mustFromHex(t, "01999142A92177889AFC3D4C16978DB0")
	originExample := mustFromHex(t, "01999142A92177889AFC3D352E82C393")
	deltaExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D60")
	deltaID := mustFromHex(t, "01999142A92177889AFC3D4C8860FF87")

	parent := originID

	store := newMemoryStore(
		massert.Assert{ID: originID, ExampleID: originExample, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 200"}}, Enable: true},
		massert.Assert{ID: deltaID, ExampleID: deltaExample, DeltaParentID: &parent, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}}, Enable: true},
	)

	ctx := context.Background()
	set, err := LoadEffective(ctx, store, LoadInput{Origin: ExampleMeta{ID: originExample}, Delta: &ExampleMeta{ID: deltaExample, HasVersionParent: true}})
	require.NoError(t, err)
	require.Len(t, set.Merged, 1)
	require.Equal(t, "response.status == 304", set.Merged[0].Assert.Condition.Comparisons.Expression)
	require.Equal(t, massert.AssertSourceMixed, set.Merged[0].Source)
}
