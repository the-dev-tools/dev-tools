package core_test

import (
	"context"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	core "the-dev-tools/server/pkg/overlay/core"
	orank "the-dev-tools/server/pkg/overlay/rank"
	deltav1 "the-dev-tools/spec/dist/buf/go/api/delta/v1"
)

// In-memory stores implementing the core interfaces for tests
type memOrder struct{ rows map[string][]entry }
type entry struct {
	kind int8
	id   idwrap.IDWrap
	rank string
	rev  int64
}

func newMemOrder() *memOrder           { return &memOrder{rows: map[string][]entry{}} }
func exKeyStr(ex idwrap.IDWrap) string { return ex.String() }
func idKeyStr(id idwrap.IDWrap) string { return id.String() }
func (m *memOrder) Count(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	return int64(len(m.rows[exKeyStr(ex)])), nil
}
func (m *memOrder) SelectAsc(ctx context.Context, ex idwrap.IDWrap) ([]core.OrderRow, error) {
	arr := append([]entry(nil), m.rows[exKeyStr(ex)]...)
	// simple bubble by rank (small dataset)
	for i := 0; i < len(arr); i++ {
		for j := i + 1; j < len(arr); j++ {
			if arr[j].rank < arr[i].rank {
				arr[i], arr[j] = arr[j], arr[i]
			}
		}
	}
	out := make([]core.OrderRow, 0, len(arr))
	for _, e := range arr {
		out = append(out, core.OrderRow{RefKind: e.kind, RefID: e.id, Rank: e.rank})
	}
	return out, nil
}
func (m *memOrder) LastRank(ctx context.Context, ex idwrap.IDWrap) (string, bool, error) {
	rows := m.rows[exKeyStr(ex)]
	if len(rows) == 0 {
		return "", false, nil
	}
	// find max
	max := rows[0].rank
	for _, e := range rows {
		if e.rank > max {
			max = e.rank
		}
	}
	return max, true, nil
}
func (m *memOrder) MaxRevision(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	rows := m.rows[exKeyStr(ex)]
	var max int64
	for _, e := range rows {
		if e.rev > max {
			max = e.rev
		}
	}
	return max, nil
}
func (m *memOrder) InsertIgnore(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error {
	k := exKeyStr(ex)
	for _, e := range m.rows[k] {
		if e.kind == refKind && e.id.Compare(refID) == 0 {
			return nil
		}
	}
	m.rows[k] = append(m.rows[k], entry{kind: refKind, id: refID, rank: rank, rev: revision})
	return nil
}
func (m *memOrder) Upsert(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error {
	k := exKeyStr(ex)
	for i, e := range m.rows[k] {
		if e.kind == refKind && e.id.Compare(refID) == 0 {
			m.rows[k][i].rank = rank
			m.rows[k][i].rev = revision
			return nil
		}
	}
	m.rows[k] = append(m.rows[k], entry{kind: refKind, id: refID, rank: rank, rev: revision})
	return nil
}
func (m *memOrder) DeleteByRef(ctx context.Context, ex idwrap.IDWrap, refID idwrap.IDWrap) error {
	k := exKeyStr(ex)
	out := make([]entry, 0, len(m.rows[k]))
	for _, e := range m.rows[k] {
		if e.id.Compare(refID) != 0 {
			out = append(out, e)
		}
	}
	m.rows[k] = out
	return nil
}
func (m *memOrder) Exists(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap) (bool, error) {
	k := exKeyStr(ex)
	for _, e := range m.rows[k] {
		if e.kind == refKind && e.id.Compare(refID) == 0 {
			return true, nil
		}
	}
	return false, nil
}

type memState struct {
	m map[string]map[string]core.StateRow
}

func newMemState() *memState { return &memState{m: map[string]map[string]core.StateRow{}} }
func (s *memState) Upsert(ctx context.Context, ex, origin idwrap.IDWrap, suppressed bool, k, val, desc *string, enabled *bool) error {
	ek, ok := s.m[exKeyStr(ex)]
	if !ok {
		ek = map[string]core.StateRow{}
		s.m[exKeyStr(ex)] = ek
	}
	row := ek[idKeyStr(origin)]
	row.Suppressed = suppressed
	if k != nil {
		row.Key = k
	}
	if val != nil {
		row.Val = val
	}
	if desc != nil {
		row.Desc = desc
	}
	if enabled != nil {
		row.Enabled = enabled
	}
	ek[idKeyStr(origin)] = row
	return nil
}
func (s *memState) Get(ctx context.Context, ex, origin idwrap.IDWrap) (core.StateRow, bool, error) {
	ek := s.m[exKeyStr(ex)]
	if ek == nil {
		return core.StateRow{}, false, nil
	}
	row, ok := ek[idKeyStr(origin)]
	if !ok {
		return core.StateRow{}, false, nil
	}
	return row, true, nil
}
func (s *memState) ClearOverrides(ctx context.Context, ex, origin idwrap.IDWrap) error {
	ek := s.m[exKeyStr(ex)]
	if ek == nil {
		return nil
	}
	row := ek[idKeyStr(origin)]
	row.Key, row.Val, row.Desc, row.Enabled = nil, nil, nil, nil
	ek[idKeyStr(origin)] = row
	return nil
}
func (s *memState) Suppress(ctx context.Context, ex, origin idwrap.IDWrap) error {
	ek, ok := s.m[exKeyStr(ex)]
	if !ok {
		ek = map[string]core.StateRow{}
		s.m[exKeyStr(ex)] = ek
	}
	row := ek[idKeyStr(origin)]
	row.Suppressed = true
	ek[idKeyStr(origin)] = row
	return nil
}
func (s *memState) Unsuppress(ctx context.Context, ex, origin idwrap.IDWrap) error {
	ek := s.m[exKeyStr(ex)]
	if ek == nil {
		return nil
	}
	row := ek[idKeyStr(origin)]
	row.Suppressed = false
	ek[idKeyStr(origin)] = row
	return nil
}

type memDelta struct{ m map[string]map[string]drow }
type drow struct {
	k, v, d string
	e       bool
}

func newMemDelta() *memDelta { return &memDelta{m: map[string]map[string]drow{}} }
func (d *memDelta) Insert(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	ek := d.m[exKeyStr(ex)]
	if ek == nil {
		ek = map[string]drow{}
		d.m[exKeyStr(ex)] = ek
	}
	ek[idKeyStr(id)] = drow{k: key, v: value, d: desc, e: enabled}
	return nil
}
func (d *memDelta) Update(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	ek := d.m[exKeyStr(ex)]
	if ek == nil {
		ek = map[string]drow{}
		d.m[exKeyStr(ex)] = ek
	}
	ek[idKeyStr(id)] = drow{k: key, v: value, d: desc, e: enabled}
	return nil
}
func (d *memDelta) Get(ctx context.Context, ex, id idwrap.IDWrap) (string, string, string, bool, bool, error) {
	ek := d.m[exKeyStr(ex)]
	if ek == nil {
		return "", "", "", false, false, nil
	}
	row, ok := ek[idKeyStr(id)]
	if !ok {
		return "", "", "", false, false, nil
	}
	return row.k, row.v, row.d, row.e, true, nil
}
func (d *memDelta) Exists(ctx context.Context, ex, id idwrap.IDWrap) (bool, error) {
	ek := d.m[exKeyStr(ex)]
	if ek == nil {
		return false, nil
	}
	_, ok := ek[idKeyStr(id)]
	return ok, nil
}
func (d *memDelta) Delete(ctx context.Context, ex, id idwrap.IDWrap) error {
	ek := d.m[exKeyStr(ex)]
	if ek == nil {
		return nil
	}
	delete(ek, idKeyStr(id))
	return nil
}

// Helpers
func originValsFrom(m map[idwrap.IDWrap]core.Values) func(ctx context.Context, ids []idwrap.IDWrap) (map[idwrap.IDWrap]core.Values, error) {
	return func(ctx context.Context, ids []idwrap.IDWrap) (map[idwrap.IDWrap]core.Values, error) { return m, nil }
}

type testItem struct {
	id     idwrap.IDWrap
	vals   core.Values
	origin *core.Values
	src    deltav1.SourceKind
}

func TestCore_List_ClassifyOriginMixedDelta(t *testing.T) {
	ctx := context.Background()
	ex := idwrap.NewNow()
	oex := idwrap.NewNow()
	a := idwrap.NewNow()
	b := idwrap.NewNow()
	d := idwrap.NewNow()

	order := newMemOrder()
	state := newMemState()
	delta := newMemDelta()

	// Seed order: A(origin), B(origin), D(delta)
	ra := orank.First()
	rb := orank.Between(ra, "")
	rd := orank.Between(rb, "")
	_ = order.Upsert(ctx, ex, core.RefKindOrigin, a, ra, 1)
	_ = order.Upsert(ctx, ex, core.RefKindOrigin, b, rb, 2)
	_ = order.Upsert(ctx, ex, core.RefKindDelta, d, rd, 3)

	// Origins
	origins := map[idwrap.IDWrap]core.Values{
		a: {Key: "ka", Value: "va", Description: "da", Enabled: true},
		b: {Key: "kb", Value: "vb", Description: "db", Enabled: false},
	}
	// State: A override differs => MIXED; B override same value => still ORIGIN
	ov := "va2"
	_ = state.Upsert(ctx, ex, a, false, nil, &ov, nil, nil)
	same := "kb"
	_ = state.Upsert(ctx, ex, b, false, &same, nil, nil, nil)
	// Delta-only
	_ = delta.Insert(ctx, ex, d, "kd", "vd", "dd", true)

	build := func(m core.Merged) any {
		var o *core.Values
		if m.Origin != nil {
			v := *m.Origin
			o = &v
		}
		return testItem{id: m.ID, vals: m.Values, origin: o, src: m.Source}
	}

	itemsAny, err := core.List(ctx, order, state, delta, func(context.Context, idwrap.IDWrap) ([]struct{}, error) { return nil, nil }, func(struct{}) core.Values { return core.Values{} }, originValsFrom(origins), build, ex, oex)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]testItem, 0, len(itemsAny))
	for _, it := range itemsAny {
		items = append(items, it.(testItem))
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 merged items, got %d", len(items))
	}
	// A should be MIXED
	if items[0].id.Compare(a) != 0 || items[0].src != deltav1.SourceKind_SOURCE_KIND_MIXED || items[0].origin == nil || items[0].vals.Value != "va2" {
		t.Fatalf("expected A mixed with override, got %+v", items[0])
	}
	// B should be ORIGIN (override equals origin)
	if items[1].id.Compare(b) != 0 || items[1].src != deltav1.SourceKind_SOURCE_KIND_ORIGIN || items[1].origin == nil {
		t.Fatalf("expected B origin, got %+v", items[1])
	}
	// D should be DELTA with no origin
	if items[2].id.Compare(d) != 0 || items[2].src != deltav1.SourceKind_SOURCE_KIND_DELTA || items[2].origin != nil {
		t.Fatalf("expected D delta, got %+v", items[2])
	}
}

func TestCore_Move_Order(t *testing.T) {
	ctx := context.Background()
	ex := idwrap.NewNow()
	a := idwrap.NewNow()
	b := idwrap.NewNow()
	d := idwrap.NewNow()
	order := newMemOrder()
	// seed order A,B,D
	_ = order.Upsert(ctx, ex, core.RefKindOrigin, a, orank.First(), 1)
	_ = order.Upsert(ctx, ex, core.RefKindOrigin, b, orank.Between(orank.First(), ""), 2)
	_ = order.Upsert(ctx, ex, core.RefKindDelta, d, orank.Between(orank.Between(orank.First(), ""), ""), 3)

	// Move A after B => order B,A,D
	if err := core.Move(ctx, order, ex, a, b, true); err != nil {
		t.Fatal(err)
	}
	ord, _ := order.SelectAsc(ctx, ex)
	if !(ord[0].RefID.Compare(b) == 0 && ord[1].RefID.Compare(a) == 0 && ord[2].RefID.Compare(d) == 0) {
		t.Fatalf("unexpected order after move: %+v", ord)
	}
	// Move D before B => D,B,A
	if err := core.Move(ctx, order, ex, d, b, false); err != nil {
		t.Fatal(err)
	}
	ord, _ = order.SelectAsc(ctx, ex)
	if !(ord[0].RefID.Compare(d) == 0 && ord[1].RefID.Compare(b) == 0 && ord[2].RefID.Compare(a) == 0) {
		t.Fatalf("unexpected order after second move: %+v", ord)
	}
	// Ensure kinds preserved
	if ord[0].RefKind != core.RefKindDelta || ord[1].RefKind != core.RefKindOrigin || ord[2].RefKind != core.RefKindOrigin {
		t.Fatalf("ref kinds not preserved after moves: %+v", ord)
	}
}

func TestCore_Reset_Delete_Undelete(t *testing.T) {
	ctx := context.Background()
	ex := idwrap.NewNow()
	oex := idwrap.NewNow()
	a := idwrap.NewNow()
	d := idwrap.NewNow()
	order := newMemOrder()
	state := newMemState()
	delta := newMemDelta()
	// seed A origin and D delta
	_ = order.Upsert(ctx, ex, core.RefKindOrigin, a, orank.First(), 1)
	_ = order.Upsert(ctx, ex, core.RefKindDelta, d, orank.Between(orank.First(), ""), 2)
	origins := map[idwrap.IDWrap]core.Values{a: {Key: "ka", Value: "va", Description: "da", Enabled: true}}
	// set override for A
	nv := "va2"
	_ = state.Upsert(ctx, ex, a, false, nil, &nv, nil, nil)
	// set delta values for D
	_ = delta.Insert(ctx, ex, d, "kd", "vd", "dd", false)
	// Reset A and D
	if err := core.Reset(ctx, state, delta, ex, a); err != nil {
		t.Fatal(err)
	}
	if err := core.Reset(ctx, state, delta, ex, d); err != nil {
		t.Fatal(err)
	}
	// List: A should be ORIGIN (override cleared), D should be DELTA with empty values and enabled true
	build := func(m core.Merged) any { return m }
	itemsAny, err := core.List(ctx, order, state, delta, func(context.Context, idwrap.IDWrap) ([]struct{}, error) { return nil, nil }, func(struct{}) core.Values { return core.Values{} }, originValsFrom(origins), build, ex, oex)
	if err != nil {
		t.Fatal(err)
	}
	// find A and D
	var foundA, foundD bool
	for _, it := range itemsAny {
		m := it.(core.Merged)
		if m.ID.Compare(a) == 0 {
			foundA = (m.Source == deltav1.SourceKind_SOURCE_KIND_ORIGIN)
		}
		if m.ID.Compare(d) == 0 {
			foundD = (m.Source == deltav1.SourceKind_SOURCE_KIND_DELTA && m.Values.Key == "" && m.Values.Value == "" && m.Values.Description == "" && m.Values.Enabled)
		}
	}
	if !foundA || !foundD {
		t.Fatalf("unexpected merged after reset: A:%v D:%v", foundA, foundD)
	}
	// Delete A (origin) => suppressed
	if err := core.Delete(ctx, order, state, delta, ex, a); err != nil {
		t.Fatal(err)
	}
	itemsAny, err = core.List(ctx, order, state, delta, func(context.Context, idwrap.IDWrap) ([]struct{}, error) { return nil, nil }, func(struct{}) core.Values { return core.Values{} }, originValsFrom(origins), build, ex, oex)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range itemsAny {
		if it.(core.Merged).ID.Compare(a) == 0 {
			t.Fatalf("expected A suppressed and removed from list")
		}
	}
	// Undelete A => appended at tail
	if err := core.Undelete(ctx, order, state, ex, a); err != nil {
		t.Fatal(err)
	}
	ord, _ := order.SelectAsc(ctx, ex)
	if ord[len(ord)-1].RefID.Compare(a) != 0 {
		t.Fatalf("expected A appended at tail on undelete")
	}
}

func TestCore_Delete_DeltaOnly_NoSuppress(t *testing.T) {
	ctx := context.Background()
	ex := idwrap.NewNow()
	d := idwrap.NewNow()
	order := newMemOrder()
	state := newMemState()
	delta := newMemDelta()
	// delta-only row present in delta + order
	_ = delta.Insert(ctx, ex, d, "k", "v", "d", true)
	_ = order.Upsert(ctx, ex, core.RefKindDelta, d, orank.First(), 1)
	// delete
	if err := core.Delete(ctx, order, state, delta, ex, d); err != nil {
		t.Fatal(err)
	}
	// ensure removed from order
	ord, _ := order.SelectAsc(ctx, ex)
	if len(ord) != 0 {
		t.Fatalf("expected empty order after delta delete, got %v", ord)
	}
	// ensure no state row created
	if _, ok, _ := state.Get(ctx, ex, d); ok {
		t.Fatalf("expected no state row for delta-only delete")
	}
}
