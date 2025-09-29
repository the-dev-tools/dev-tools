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

func TestApplyUpdateMirrorsMultipleDeltaSources(t *testing.T) {
	originID := mustFromHex(t, "01999142A92177889AFC3D4C16978DB1")
	originExample := mustFromHex(t, "01999142A92177889AFC3D352E82C394")
	defaultExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D70")
	deltaExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D71")
	legacyExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D72")
	defaultAssertID := mustFromHex(t, "01999142A92177889AFC3D4C8860EE01")
	deltaID := mustFromHex(t, "01999142A92177889AFC3D4C8860FF88")
	legacyDeltaID := mustFromHex(t, "01999142A92177889AFC3D4C8860FF89")

	parent := originID
	store := newMemoryStore(
		massert.Assert{ID: originID, ExampleID: originExample, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}}, Enable: true},
		massert.Assert{ID: defaultAssertID, ExampleID: defaultExample, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}}, Enable: true},
		massert.Assert{ID: deltaID, ExampleID: deltaExample, DeltaParentID: &parent, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}}, Enable: true},
		massert.Assert{ID: legacyDeltaID, ExampleID: legacyExample, DeltaParentID: &parent, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}}, Enable: true},
	)

	ctx := context.Background()
	result, err := ApplyUpdate(ctx, store, ApplyUpdateInput{
		Origin: ExampleMeta{ID: originExample},
		Delta: []ExampleMeta{
			{ID: defaultExample, HasVersionParent: false},
			{ID: deltaExample, HasVersionParent: true},
		},
		AssertID:  originID,
		Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.time < 500"}},
		Enable:    false,
	})
	require.NoError(t, err)
	require.Len(t, result.UpdatedDeltas, 3)
	require.Empty(t, result.CreatedDeltas)

	for _, updated := range result.UpdatedDeltas {
		require.Equal(t, "response.time < 500", updated.Condition.Comparisons.Expression)
		require.False(t, updated.Enable)
	}

	legacy, err := store.GetAssert(ctx, legacyDeltaID)
	require.NoError(t, err)
	require.Equal(t, "response.time < 500", legacy.Condition.Comparisons.Expression)
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

func TestApplyUpdateCreatesMultipleDeltaWhenMissing(t *testing.T) {
	originID := mustFromHex(t, "01999142A92177889AFC3D4C16978DB2")
	originExample := mustFromHex(t, "01999142A92177889AFC3D352E82C395")
	deltaExampleA := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D80")
	deltaExampleB := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D81")

	store := newMemoryStore(massert.Assert{ID: originID, ExampleID: originExample, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}}, Enable: true})

	ctx := context.Background()
	result, err := ApplyUpdate(ctx, store, ApplyUpdateInput{
		Origin: ExampleMeta{ID: originExample},
		Delta: []ExampleMeta{
			{ID: deltaExampleA, HasVersionParent: true},
			{ID: deltaExampleB, HasVersionParent: true},
		},
		AssertID:  originID,
		Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 201"}},
		Enable:    true,
	})
	require.NoError(t, err)
	require.Len(t, result.CreatedDeltas, 2)
	require.Len(t, result.UpdatedDeltas, 2)

	seenExamples := map[string]struct{}{}
	for _, created := range result.CreatedDeltas {
		seenExamples[created.ExampleID.String()] = struct{}{}
		require.Equal(t, "response.status == 201", created.Condition.Comparisons.Expression)
		require.NotNil(t, created.DeltaParentID)
		require.Equal(t, 0, created.DeltaParentID.Compare(originID))
	}
	require.Contains(t, seenExamples, deltaExampleA.String())
	require.Contains(t, seenExamples, deltaExampleB.String())
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

func TestApplyDeleteCascadesAcrossMultipleDeltaExamples(t *testing.T) {
	originID := mustFromHex(t, "01999142A92177889AFC3D4C16978DB3")
	originExample := mustFromHex(t, "01999142A92177889AFC3D352E82C396")
	defaultExample := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D90")
	deltaExampleA := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D91")
	deltaExampleB := mustFromHex(t, "01999142A92177889AFC3D35BC2F8D92")
	defaultAssertID := mustFromHex(t, "01999142A92177889AFC3D4C8860EE02")
	deltaID := mustFromHex(t, "01999142A92177889AFC3D4C8860FF98")
	legacyDeltaID := mustFromHex(t, "01999142A92177889AFC3D4C8860FF99")

	parent := originID
	store := newMemoryStore(
		massert.Assert{ID: originID, ExampleID: originExample, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 200"}}, Enable: true, Prev: nil, Next: nil},
		massert.Assert{ID: defaultAssertID, ExampleID: defaultExample, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 200"}}, Enable: true},
		massert.Assert{ID: deltaID, ExampleID: deltaExampleA, DeltaParentID: &parent, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 200"}}, Enable: true},
		massert.Assert{ID: legacyDeltaID, ExampleID: deltaExampleB, DeltaParentID: &parent, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 200"}}, Enable: true},
	)

	ctx := context.Background()
	result, err := ApplyDelete(ctx, store, ApplyDeleteInput{
		Origin:   ExampleMeta{ID: originExample},
		Delta:    []ExampleMeta{{ID: defaultExample, HasVersionParent: false}, {ID: deltaExampleA, HasVersionParent: true}},
		AssertID: originID,
	})
	require.NoError(t, err)

	require.ElementsMatch(t, []idwrap.IDWrap{defaultAssertID, deltaID, legacyDeltaID, originID}, result.DeletedIDs)
	require.Empty(t, store.asserts)
}
