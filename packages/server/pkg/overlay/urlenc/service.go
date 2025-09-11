package urlenc

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyurl"
	orank "the-dev-tools/server/pkg/overlay/rank"
	"the-dev-tools/server/pkg/service/sbodyurl"
	soverlayurlenc "the-dev-tools/server/pkg/service/soverlayurlenc"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
)

// RefKind for delta overlay order rows
const (
	refKindOrigin int8 = 1
	refKindDelta  int8 = 2
)

type Service struct {
    bues sbodyurl.BodyURLEncodedService
    ovs  *soverlayurlenc.Service
}

func New(db *sql.DB, bues sbodyurl.BodyURLEncodedService) *Service {
    ovs, _ := soverlayurlenc.New(db)
    return &Service{bues: bues, ovs: ovs}
}

// Proxy ID encoding helpers. We expose origin-anchored proxies with a stable
// overlay-specific ID that differs from the origin ID. This lets clients
// distinguish proxy IDs from origin IDs for update semantics.
var proxySalt = [16]byte{0x21, 0x42, 0x63, 0x84, 0xa5, 0xc6, 0xe7, 0x08, 0x29, 0x4a, 0x6b, 0x8c, 0xad, 0xce, 0xef, 0x10}

func EncodeProxyID(id idwrap.IDWrap) idwrap.IDWrap {
    b := make([]byte, len(id.Bytes()))
    copy(b, id.Bytes())
    for i := 0; i < len(b) && i < len(proxySalt); i++ {
        b[i] ^= proxySalt[i]
    }
    return idwrap.NewFromBytesMust(b)
}

func TryDecodeProxyID(id idwrap.IDWrap) (idwrap.IDWrap, bool) {
    // Decoding is symmetric XOR. Caller should validate existence after decoding.
    b := make([]byte, len(id.Bytes()))
    copy(b, id.Bytes())
    for i := 0; i < len(b) && i < len(proxySalt); i++ {
        b[i] ^= proxySalt[i]
    }
    return idwrap.NewFromBytesMust(b), true
}

// EnsureSeeded creates overlay order entries for all origin url-enc items
// in origin order if the overlay has no entries yet for deltaExampleID.
func (s *Service) EnsureSeeded(ctx context.Context, deltaExampleID, originExampleID idwrap.IDWrap) error {
    // Read current overlay order to decide seeding strategy (supports incremental ensure-missing behavior)
    ord, err := s.readOrder(ctx, deltaExampleID)
    if err != nil {
        return err
    }
    // Fetch origin list (ordered, with fallback)
    origin, err := s.bues.GetBodyURLEncodedByExampleIDOrdered(ctx, originExampleID)
    if err != nil {
        origin, err = s.bues.GetBodyURLEncodedByExampleID(ctx, originExampleID)
        if err != nil {
            return err
        }
    }
    if len(origin) == 0 {
        return nil
    }
    // Map existing origin anchors in current order
    present := map[string]bool{}
    hasAnyOrigin := false
    for _, r := range ord {
        if r.RefKind == refKindOrigin {
            present[r.RefID.String()] = true
            hasAnyOrigin = true
        }
    }
    nextRev, err := s.nextRevision(ctx, deltaExampleID)
    if err != nil {
        return err
    }
    // Case 1: No origin anchors yet — append all proxies after any existing delta-only rows
    if !hasAnyOrigin {
        last, ok, err := s.ovs.LastOrderRank(ctx, deltaExampleID)
        if err != nil {
            return err
        }
        for i := range origin {
            newRank := ""
            if ok {
                newRank = orank.Between(last, "")
            } else {
                newRank = orank.First()
            }
            if err := s.ovs.UpsertOrderRank(ctx, deltaExampleID, refKindOrigin, origin[i].ID, newRank, nextRev); err != nil {
                return err
            }
            // Ensure not suppressed if previously deleted
            _ = s.ovs.UnsuppressState(ctx, deltaExampleID, origin[i].ID)
            nextRev++
            last = newRank
            ok = true
        }
        return nil
    }
    // Helper: rank and position maps for quick lookup (positions in current overlay order)
    rankByID := map[string]string{}
    posByID := map[string]int{}
    for i, r := range ord {
        rankByID[r.RefID.String()] = r.Rank
        posByID[r.RefID.String()] = i
    }
    // Case 2: Some anchors exist — insert any missing origin proxies between nearest anchors based on origin order
    for idx := range origin {
        id := origin[idx].ID
        idStr := id.String()
        if present[idStr] {
            continue // already anchored
        }
        // Find nearest present origin anchors in origin order
        var leftAnchorID *idwrap.IDWrap
        var rightAnchorID *idwrap.IDWrap
        for l := idx - 1; l >= 0; l-- {
            if present[origin[l].ID.String()] {
                tmp := origin[l].ID
                leftAnchorID = &tmp
                break
            }
        }
        for r := idx + 1; r < len(origin); r++ {
            if present[origin[r].ID.String()] {
                tmp := origin[r].ID
                rightAnchorID = &tmp
                break
            }
        }
        // Choose placement bounds based on overlay positions to preserve user order
        var leftBound, rightBound string
        if leftAnchorID != nil && rightAnchorID != nil {
            // Insert after whichever anchor appears later in current overlay order
            li := posByID[leftAnchorID.String()]
            ri := posByID[rightAnchorID.String()]
            if li >= ri {
                // after left anchor
                leftBound = rankByID[leftAnchorID.String()]
                if li+1 < len(ord) {
                    rightBound = ord[li+1].Rank
                }
            } else {
                // before right anchor
                rightBound = rankByID[rightAnchorID.String()]
                if ri-1 >= 0 {
                    leftBound = ord[ri-1].Rank
                }
            }
        } else {
            // Only one or no anchor: do not auto-insert (respect deletions)
            continue
        }
        newRank := orank.Between(leftBound, rightBound)
        if err := s.ovs.UpsertOrderRank(ctx, deltaExampleID, refKindOrigin, id, newRank, nextRev); err != nil {
            return err
        }
        // If this origin was previously suppressed, clear suppression
        _ = s.ovs.UnsuppressState(ctx, deltaExampleID, id)
        nextRev++
        // update maps for subsequent insertions
        present[idStr] = true
        rankByID[idStr] = newRank
        // Best-effort position update: assume inserted after/before chosen bounds near existing indices
        // We set position equal to len(ord) to bias subsequent insertions towards tail when unsure.
        posByID[idStr] = len(ord)
    }
    return nil
}

type orderRow struct {
	RefKind int8
	RefID   idwrap.IDWrap
	Rank    string
}

func (s *Service) readOrder(ctx context.Context, exampleID idwrap.IDWrap) ([]orderRow, error) {
	rows, err := s.ovs.SelectOrderAsc(ctx, exampleID)
	if err != nil {
		return nil, err
	}
	out := make([]orderRow, 0, len(rows))
	for _, r := range rows {
		oid, err := idwrap.NewFromBytes(r.RefID)
		if err != nil {
			return nil, err
		}
		out = append(out, orderRow{RefKind: r.RefKind, RefID: oid, Rank: r.Rank})
	}
	return out, nil
}

func (s *Service) nextRevision(ctx context.Context, exampleID idwrap.IDWrap) (int64, error) {
	max, err := s.ovs.MaxOrderRevision(ctx, exampleID)
	if err != nil {
		return 0, err
	}
	return max + 1, nil
}

// List merges overlay order, state, and delta.
func (s *Service) List(ctx context.Context, deltaExampleID, originExampleID idwrap.IDWrap) ([]*bodyv1.BodyUrlEncodedDeltaListItem, error) {
	// ensure seeded at least once
	if err := s.EnsureSeeded(ctx, deltaExampleID, originExampleID); err != nil {
		return nil, err
	}
	ord, err := s.readOrder(ctx, deltaExampleID)
	if err != nil {
		return nil, err
	}
	// build origin maps
	originItems, err := s.bues.GetBodyURLEncodedByExampleID(ctx, originExampleID)
	if err != nil {
		return nil, err
	}
	originByID := map[idwrap.IDWrap]mbodyurl.BodyURLEncoded{}
	for _, o := range originItems {
		originByID[o.ID] = o
	}
	// state cached per origin id
	type stateRow struct {
		Suppressed     bool
		Key, Val, Desc sql.NullString
		Enabled        sql.NullBool
	}
	stateCache := map[idwrap.IDWrap]stateRow{}
	loadState := func(id idwrap.IDWrap) (stateRow, error) {
		if v, ok := stateCache[id]; ok {
			return v, nil
		}
		sr, ok, err := s.ovs.GetState(ctx, deltaExampleID, id)
		if err != nil {
			return stateRow{}, err
		}
		if !ok {
			sRow := stateRow{Suppressed: false}
			stateCache[id] = sRow
			return sRow, nil
		}
		sRow := stateRow{Suppressed: sr.Suppressed, Key: sr.Key, Val: sr.Val, Desc: sr.Desc, Enabled: sr.Enabled}
		stateCache[id] = sRow
		return sRow, nil
	}
	// helper to coalesce override
	coalesce := func(ns sql.NullString, def string) string {
		if ns.Valid {
			return ns.String
		}
		return def
	}
	coalesceB := func(nb sql.NullBool, def bool) bool {
		if nb.Valid {
			return nb.Bool
		}
		return def
	}

	out := make([]*bodyv1.BodyUrlEncodedDeltaListItem, 0, len(ord))
	for _, o := range ord {
		switch o.RefKind {
		case refKindOrigin:
			orig, ok := originByID[o.RefID]
			if !ok {
				continue
			}
			st, err := loadState(o.RefID)
			if err != nil {
				return nil, err
			}
			if st.Suppressed {
				continue
			}
			// Determine source kind: MIXED if any override present
			var mixed bool
			mixed = st.Key.Valid || st.Val.Valid || st.Desc.Valid || st.Enabled.Valid
            // Treat proxies anchored to origin as delta-layer items by default
            kind := deltav1.SourceKind_SOURCE_KIND_DELTA
            key := orig.BodyKey
            val := orig.Value
            desc := orig.Description
            en := orig.Enable
            if mixed {
                kind = deltav1.SourceKind_SOURCE_KIND_MIXED
                key = coalesce(st.Key, key)
                val = coalesce(st.Val, val)
                desc = coalesce(st.Desc, desc)
                en = coalesceB(st.Enabled, en)
            }
            // embed origin RPC
            originRPC := &bodyv1.BodyUrlEncoded{BodyId: orig.ID.Bytes(), Key: orig.BodyKey, Enabled: orig.Enable, Value: orig.Value, Description: orig.Description}
            // expose proxy with encoded ID distinct from origin ID
            proxyID := EncodeProxyID(o.RefID)
            out = append(out, &bodyv1.BodyUrlEncodedDeltaListItem{
                BodyId: proxyID.Bytes(),
                Key:    key, Enabled: en, Value: val, Description: desc,
                Origin: originRPC, Source: &kind,
            })
		case refKindDelta:
			// load delta-only row
			var key, val, desc string
			var en bool
			var found bool
			key, val, desc, en, found, err = s.ovs.GetDelta(ctx, deltaExampleID, o.RefID)
			if err != nil {
				return nil, err
			}
			if !found {
				continue
			}
			kind := deltav1.SourceKind_SOURCE_KIND_DELTA
			out = append(out, &bodyv1.BodyUrlEncodedDeltaListItem{
				BodyId: o.RefID.Bytes(),
				Key:    key, Enabled: en, Value: val, Description: desc,
				Origin: nil, Source: &kind,
			})
		}
	}
	return out, nil
}

// CreateDelta inserts a delta-only row and appends to tail in overlay order.
func (s *Service) CreateDelta(ctx context.Context, deltaExampleID idwrap.IDWrap) (idwrap.IDWrap, error) {
	id := idwrap.NewNow()
	if err := s.ovs.InsertDelta(ctx, deltaExampleID, id, "", "", "", true); err != nil {
		return idwrap.IDWrap{}, err
	}
	// tail rank
	last, ok, err := s.ovs.LastOrderRank(ctx, deltaExampleID)
	if err != nil {
		return idwrap.IDWrap{}, err
	}
	var newRank string
	if ok {
		newRank = orank.Between(last, "")
	} else {
		newRank = orank.First()
	}
	nextRev, err := s.nextRevision(ctx, deltaExampleID)
	if err != nil {
		return idwrap.IDWrap{}, err
	}
	if err := s.ovs.UpsertOrderRank(ctx, deltaExampleID, refKindDelta, id, newRank, nextRev); err != nil {
		return idwrap.IDWrap{}, err
	}
	return id, nil
}

// Update applies overrides: if bodyID exists in delta table, updates it; else upserts state for origin id.
func (s *Service) Update(ctx context.Context, deltaExampleID idwrap.IDWrap, bodyID idwrap.IDWrap, key, val, desc *string, enabled *bool) error {
	// is delta?
	if ok, _ := s.ovs.ExistsDelta(ctx, deltaExampleID, bodyID); ok {
		var ck, cv, cd string
		var ce bool
		var found bool
		ck, cv, cd, ce, found, err := s.ovs.GetDelta(ctx, deltaExampleID, bodyID)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("delta row disappeared during update")
		}
		if key != nil {
			ck = *key
		}
		if val != nil {
			cv = *val
		}
		if desc != nil {
			cd = *desc
		}
		if enabled != nil {
			ce = *enabled
		}
		return s.ovs.UpdateDelta(ctx, deltaExampleID, bodyID, ck, cv, cd, ce)
	}
	// origin-ref: upsert state
	return s.ovs.UpsertState(ctx, deltaExampleID, bodyID, false, key, val, desc, enabled)
}

// Move re-ranks bodyID relative to targetID using BEFORE/AFTER. Seeds if needed.
func (s *Service) Move(ctx context.Context, deltaExampleID, originExampleID idwrap.IDWrap, bodyID, targetID idwrap.IDWrap, after bool) error {
	// ensure order exists (seed if empty)
	ord, err := s.readOrder(ctx, deltaExampleID)
	if err != nil {
		return err
	}
	if len(ord) == 0 {
		if err := s.EnsureSeeded(ctx, deltaExampleID, originExampleID); err != nil {
			return err
		}
		ord, err = s.readOrder(ctx, deltaExampleID)
		if err != nil {
			return err
		}
	}
	// ensure moving item exists; if not, insert with tail rank so we can then move
	idxBy := map[string]int{}
	for i, r := range ord {
		idxBy[r.RefID.String()] = i
	}
	_, hasTarget := idxBy[targetID.String()]
	if !hasTarget {
		return errors.New("target not found in overlay order")
	}
	// compute neighbors around target
	tIdx := idxBy[targetID.String()]
	var leftRank, rightRank string
	if after {
		// insert after target -> between target and target+1
		leftRank = ord[tIdx].Rank
		if tIdx+1 < len(ord) {
			rightRank = ord[tIdx+1].Rank
		} else {
			rightRank = ""
		}
	} else {
		// before target -> between target-1 and target
		rightRank = ord[tIdx].Rank
		if tIdx-1 >= 0 {
			leftRank = ord[tIdx-1].Rank
		} else {
			leftRank = ""
		}
	}
	newRank := orank.Between(leftRank, rightRank)
	rev, err := s.nextRevision(ctx, deltaExampleID)
	if err != nil {
		return err
	}
	// upsert order row for bodyID; we don't know if bodyID is origin or delta, try delta first
	rk := refKindOrigin
	if ok, _ := s.ovs.ExistsDelta(ctx, deltaExampleID, bodyID); ok {
		rk = refKindDelta
	}
	// upsert
	return s.ovs.UpsertOrderRank(ctx, deltaExampleID, rk, bodyID, newRank, rev)
}

// Delete deletes delta-only rows and removes origin-ref entries (soft-delete state + remove from order).
func (s *Service) Delete(ctx context.Context, deltaExampleID idwrap.IDWrap, bodyID idwrap.IDWrap) error {
	// delta-only: delete delta row and remove order; do not suppress state
	if ok, _ := s.ovs.ExistsDelta(ctx, deltaExampleID, bodyID); ok {
		if err := s.ovs.DeleteDelta(ctx, deltaExampleID, bodyID); err != nil {
			return err
		}
		if err := s.ovs.DeleteOrderByRef(ctx, deltaExampleID, bodyID); err != nil {
			return err
		}
		return nil
	}
	// origin-ref: remove from order and mark suppressed in state
	if err := s.ovs.DeleteOrderByRef(ctx, deltaExampleID, bodyID); err != nil {
		return err
	}
	return s.ovs.SuppressState(ctx, deltaExampleID, bodyID)
}

// Reset clears overrides for origin-ref or values for delta-only.
func (s *Service) Reset(ctx context.Context, deltaExampleID idwrap.IDWrap, bodyID idwrap.IDWrap) error {
	// if delta: clear values
	if ok, _ := s.ovs.ExistsDelta(ctx, deltaExampleID, bodyID); ok {
		return s.ovs.UpdateDelta(ctx, deltaExampleID, bodyID, "", "", "", true)
	}
	// origin: clear overrides (keep suppressed as is)
	return s.ovs.ClearStateOverrides(ctx, deltaExampleID, bodyID)
}

// IsDelta reports whether the given id corresponds to a delta-only row in this overlay example.
func (s *Service) IsDelta(ctx context.Context, deltaExampleID, id idwrap.IDWrap) (bool, error) {
    return s.ovs.ExistsDelta(ctx, deltaExampleID, id)
}

// Undelete clears suppressed and appends the origin-ref at tail.
func (s *Service) Undelete(ctx context.Context, deltaExampleID idwrap.IDWrap, bodyID idwrap.IDWrap) error {
	// unsuppress
	if err := s.ovs.UnsuppressState(ctx, deltaExampleID, bodyID); err != nil {
		return err
	}
	// append order entry if not exists
	if ok, _ := s.ovs.ExistsOrderRow(ctx, deltaExampleID, refKindOrigin, bodyID); ok {
		return nil
	}
	last, ok, err := s.ovs.LastOrderRank(ctx, deltaExampleID)
	if err != nil {
		return err
	}
	var newRank string
	if ok {
		newRank = orank.Between(last, "")
	} else {
		newRank = orank.First()
	}
	rev, err := s.nextRevision(ctx, deltaExampleID)
	if err != nil {
		return err
	}
	return s.ovs.UpsertOrderRank(ctx, deltaExampleID, refKindOrigin, bodyID, newRank, rev)
}

// Utility nullable wrappers
// nolint:unused
func strPtrOrNil(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}
// nolint:unused
func boolPtrOrNil(p *bool) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// ByKeySort sorts list items by key, stable (used in tests if needed)
type byKey []*bodyv1.BodyUrlEncodedDeltaListItem

func (s byKey) Len() int           { return len(s) }
func (s byKey) Less(i, j int) bool { return s[i].Key < s[j].Key }
func (s byKey) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// SortByKey is a helper to make output deterministic in some scenarios
func SortByKey(items []*bodyv1.BodyUrlEncodedDeltaListItem) {
	sort.Sort(byKey(items))
}

// ResolveExampleForBodyID attempts to find the delta example scope for a body id
// by checking delta-only table first, then order (origin-ref).
func (s *Service) ResolveExampleForBodyID(ctx context.Context, bodyID idwrap.IDWrap) (idwrap.IDWrap, bool, error) {
	if ex, ok, err := s.ovs.ResolveExampleByDeltaID(ctx, bodyID); err != nil {
		return idwrap.IDWrap{}, false, err
	} else if ok {
		return ex, true, nil
	}
	if ex, ok, err := s.ovs.ResolveExampleByOrderRefID(ctx, bodyID); err != nil {
		return idwrap.IDWrap{}, false, err
	} else if ok {
		return ex, true, nil
	}
	return idwrap.IDWrap{}, false, nil
}
