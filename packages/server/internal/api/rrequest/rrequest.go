// Package rrequest implements the request RPC service with delta functionality
//
// Delta System Overview:
// The delta system allows versioning of API examples and their components (queries, headers, asserts).
// It tracks modifications while maintaining references to original values.
//
// Key Concepts:
// 1. Origin Example: The base version with no VersionParentID
// 2. Delta Example: A versioned copy with VersionParentID pointing to the origin
// 3. DeltaParentID: Links items in delta examples to their origin counterparts
//
// Source Types (How items are displayed in the frontend):
// - ORIGIN: Unmodified items (shown as inherited)
// - MIXED: Modified items with local changes (shown as customized deltas)
// - DELTA: New items created only in the delta example (no parent)
//
// The frontend interprets these states:
// - ORIGIN items show they inherit from the parent (grayed out, read-only feel)
// - MIXED items show they've been customized (highlighted as modified)
// - DELTA items show they're new additions (marked as new)
//
// Implementation Details:
// The DetermineDeltaType function returns:
// - ORIGIN: Items without DeltaParentID in origin examples
// - MIXED: Items with DeltaParentID in origin examples
// - DELTA: All items in delta examples (with or without DeltaParentID)
//
// For delta examples, the system cannot distinguish between:
// - Unmodified items that inherit from parent (conceptually ORIGIN)
// - Modified items with local changes (conceptually MIXED)
// Both return DELTA because DetermineDeltaType only looks at structure.
//
// The frontend is responsible for visual differentiation of these states.
// Test note: The test expects DELTA for modified items (current behavior),
// though conceptually MIXED might be more intuitive.
//
// Update Flow:
// 1. Delta example created → All items copied with DeltaParentID, source=ORIGIN
// 2. User modifies item → Source remains DELTA (but conceptually is modified)
// 3. User resets item → Values restored from parent (still DELTA structurally)
// 4. Origin item updated → Propagates to unmodified items only
// 5. Origin item deleted → Cascades to all related delta items
//
// This design allows tracking what's been modified while maintaining inheritance.
package rrequest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/internal/service/assertiondelta"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	overcore "the-dev-tools/server/pkg/overlay/core"
	orank "the-dev-tools/server/pkg/overlay/rank"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	soverlayheader "the-dev-tools/server/pkg/service/soverlayheader"
	soverlayquery "the-dev-tools/server/pkg/service/soverlayquery"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/tassert"
	"the-dev-tools/server/pkg/translate/tcondition"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"the-dev-tools/server/pkg/translate/theader"
	"the-dev-tools/server/pkg/translate/tquery"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/request/v1/requestv1connect"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
)

type RequestRPC struct {
	DB   *sql.DB
	cs   scollection.CollectionService
	us   suser.UserService
	iaes sitemapiexample.ItemApiExampleService
	ias  sitemapi.ItemApiService

	// Sub
	ehs sexampleheader.HeaderService
	eqs sexamplequery.ExampleQueryService

	// Assert
	as sassert.AssertService

	// overlay services
	hov *soverlayheader.Service
	qov *soverlayquery.Service
}

// --- Overlay header adapters (minimal) ---
type headerOrderStore struct{ s *soverlayheader.Service }
type headerStateStore struct{ s *soverlayheader.Service }
type headerDeltaStore struct{ s *soverlayheader.Service }

func (o headerOrderStore) Count(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	return o.s.Count(ctx, ex)
}
func (o headerOrderStore) SelectAsc(ctx context.Context, ex idwrap.IDWrap) ([]overcore.OrderRow, error) {
	rows, err := o.s.SelectAsc(ctx, ex)
	if err != nil {
		return nil, err
	}
	out := make([]overcore.OrderRow, 0, len(rows))
	for _, r := range rows {
		id, err := idwrap.NewFromBytes(r.RefID)
		if err != nil {
			return nil, err
		}
		out = append(out, overcore.OrderRow{RefKind: r.RefKind, RefID: id, Rank: r.Rank})
	}
	return out, nil
}
func (o headerOrderStore) LastRank(ctx context.Context, ex idwrap.IDWrap) (string, bool, error) {
	return o.s.LastRank(ctx, ex)
}
func (o headerOrderStore) MaxRevision(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	return o.s.MaxRevision(ctx, ex)
}
func (o headerOrderStore) InsertIgnore(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, rev int64) error {
	return o.s.InsertIgnore(ctx, ex, refKind, refID, rank, rev)
}
func (o headerOrderStore) Upsert(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, rev int64) error {
	return o.s.Upsert(ctx, ex, refKind, refID, rank, rev)
}
func (o headerOrderStore) DeleteByRef(ctx context.Context, ex idwrap.IDWrap, refID idwrap.IDWrap) error {
	return o.s.DeleteByRef(ctx, ex, refID)
}
func (o headerOrderStore) Exists(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap) (bool, error) {
	return o.s.Exists(ctx, ex, refKind, refID)
}

func (s headerStateStore) Upsert(ctx context.Context, ex, origin idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error {
	return s.s.UpsertState(ctx, ex, origin, suppressed, key, val, desc, enabled)
}
func (s headerStateStore) Get(ctx context.Context, ex, origin idwrap.IDWrap) (overcore.StateRow, bool, error) {
	r, ok, err := s.s.GetState(ctx, ex, origin)
	if err != nil {
		return overcore.StateRow{}, false, err
	}
	if !ok {
		return overcore.StateRow{}, false, nil
	}
	var kp, vp, dp *string
	var ep *bool
	if r.Key.Valid {
		v := r.Key.String
		kp = &v
	}
	if r.Val.Valid {
		v := r.Val.String
		vp = &v
	}
	if r.Desc.Valid {
		v := r.Desc.String
		dp = &v
	}
	if r.Enabled.Valid {
		v := r.Enabled.Bool
		ep = &v
	}
	return overcore.StateRow{Suppressed: r.Suppressed, Key: kp, Val: vp, Desc: dp, Enabled: ep}, true, nil
}
func (s headerStateStore) ClearOverrides(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.ClearStateOverrides(ctx, ex, origin)
}
func (s headerStateStore) Suppress(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.SuppressState(ctx, ex, origin)
}
func (s headerStateStore) Unsuppress(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.UnsuppressState(ctx, ex, origin)
}

func (d headerDeltaStore) Insert(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return d.s.InsertDelta(ctx, ex, id, key, value, desc, enabled)
}
func (d headerDeltaStore) Update(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return d.s.UpdateDelta(ctx, ex, id, key, value, desc, enabled)
}
func (d headerDeltaStore) Get(ctx context.Context, ex, id idwrap.IDWrap) (string, string, string, bool, bool, error) {
	return d.s.GetDelta(ctx, ex, id)
}
func (d headerDeltaStore) Exists(ctx context.Context, ex, id idwrap.IDWrap) (bool, error) {
	return d.s.ExistsDelta(ctx, ex, id)
}
func (d headerDeltaStore) Delete(ctx context.Context, ex, id idwrap.IDWrap) error {
	return d.s.DeleteDelta(ctx, ex, id)
}

// --- Overlay query adapters (minimal) ---
type queryOrderStore struct{ s *soverlayquery.Service }
type queryStateStore struct{ s *soverlayquery.Service }
type queryDeltaStore struct{ s *soverlayquery.Service }

func (o queryOrderStore) Count(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	return o.s.Count(ctx, ex)
}
func (o queryOrderStore) SelectAsc(ctx context.Context, ex idwrap.IDWrap) ([]overcore.OrderRow, error) {
	rows, err := o.s.SelectAsc(ctx, ex)
	if err != nil {
		return nil, err
	}
	out := make([]overcore.OrderRow, 0, len(rows))
	for _, r := range rows {
		id, err := idwrap.NewFromBytes(r.RefID)
		if err != nil {
			return nil, err
		}
		out = append(out, overcore.OrderRow{RefKind: r.RefKind, RefID: id, Rank: r.Rank})
	}
	return out, nil
}
func (o queryOrderStore) LastRank(ctx context.Context, ex idwrap.IDWrap) (string, bool, error) {
	return o.s.LastRank(ctx, ex)
}
func (o queryOrderStore) MaxRevision(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	return o.s.MaxRevision(ctx, ex)
}
func (o queryOrderStore) InsertIgnore(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, rev int64) error {
	return o.s.InsertIgnore(ctx, ex, refKind, refID, rank, rev)
}
func (o queryOrderStore) Upsert(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, rev int64) error {
	return o.s.Upsert(ctx, ex, refKind, refID, rank, rev)
}
func (o queryOrderStore) DeleteByRef(ctx context.Context, ex idwrap.IDWrap, refID idwrap.IDWrap) error {
	return o.s.DeleteByRef(ctx, ex, refID)
}
func (o queryOrderStore) Exists(ctx context.Context, ex idwrap.IDWrap, refKind int8, refID idwrap.IDWrap) (bool, error) {
	return o.s.Exists(ctx, ex, refKind, refID)
}

func (s queryStateStore) Upsert(ctx context.Context, ex, origin idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error {
	return s.s.UpsertState(ctx, ex, origin, suppressed, key, val, desc, enabled)
}
func (s queryStateStore) Get(ctx context.Context, ex, origin idwrap.IDWrap) (overcore.StateRow, bool, error) {
	r, ok, err := s.s.GetState(ctx, ex, origin)
	if err != nil {
		return overcore.StateRow{}, false, err
	}
	if !ok {
		return overcore.StateRow{}, false, nil
	}
	var kp, vp, dp *string
	var ep *bool
	if r.Key.Valid {
		v := r.Key.String
		kp = &v
	}
	if r.Val.Valid {
		v := r.Val.String
		vp = &v
	}
	if r.Desc.Valid {
		v := r.Desc.String
		dp = &v
	}
	if r.Enabled.Valid {
		v := r.Enabled.Bool
		ep = &v
	}
	return overcore.StateRow{Suppressed: r.Suppressed, Key: kp, Val: vp, Desc: dp, Enabled: ep}, true, nil
}
func (s queryStateStore) ClearOverrides(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.ClearStateOverrides(ctx, ex, origin)
}
func (s queryStateStore) Suppress(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.SuppressState(ctx, ex, origin)
}
func (s queryStateStore) Unsuppress(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.UnsuppressState(ctx, ex, origin)
}

func (d queryDeltaStore) Insert(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return d.s.InsertDelta(ctx, ex, id, key, value, desc, enabled)
}
func (d queryDeltaStore) Update(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return d.s.UpdateDelta(ctx, ex, id, key, value, desc, enabled)
}
func (d queryDeltaStore) Get(ctx context.Context, ex, id idwrap.IDWrap) (string, string, string, bool, bool, error) {
	return d.s.GetDelta(ctx, ex, id)
}
func (d queryDeltaStore) Exists(ctx context.Context, ex, id idwrap.IDWrap) (bool, error) {
	return d.s.ExistsDelta(ctx, ex, id)
}
func (d queryDeltaStore) Delete(ctx context.Context, ex, id idwrap.IDWrap) error {
	return d.s.DeleteDelta(ctx, ex, id)
}

func New(db *sql.DB, cs scollection.CollectionService, us suser.UserService, ias sitemapi.ItemApiService, iaes sitemapiexample.ItemApiExampleService,
	ehs sexampleheader.HeaderService, eqs sexamplequery.ExampleQueryService, as sassert.AssertService,
) RequestRPC {
	return RequestRPC{
		DB:   db,
		cs:   cs,
		us:   us,
		ias:  ias,
		iaes: iaes,
		ehs:  ehs,
		eqs:  eqs,
		as:   as,
		hov:  func() *soverlayheader.Service { s, _ := soverlayheader.New(db); return s }(),
		qov:  func() *soverlayquery.Service { s, _ := soverlayquery.New(db); return s }(),
	}
}

func CreateService(srv RequestRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := requestv1connect.NewRequestServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func CheckOwnerHeader(ctx context.Context, hs sexampleheader.HeaderService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, headerUlid idwrap.IDWrap) (bool, error) {
	header, err := hs.GetHeaderByID(ctx, headerUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, header.ExampleID)
}

func CheckOwnerQuery(ctx context.Context, qs sexamplequery.ExampleQueryService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, queryUlid idwrap.IDWrap) (bool, error) {
	query, err := qs.GetExampleQuery(ctx, queryUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, query.ExampleID)
}

func collectDeltaExampleMetas(ctx context.Context, iaes sitemapiexample.ItemApiExampleService, origin *mitemapiexample.ItemApiExample) ([]assertiondelta.ExampleMeta, error) {
	if origin == nil {
		return nil, nil
	}
	visited := map[string]struct{}{origin.ID.String(): {}}
	metas := make([]assertiondelta.ExampleMeta, 0)
	queue := []idwrap.IDWrap{origin.ID}

	add := func(ex mitemapiexample.ItemApiExample) {
		key := ex.ID.String()
		if _, ok := visited[key]; ok {
			return
		}
		visited[key] = struct{}{}
		metas = append(metas, assertiondelta.ExampleMeta{
			ID:               ex.ID,
			HasVersionParent: ex.VersionParentID != nil,
		})
		queue = append(queue, ex.ID)
	}

	if def, err := iaes.GetDefaultApiExample(ctx, origin.ItemApiID); err == nil && def != nil {
		add(*def)
	} else if err != nil && !errors.Is(err, sitemapiexample.ErrNoItemApiExampleFound) {
		return nil, err
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		children, err := iaes.GetApiExampleByVersionParentID(ctx, current)
		if err != nil {
			if errors.Is(err, sitemapiexample.ErrNoItemApiExampleFound) || errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}

		for _, child := range children {
			add(child)
		}
	}

	return metas, nil
}

// isExampleDelta checks if an example has a VersionParentID (making it a delta example)
// A delta example is a versioned copy of an origin example, used to track modifications
func (c *RequestRPC) isExampleDelta(ctx context.Context, exampleID idwrap.IDWrap) (bool, error) {
	example, err := c.iaes.GetApiExample(ctx, exampleID)
	if err != nil {
		return false, err
	}

	// First check: if example has VersionParentID, it's definitely a delta
	if example.VersionParentID != nil {
		return true, nil
	}

	// Second check: if the example belongs to a hidden endpoint, it might be a delta
	// This handles cases where delta examples are created via API without VersionParentID
	endpoint, err := c.ias.GetItemApi(ctx, example.ItemApiID)
	if err != nil {
		// If we can't get the endpoint, fall back to the original check
		return false, nil
	}

	// Hidden endpoints are typically delta endpoints created for nodes
	// Combined with the fact that we're calling delta APIs on them, this is a strong signal
	if endpoint.Hidden {
		return true, nil
	}

	return false, nil
}

// propagateQueryUpdatesToDeltas finds all delta queries that inherit from the given origin
// and updates them if they haven't been modified (their values match the original values)
func (c *RequestRPC) propagateQueryUpdatesToDeltas(ctx context.Context, originQueryID idwrap.IDWrap, originalQuery, updatedQuery mexamplequery.Query) error {
	// Get the origin example to find delta examples
	originExample, err := c.iaes.GetApiExample(ctx, originalQuery.ExampleID)
	if err != nil {
		return nil // Can't propagate if we can't find the example
	}

	// Find all delta examples that have the origin example as their parent
	deltaExamples, err := c.iaes.GetApiExampleByVersionParentID(ctx, originExample.ID)
	if err != nil {
		return nil // No delta examples found
	}

	// For each delta example, check if it has queries that reference the origin query
	for _, deltaExample := range deltaExamples {
		deltaQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, deltaExample.ID)
		if err != nil {
			continue // Skip this example if we can't get its queries
		}

		for _, deltaQuery := range deltaQueries {
			// Check if this delta query references the origin query
			if deltaQuery.DeltaParentID != nil && deltaQuery.DeltaParentID.Compare(originQueryID) == 0 {
				// Check if the delta query's values match the ORIGINAL values
				// If they do, it means the delta hasn't been modified, so we should update it
				if deltaQuery.QueryKey == originalQuery.QueryKey &&
					deltaQuery.Enable == originalQuery.Enable &&
					deltaQuery.Value == originalQuery.Value &&
					deltaQuery.Description == originalQuery.Description {
					// Update the delta query to match the new origin values
					deltaQuery.QueryKey = updatedQuery.QueryKey
					deltaQuery.Enable = updatedQuery.Enable
					deltaQuery.Value = updatedQuery.Value
					deltaQuery.Description = updatedQuery.Description

					err = c.eqs.UpdateExampleQuery(ctx, deltaQuery)
					if err != nil {
						// Continue with other queries even if one fails
						continue
					}
				}
			}
		}
	}

	return nil
}

// determineHeaderDeltaType determines the delta type for a header based on relationships
//
// Delta Type System:
// The system uses three source types to track item states in versioned examples:
//
// 1. ORIGIN: Item has no modifications from its parent
//   - In original example: Item with no DeltaParentID (standalone item)
//   - In delta example: Item with DeltaParentID that hasn't been modified yet
//   - Frontend shows these as inherited/unmodified items
//
// 2. MIXED: Item has been modified from its parent (contains local changes)
//   - In original example: Item with DeltaParentID (references another item)
//   - In delta example: Item with DeltaParentID that has been modified
//   - Frontend shows these as modified delta items
//   - This is the expected state after updating a delta item
//
// 3. DELTA: Standalone item in a delta example (no parent reference)
//   - Only exists in delta examples
//   - Item created directly in the delta example without a parent
//   - Has no DeltaParentID
//
// Returns the appropriate HeaderSource based on these rules
func (c *RequestRPC) determineHeaderDeltaType(ctx context.Context, header mexampleheader.Header) (mexampleheader.HeaderSource, error) {
	exampleIsDelta, err := c.isExampleDelta(ctx, header.ExampleID)
	if err != nil {
		return mexampleheader.HeaderSourceOrigin, err
	}

	deltaType := header.DetermineDeltaType(exampleIsDelta)
	return deltaType, nil
}

func CheckOwnerAssert(ctx context.Context, as sassert.AssertService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, assertUlid idwrap.IDWrap) (bool, error) {
	assert, err := as.GetAssert(ctx, assertUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, assert.ExampleID)
}

func (c RequestRPC) QueryList(ctx context.Context, req *connect.Request[requestv1.QueryListRequest]) (*connect.Response[requestv1.QueryListResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(
		ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	allQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if example has a version parent
	example, err := c.iaes.GetApiExample(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exampleHasVersionParent := example.VersionParentID != nil

	// Filter to only include origin queries (source = 1)
	var originQueries []mexamplequery.Query
	for _, query := range allQueries {
		deltaType := query.DetermineDeltaType(exampleHasVersionParent)
		if deltaType == mexamplequery.QuerySourceOrigin {
			originQueries = append(originQueries, query)
		}
	}

	rpcQueries := tgeneric.MassConvert(originQueries, tquery.SerializeQueryModelToRPCItem)
	resp := &requestv1.QueryListResponse{
		ExampleId: exID.Bytes(),
		Items:     rpcQueries,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) QueryCreate(ctx context.Context, req *connect.Request[requestv1.QueryCreateRequest]) (*connect.Response[requestv1.QueryCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(
		ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	reqQuery := requestv1.Query{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	query, err := tquery.SerlializeQueryRPCtoModelNoID(&reqQuery, exID)
	if err != nil {
		return nil, err
	}
	queryID := idwrap.NewNow()
	query.ID = queryID

	err = c.eqs.CreateExampleQuery(ctx, query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.QueryCreateResponse{QueryId: queryID.Bytes()}), nil
}

func (c RequestRPC) QueryUpdate(ctx context.Context, req *connect.Request[requestv1.QueryUpdateRequest]) (*connect.Response[requestv1.QueryUpdateResponse], error) {
	reqQuery := requestv1.Query{
		QueryId:     req.Msg.GetQueryId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	query, err := tquery.SerlializeQueryRPCtoModel(&reqQuery, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerQuery(ctx, c.eqs, c.iaes, c.cs, c.us, query.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the original query values before update
	originalQuery, err := c.eqs.GetExampleQuery(ctx, query.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update the origin query
	err = c.eqs.UpdateExampleQuery(ctx, query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Find and update all delta queries that inherit from this origin
	// We need to update queries that:
	// 1. Have this query as their DeltaParentID
	// 2. Have values that match the ORIGINAL values (indicating they haven't been modified)
	// Propagate updates to delta queries
	// This is a best-effort operation - we don't fail the request if propagation fails
	_ = c.propagateQueryUpdatesToDeltas(ctx, query.ID, originalQuery, query)

	return connect.NewResponse(&requestv1.QueryUpdateResponse{}), nil
}

func (c RequestRPC) QueryDelete(ctx context.Context, req *connect.Request[requestv1.QueryDeleteRequest]) (*connect.Response[requestv1.QueryDeleteResponse], error) {
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerQuery(ctx, c.eqs, c.iaes, c.cs, c.us, queryID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the query to check if it's an origin query and get its example ID
	originQuery, err := c.eqs.GetExampleQuery(ctx, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get the example to determine if it has a version parent
	example, err := c.iaes.GetApiExample(ctx, originQuery.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exampleHasVersionParent := example.VersionParentID != nil

	// Determine if this is an origin query
	originDeltaType := originQuery.DetermineDeltaType(exampleHasVersionParent)
	if originDeltaType == mexamplequery.QuerySourceOrigin {
		// Need to find and delete delta queries in OTHER examples that reference this origin
		// Get all examples in the collection to search for delta queries
		examples, err := c.iaes.GetApiExampleByCollection(ctx, example.CollectionID)
		if err == nil {
			for _, ex := range examples {
				// Check all examples (both origin and delta) for queries that reference this one
				deltaQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, ex.ID)
				if err == nil {
					for _, deltaQuery := range deltaQueries {
						if deltaQuery.DeltaParentID != nil &&
							deltaQuery.DeltaParentID.Compare(queryID) == 0 {
							// Delete this delta query that references the origin being deleted
							_ = c.eqs.DeleteExampleQuery(ctx, deltaQuery.ID)
						}
					}
				}
			}
		}

		// ADDITIONAL WORKAROUND: Also check for delta examples that have this example as their parent
		// This is needed because GetApiExampleByCollection may not return all examples
		deltaExamples, err := c.iaes.GetApiExampleByVersionParentID(ctx, example.ID)
		if err == nil {
			for _, deltaExample := range deltaExamples {
				// Check for queries in these delta examples that reference the origin query
				deltaQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, deltaExample.ID)
				if err == nil {
					for _, deltaQuery := range deltaQueries {
						if deltaQuery.DeltaParentID != nil &&
							deltaQuery.DeltaParentID.Compare(queryID) == 0 {
							// Delete this delta query that references the origin being deleted
							_ = c.eqs.DeleteExampleQuery(ctx, deltaQuery.ID)
						}
					}
				}
			}
		}
	}

	// Delete the origin query itself
	err = c.eqs.DeleteExampleQuery(ctx, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.QueryDeleteResponse{}), nil
}

// QueryDeltaExampleCopy copies all queries from an origin example to a delta example
// This implements the "Delta example create" functionality
//
// When creating a delta example, all items from the origin are copied with:
// - DeltaParentID pointing to the origin item
// - Initial source type of ORIGIN (unmodified)
// - Same values as the origin
//
// This allows the delta example to track which items have been modified later
func (c RequestRPC) QueryDeltaExampleCopy(ctx context.Context, originExampleID, deltaExampleID idwrap.IDWrap) error {
	// Check permissions for both examples
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID))
	if rpcErr != nil {
		return rpcErr
	}

	// Get the origin example to determine if it has a version parent
	originExample, err := c.iaes.GetApiExample(ctx, originExampleID)
	if err != nil {
		return err
	}
	originExampleHasVersionParent := originExample.VersionParentID != nil

	// Get all queries from the origin example
	originQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, originExampleID)
	if err != nil {
		return err
	}

	// Create corresponding queries in the delta example
	var deltaQueries []mexamplequery.Query
	for _, originQuery := range originQueries {
		// Only copy origin queries (not mixed or delta queries)
		originDeltaType := originQuery.DetermineDeltaType(originExampleHasVersionParent)
		if originDeltaType == mexamplequery.QuerySourceOrigin {
			deltaQuery := mexamplequery.Query{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				DeltaParentID: &originQuery.ID, // Reference the origin query
				QueryKey:      originQuery.QueryKey,
				Enable:        originQuery.Enable,
				Description:   originQuery.Description,
				Value:         originQuery.Value,
			}
			deltaQueries = append(deltaQueries, deltaQuery)
		}
	}

	// Bulk create all delta queries
	if len(deltaQueries) > 0 {
		err = c.eqs.CreateBulkQuery(ctx, deltaQueries)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c RequestRPC) QueryDeltaDelete(ctx context.Context, req *connect.Request[requestv1.QueryDeltaDeleteRequest]) (*connect.Response[requestv1.QueryDeltaDeleteResponse], error) {
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ex, ok, _ := c.qov.ResolveExampleByDeltaID(ctx, queryID)
	if !ok {
		ex, ok, _ = c.qov.ResolveExampleByOrderRefID(ctx, queryID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve example for delete"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	ord := queryOrderStore{s: c.qov}
	st := queryStateStore{s: c.qov}
	dl := queryDeltaStore{s: c.qov}
	if err := overcore.Delete(ctx, ord, st, dl, ex, queryID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.QueryDeltaDeleteResponse{}), nil
}

// QueryDeltaList returns the combined view of queries in a delta example
//
// This function merges queries from both origin and delta examples to show:
// 1. Delta/Mixed items that have been modified (shown with their current values)
// 2. Origin items that haven't been touched yet (automatically created if missing)
//
// The frontend uses this to display a complete view where:
// - ORIGIN items show they're inherited (no local changes)
// - MIXED items show they've been modified (have local changes)
// - DELTA items show they're new in this version (no parent)
func (c RequestRPC) QueryDeltaList(ctx context.Context, req *connect.Request[requestv1.QueryDeltaListRequest]) (*connect.Response[requestv1.QueryDeltaListResponse], error) {
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	ord := queryOrderStore{s: c.qov}
	if cnt, err := ord.Count(ctx, deltaExampleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if cnt == 0 {
		origin, err := c.eqs.GetExampleQueriesByExampleID(ctx, originExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		r := ""
		for _, q := range origin {
			nr := orank.Between(r, "")
			if r == "" {
				nr = orank.First()
			}
			if err := ord.InsertIgnore(ctx, deltaExampleID, int8(overcore.RefKindOrigin), q.ID, nr, 0); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			r = nr
		}
	}
	st := queryStateStore{s: c.qov}
	dl := queryDeltaStore{s: c.qov}

	originQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, originExampleID)
	if err != nil {
		if errors.Is(err, sexamplequery.ErrNoQueryFound) {
			originQueries = nil
		} else {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	deltaQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, deltaExampleID)
	if err != nil {
		if errors.Is(err, sexamplequery.ErrNoQueryFound) {
			deltaQueries = nil
		} else {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	originQueryByID := make(map[idwrap.IDWrap]mexamplequery.Query, len(originQueries))
	originValuesByID := make(map[idwrap.IDWrap]overcore.Values, len(originQueries))
	for _, q := range originQueries {
		originQueryByID[q.ID] = q
		originValuesByID[q.ID] = overcore.Values{Key: q.QueryKey, Value: q.Value, Description: q.Description, Enabled: q.Enable}
	}

	if len(deltaQueries) > 0 {
		if err := seedMissingQueryStateFromDelta(ctx, st, deltaQueries, originQueryByID, deltaExampleID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	originVals := func(ctx context.Context, ids []idwrap.IDWrap) (map[idwrap.IDWrap]overcore.Values, error) {
		if len(ids) == 0 || len(originValuesByID) == 0 {
			return map[idwrap.IDWrap]overcore.Values{}, nil
		}
		m := make(map[idwrap.IDWrap]overcore.Values, len(ids))
		for _, id := range ids {
			if val, ok := originValuesByID[id]; ok {
				m[id] = val
			}
		}
		return m, nil
	}
	build := func(m overcore.Merged) any {
		var origin *requestv1.Query
		if m.Origin != nil {
			origin = &requestv1.Query{QueryId: m.ID.Bytes(), Key: m.Origin.Key, Enabled: m.Origin.Enabled, Value: m.Origin.Value, Description: m.Origin.Description}
		}
		src := m.Source
		return &requestv1.QueryDeltaListItem{QueryId: m.ID.Bytes(), Key: m.Values.Key, Enabled: m.Values.Enabled, Value: m.Values.Value, Description: m.Values.Description, Origin: origin, Source: &src}
	}
	fetch := func(ctx context.Context, ex idwrap.IDWrap) ([]mexamplequery.Query, error) { return nil, nil }
	extract := func(q mexamplequery.Query) overcore.Values { return overcore.Values{} }
	itemsAny, err := overcore.List[mexamplequery.Query](ctx, ord, st, dl, fetch, extract, originVals, build, deltaExampleID, originExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*requestv1.QueryDeltaListItem, 0, len(itemsAny))
	for _, it := range itemsAny {
		out = append(out, it.(*requestv1.QueryDeltaListItem))
	}
	return connect.NewResponse(&requestv1.QueryDeltaListResponse{ExampleId: deltaExampleID.Bytes(), Items: out}), nil
}

func (c RequestRPC) QueryDeltaCreate(ctx context.Context, req *connect.Request[requestv1.QueryDeltaCreateRequest]) (*connect.Response[requestv1.QueryDeltaCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID)); rpcErr != nil {
		return nil, rpcErr
	}
	ord := queryOrderStore{s: c.qov}
	dl := queryDeltaStore{s: c.qov}
	st := queryStateStore{s: c.qov}
	id, err := overcore.CreateDelta(ctx, ord, dl, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	vals := overcore.Values{Key: req.Msg.GetKey(), Value: req.Msg.GetValue(), Description: req.Msg.GetDescription(), Enabled: req.Msg.GetEnabled()}
	if err := overcore.Update(ctx, st, dl, exID, id, &vals); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.QueryDeltaCreateResponse{QueryId: id.Bytes()}), nil
}

// QueryDeltaUpdate updates a delta query with new values
//
// Important behavior: When a delta item is updated, it transitions from:
// - ORIGIN -> MIXED (if it was unmodified, now has local changes)
// - MIXED -> MIXED (stays mixed with new local changes)
// - DELTA -> DELTA (standalone items remain standalone)
//
// The transition to MIXED is automatic based on the DetermineDeltaType logic
// Frontend uses MIXED state to show this item has been customized
func (c RequestRPC) QueryDeltaUpdate(ctx context.Context, req *connect.Request[requestv1.QueryDeltaUpdateRequest]) (*connect.Response[requestv1.QueryDeltaUpdateResponse], error) {
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ex, ok, _ := c.qov.ResolveExampleByDeltaID(ctx, queryID)
	if !ok {
		ex, ok, _ = c.qov.ResolveExampleByOrderRefID(ctx, queryID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve example for update"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	st := queryStateStore{s: c.qov}
	dl := queryDeltaStore{s: c.qov}
	vals := &overcore.Values{}
	if req.Msg.Key != nil {
		vals.Key = *req.Msg.Key
	}
	if req.Msg.Value != nil {
		vals.Value = *req.Msg.Value
	}
	if req.Msg.Description != nil {
		vals.Description = *req.Msg.Description
	}
	if req.Msg.Enabled != nil {
		vals.Enabled = *req.Msg.Enabled
	}
	if err := overcore.Update(ctx, st, dl, ex, queryID, vals); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.QueryDeltaUpdateResponse{}), nil
}

// QueryDeltaReset resets a delta query to its origin values
//
// Reset behavior:
//   - If item has DeltaParentID: Restores all values from the parent
//     This transitions the item from MIXED -> ORIGIN (removes local changes)
//   - If item has no DeltaParentID: Clears all fields (DELTA items)
//
// This allows users to undo modifications and return to inherited values
func (c RequestRPC) QueryDeltaReset(ctx context.Context, req *connect.Request[requestv1.QueryDeltaResetRequest]) (*connect.Response[requestv1.QueryDeltaResetResponse], error) {
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ex, ok, _ := c.qov.ResolveExampleByDeltaID(ctx, queryID)
	if !ok {
		ex, ok, _ = c.qov.ResolveExampleByOrderRefID(ctx, queryID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve example for reset"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	st := queryStateStore{s: c.qov}
	dl := queryDeltaStore{s: c.qov}
	hasDeltaRow, err := dl.Exists(ctx, ex, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := overcore.Reset(ctx, st, dl, ex, queryID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !hasDeltaRow {
		if err := c.syncQueryDeltaFromOrigin(ctx, ex, queryID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&requestv1.QueryDeltaResetResponse{}), nil
}

func (c RequestRPC) syncQueryDeltaFromOrigin(ctx context.Context, deltaExampleID, originQueryID idwrap.IDWrap) error {
	deltaQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, deltaExampleID)
	if err != nil {
		if errors.Is(err, sexamplequery.ErrNoQueryFound) {
			return nil
		}
		return err
	}
	var deltaQuery mexamplequery.Query
	found := false
	for _, q := range deltaQueries {
		if q.DeltaParentID != nil && q.DeltaParentID.Compare(originQueryID) == 0 {
			deltaQuery = q
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	originQuery, err := c.eqs.GetExampleQuery(ctx, originQueryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, sexamplequery.ErrNoQueryFound) {
			return nil
		}
		return err
	}
	deltaQuery.QueryKey = originQuery.QueryKey
	deltaQuery.Enable = originQuery.Enable
	deltaQuery.Description = originQuery.Description
	deltaQuery.Value = originQuery.Value
	return c.eqs.UpdateExampleQuery(ctx, deltaQuery)
}

func (c RequestRPC) HeaderList(ctx context.Context, req *connect.Request[requestv1.HeaderListRequest]) (*connect.Response[requestv1.HeaderListResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Use the ordered version that traverses the linked list
	allHeaders, err := c.ehs.GetHeaderByExampleIDOrdered(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Filter to only include origin headers
	var originHeaders []mexampleheader.Header
	for _, header := range allHeaders {
		deltaType, err := c.determineHeaderDeltaType(ctx, header)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if deltaType == mexampleheader.HeaderSourceOrigin {
			originHeaders = append(originHeaders, header)
		}
	}

	rpcHeaders := tgeneric.MassConvert(originHeaders, theader.SerializeHeaderModelToRPCItem)
	resp := &requestv1.HeaderListResponse{
		ExampleId: exID.Bytes(),
		Items:     rpcHeaders,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) HeaderCreate(ctx context.Context, req *connect.Request[requestv1.HeaderCreateRequest]) (*connect.Response[requestv1.HeaderCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcHeader := requestv1.Header{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	headerID := idwrap.NewNow()
	var deltaParentIDPtr *idwrap.IDWrap
	header := theader.SerlializeHeaderRPCtoModelNoID(&rpcHeader, exID, deltaParentIDPtr)
	header.ID = headerID

	// Note: Source field removed - delta type is determined dynamically

	// Use AppendHeader to properly add the header to the end of the linked list
	err = c.ehs.AppendHeader(ctx, header)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.HeaderCreateResponse{HeaderId: headerID.Bytes()}), nil
}

func (c RequestRPC) HeaderUpdate(ctx context.Context, req *connect.Request[requestv1.HeaderUpdateRequest]) (*connect.Response[requestv1.HeaderUpdateResponse], error) {
	rpcHeader := requestv1.Header{
		HeaderId:    req.Msg.GetHeaderId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	header, err := theader.SerlializeHeaderRPCtoModel(&rpcHeader, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerHeader(ctx, c.ehs, c.iaes, c.cs, c.us, header.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Update the origin header
	// Note: Source field removed - delta type is determined dynamically
	err = c.ehs.UpdateHeader(ctx, header)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Propagate changes to delta items with "origin" source that reference this header
	originHeader, err := c.ehs.GetHeaderByID(ctx, header.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get all headers from this example to find any that reference this origin header
	allHeaders, err := c.ehs.GetHeaderByExampleID(ctx, originHeader.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update any delta headers that reference this origin header with source="origin"
	for _, deltaHeader := range allHeaders {
		if deltaHeader.DeltaParentID != nil &&
			deltaHeader.DeltaParentID.Compare(header.ID) == 0 {

			deltaType, err := c.determineHeaderDeltaType(ctx, deltaHeader)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if deltaType == mexampleheader.HeaderSourceOrigin {
				// Update the delta header to match the origin
				deltaHeader.HeaderKey = header.HeaderKey
				deltaHeader.Enable = header.Enable
				deltaHeader.Description = header.Description
				deltaHeader.Value = header.Value

				err = c.ehs.UpdateHeader(ctx, deltaHeader)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	}

	return connect.NewResponse(&requestv1.HeaderUpdateResponse{}), nil
}

func (c RequestRPC) HeaderDelete(ctx context.Context, req *connect.Request[requestv1.HeaderDeleteRequest]) (*connect.Response[requestv1.HeaderDeleteResponse], error) {
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerHeader(ctx, c.ehs, c.iaes, c.cs, c.us, headerID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the header to check if it's an origin header and get its example ID
	originHeader, err := c.ehs.GetHeaderByID(ctx, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If this is an origin header, delete all delta items with "origin" or "mixed" source that reference it
	deltaType, err := c.determineHeaderDeltaType(ctx, originHeader)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if deltaType == mexampleheader.HeaderSourceOrigin {
		// Need to find and delete delta headers in OTHER examples that reference this origin
		// Get the example to find the collection
		example, err := c.iaes.GetApiExample(ctx, originHeader.ExampleID)
		if err == nil {
			// Get all examples in the collection to search for delta headers
			examples, err := c.iaes.GetApiExampleByCollection(ctx, example.CollectionID)
			if err == nil {
				for _, ex := range examples {
					// Check all examples for headers that reference this one
					deltaHeaders, err := c.ehs.GetHeaderByExampleID(ctx, ex.ID)
					if err == nil {
						for _, deltaHeader := range deltaHeaders {
							if deltaHeader.DeltaParentID != nil &&
								deltaHeader.DeltaParentID.Compare(headerID) == 0 {
								// Delete this delta header that references the origin being deleted
								_ = c.ehs.DeleteHeader(ctx, deltaHeader.ID)
							}
						}
					}
				}
			}

			// ADDITIONAL WORKAROUND: Also check for delta examples that have this example as their parent
			// This is needed because GetApiExampleByCollection may not return all examples
			deltaExamples, err := c.iaes.GetApiExampleByVersionParentID(ctx, example.ID)
			if err == nil {
				for _, deltaExample := range deltaExamples {
					// Check for headers in these delta examples that reference the origin header
					deltaHeaders, err := c.ehs.GetHeaderByExampleID(ctx, deltaExample.ID)
					if err == nil {
						for _, deltaHeader := range deltaHeaders {
							if deltaHeader.DeltaParentID != nil &&
								deltaHeader.DeltaParentID.Compare(headerID) == 0 {
								// Delete this delta header that references the origin being deleted
								_ = c.ehs.DeleteHeader(ctx, deltaHeader.ID)
							}
						}
					}
				}
			}
		}
	}

	// Delete the origin header itself
	err = c.ehs.DeleteHeader(ctx, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeleteResponse{}), nil
}

// HeaderDeltaExampleCopy copies all headers from an origin example to a delta example
// This implements the "Delta example create" functionality
func (c RequestRPC) HeaderDeltaExampleCopy(ctx context.Context, originExampleID, deltaExampleID idwrap.IDWrap) error {
	// Check permissions for both examples
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID))
	if rpcErr != nil {
		return rpcErr
	}

	// Get all headers from the origin example
	originHeaders, err := c.ehs.GetHeaderByExampleID(ctx, originExampleID)
	if err != nil {
		return err
	}

	// Create corresponding headers in the delta example
	var deltaHeaders []mexampleheader.Header
	for _, originHeader := range originHeaders {
		// Only copy origin headers (not mixed or delta headers)
		deltaType, err := c.determineHeaderDeltaType(ctx, originHeader)
		if err != nil {
			return err
		}
		if deltaType == mexampleheader.HeaderSourceOrigin {
			deltaHeader := mexampleheader.Header{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				DeltaParentID: &originHeader.ID, // Reference the origin header
				HeaderKey:     originHeader.HeaderKey,
				Enable:        originHeader.Enable,
				Description:   originHeader.Description,
				Value:         originHeader.Value,
			}
			deltaHeaders = append(deltaHeaders, deltaHeader)
		}
	}

	// Bulk create all delta headers with proper linked list maintenance
	if len(deltaHeaders) > 0 {
		err = c.ehs.AppendBulkHeader(ctx, deltaHeaders)
		if err != nil {
			return err
		}
	}

	return nil
}

// AssertDeltaExampleCopy copies all asserts from an origin example to a delta example
// This implements the "Delta example create" functionality
func (c RequestRPC) AssertDeltaExampleCopy(ctx context.Context, originExampleID, deltaExampleID idwrap.IDWrap) error {
	// Check permissions for both examples
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID))
	if rpcErr != nil {
		return rpcErr
	}

	// Get the origin example to determine if it has a version parent
	originExample, err := c.iaes.GetApiExample(ctx, originExampleID)
	if err != nil {
		return err
	}
	originExampleHasVersionParent := originExample.VersionParentID != nil

	// Get all asserts from the origin example
	originAsserts, err := c.as.GetAssertByExampleID(ctx, originExampleID)
	if err != nil {
		return err
	}

	// Create corresponding asserts in the delta example
	var deltaAsserts []massert.Assert
	for _, originAssert := range originAsserts {
		// Only copy origin asserts (not mixed or delta asserts)
		originDeltaType := originAssert.DetermineDeltaType(originExampleHasVersionParent)
		if originDeltaType == massert.AssertSourceOrigin {
			deltaAssert := massert.Assert{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				DeltaParentID: &originAssert.ID, // Reference the origin assert
				Condition:     originAssert.Condition,
				Enable:        originAssert.Enable,
				Prev:          originAssert.Prev,
				Next:          originAssert.Next,
			}
			deltaAsserts = append(deltaAsserts, deltaAssert)
		}
	}

	// Bulk create all delta asserts
	if len(deltaAsserts) > 0 {
		err = c.as.CreateBulkAssert(ctx, deltaAsserts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c RequestRPC) HeaderDeltaList(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaListRequest]) (*connect.Response[requestv1.HeaderDeltaListResponse], error) {
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	// Seed if needed
	ord := headerOrderStore{s: c.hov}
	if cnt, err := ord.Count(ctx, deltaExampleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if cnt == 0 {
		origin, err := c.ehs.GetHeaderByExampleIDOrdered(ctx, originExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		r := ""
		for _, h := range origin {
			nr := orank.Between(r, "")
			if r == "" {
				nr = orank.First()
			}
			if err := ord.InsertIgnore(ctx, deltaExampleID, int8(overcore.RefKindOrigin), h.ID, nr, 0); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			r = nr
		}
	}
	st := headerStateStore{s: c.hov}
	dl := headerDeltaStore{s: c.hov}

	originHeaders, err := c.ehs.GetHeaderByExampleID(ctx, originExampleID)
	if err != nil {
		if !errors.Is(err, sexampleheader.ErrNoHeaderFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		originHeaders = nil
	}

	deltaHeaders, err := c.ehs.GetHeaderByExampleID(ctx, deltaExampleID)
	if err != nil {
		if !errors.Is(err, sexampleheader.ErrNoHeaderFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		deltaHeaders = nil
	}

	originHeaderByID := make(map[idwrap.IDWrap]mexampleheader.Header, len(originHeaders))
	originValuesByID := make(map[idwrap.IDWrap]overcore.Values, len(originHeaders))
	for _, h := range originHeaders {
		originHeaderByID[h.ID] = h
		originValuesByID[h.ID] = overcore.Values{Key: h.HeaderKey, Value: h.Value, Description: h.Description, Enabled: h.Enable}
	}

	if len(deltaHeaders) > 0 {
		if err := seedMissingHeaderStateFromDelta(ctx, st, deltaHeaders, originHeaderByID, deltaExampleID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	// origin values loader
	originVals := func(ctx context.Context, ids []idwrap.IDWrap) (map[idwrap.IDWrap]overcore.Values, error) {
		if len(ids) == 0 || len(originValuesByID) == 0 {
			return map[idwrap.IDWrap]overcore.Values{}, nil
		}
		m := make(map[idwrap.IDWrap]overcore.Values, len(ids))
		for _, id := range ids {
			if val, ok := originValuesByID[id]; ok {
				m[id] = val
			}
		}
		return m, nil
	}
	build := func(m overcore.Merged) any {
		var origin *requestv1.Header
		if m.Origin != nil {
			origin = &requestv1.Header{HeaderId: m.ID.Bytes(), Key: m.Origin.Key, Enabled: m.Origin.Enabled, Value: m.Origin.Value, Description: m.Origin.Description}
		}
		src := m.Source
		return &requestv1.HeaderDeltaListItem{HeaderId: m.ID.Bytes(), Key: m.Values.Key, Enabled: m.Values.Enabled, Value: m.Values.Value, Description: m.Values.Description, Origin: origin, Source: &src}
	}
	// Provide no-op fetch/extract (not used by core.List now)
	fetch := func(ctx context.Context, ex idwrap.IDWrap) ([]mexampleheader.Header, error) { return nil, nil }
	extract := func(h mexampleheader.Header) overcore.Values { return overcore.Values{} }
	itemsAny, err := overcore.List[mexampleheader.Header](ctx, ord, st, dl, fetch, extract, originVals, build, deltaExampleID, originExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*requestv1.HeaderDeltaListItem, 0, len(itemsAny))
	for _, it := range itemsAny {
		out = append(out, it.(*requestv1.HeaderDeltaListItem))
	}
	return connect.NewResponse(&requestv1.HeaderDeltaListResponse{ExampleId: deltaExampleID.Bytes(), Items: out}), nil
}

func seedMissingHeaderStateFromDelta(ctx context.Context, st headerStateStore, deltaHeaders []mexampleheader.Header, originHeaders map[idwrap.IDWrap]mexampleheader.Header, deltaExampleID idwrap.IDWrap) error {
	for _, header := range deltaHeaders {
		if header.DeltaParentID == nil {
			continue
		}

		base, ok := originHeaders[*header.DeltaParentID]
		if !ok {
			continue
		}

		state, exists, err := st.Get(ctx, deltaExampleID, base.ID)
		if err != nil {
			return err
		}
		if exists {
			if state.Suppressed {
				continue
			}
			if state.Key != nil || state.Val != nil || state.Desc != nil || state.Enabled != nil {
				continue
			}
		}

		keyPtr := headerStringPtrIfDifferent(base.HeaderKey, header.HeaderKey)
		valPtr := headerStringPtrIfDifferent(base.Value, header.Value)
		descPtr := headerStringPtrIfDifferent(base.Description, header.Description)
		enabledPtr := headerBoolPtrIfDifferent(base.Enable, header.Enable)

		if keyPtr == nil && valPtr == nil && descPtr == nil && enabledPtr == nil {
			continue
		}

		if err := st.Upsert(ctx, deltaExampleID, base.ID, false, keyPtr, valPtr, descPtr, enabledPtr); err != nil {
			return err
		}
	}

	return nil
}

func headerStringPtrIfDifferent(origin, next string) *string {
	if origin == next {
		return nil
	}
	v := next
	return &v
}

func headerBoolPtrIfDifferent(origin, next bool) *bool {
	if origin == next {
		return nil
	}
	v := next
	return &v
}

func seedMissingQueryStateFromDelta(ctx context.Context, st queryStateStore, deltaQueries []mexamplequery.Query, originQueries map[idwrap.IDWrap]mexamplequery.Query, deltaExampleID idwrap.IDWrap) error {
	for _, query := range deltaQueries {
		if query.DeltaParentID == nil {
			continue
		}

		base, ok := originQueries[*query.DeltaParentID]
		if !ok {
			continue
		}

		state, exists, err := st.Get(ctx, deltaExampleID, base.ID)
		if err != nil {
			return err
		}
		if exists {
			if state.Suppressed {
				continue
			}
			if state.Key != nil || state.Val != nil || state.Desc != nil || state.Enabled != nil {
				continue
			}
		}

		keyPtr := headerStringPtrIfDifferent(base.QueryKey, query.QueryKey)
		valPtr := headerStringPtrIfDifferent(base.Value, query.Value)
		descPtr := headerStringPtrIfDifferent(base.Description, query.Description)
		enabledPtr := headerBoolPtrIfDifferent(base.Enable, query.Enable)

		if keyPtr == nil && valPtr == nil && descPtr == nil && enabledPtr == nil {
			continue
		}

		if err := st.Upsert(ctx, deltaExampleID, base.ID, false, keyPtr, valPtr, descPtr, enabledPtr); err != nil {
			return err
		}
	}

	return nil
}

func (c RequestRPC) HeaderDeltaCreate(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaCreateRequest]) (*connect.Response[requestv1.HeaderDeltaCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID)); rpcErr != nil {
		return nil, rpcErr
	}
	// Always create delta-only row and optionally apply provided fields
	ord := headerOrderStore{s: c.hov}
	dl := headerDeltaStore{s: c.hov}
	st := headerStateStore{s: c.hov}
	id, err := overcore.CreateDelta(ctx, ord, dl, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	vals := overcore.Values{Key: req.Msg.GetKey(), Value: req.Msg.GetValue(), Description: req.Msg.GetDescription(), Enabled: req.Msg.GetEnabled()}
	if err := overcore.Update(ctx, st, dl, exID, id, &vals); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeltaCreateResponse{HeaderId: id.Bytes()}), nil
}

func (c RequestRPC) HeaderDeltaUpdate(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaUpdateRequest]) (*connect.Response[requestv1.HeaderDeltaUpdateResponse], error) {
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Resolve example scope
	ex, ok, _ := c.hov.ResolveExampleByDeltaID(ctx, headerID)
	if !ok {
		ex, ok, _ = c.hov.ResolveExampleByOrderRefID(ctx, headerID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve example for update"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	st := headerStateStore{s: c.hov}
	dl := headerDeltaStore{s: c.hov}
	vals := &overcore.Values{}
	if req.Msg.Key != nil {
		vals.Key = *req.Msg.Key
	}
	if req.Msg.Value != nil {
		vals.Value = *req.Msg.Value
	}
	if req.Msg.Description != nil {
		vals.Description = *req.Msg.Description
	}
	if req.Msg.Enabled != nil {
		vals.Enabled = *req.Msg.Enabled
	}
	if err := overcore.Update(ctx, st, dl, ex, headerID, vals); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeltaUpdateResponse{}), nil
}

func (c RequestRPC) HeaderDeltaDelete(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaDeleteRequest]) (*connect.Response[requestv1.HeaderDeltaDeleteResponse], error) {
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ex, ok, _ := c.hov.ResolveExampleByDeltaID(ctx, headerID)
	if !ok {
		ex, ok, _ = c.hov.ResolveExampleByOrderRefID(ctx, headerID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve example for delete"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	ord := headerOrderStore{s: c.hov}
	st := headerStateStore{s: c.hov}
	dl := headerDeltaStore{s: c.hov}
	if err := overcore.Delete(ctx, ord, st, dl, ex, headerID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeltaDeleteResponse{}), nil
}

func (c RequestRPC) HeaderDeltaReset(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaResetRequest]) (*connect.Response[requestv1.HeaderDeltaResetResponse], error) {
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ex, ok, _ := c.hov.ResolveExampleByDeltaID(ctx, headerID)
	if !ok {
		ex, ok, _ = c.hov.ResolveExampleByOrderRefID(ctx, headerID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve example for reset"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	st := headerStateStore{s: c.hov}
	dl := headerDeltaStore{s: c.hov}
	hasDeltaRow, err := dl.Exists(ctx, ex, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := overcore.Reset(ctx, st, dl, ex, headerID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !hasDeltaRow {
		if err := c.syncHeaderDeltaFromOrigin(ctx, ex, headerID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&requestv1.HeaderDeltaResetResponse{}), nil
}

func (c RequestRPC) syncHeaderDeltaFromOrigin(ctx context.Context, deltaExampleID, originHeaderID idwrap.IDWrap) error {
	deltaHeaders, err := c.ehs.GetHeaderByExampleID(ctx, deltaExampleID)
	if err != nil {
		if errors.Is(err, sexampleheader.ErrNoHeaderFound) {
			return nil
		}
		return err
	}
	var deltaHeader mexampleheader.Header
	found := false
	for _, h := range deltaHeaders {
		if h.DeltaParentID != nil && h.DeltaParentID.Compare(originHeaderID) == 0 {
			deltaHeader = h
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	originHeader, err := c.ehs.GetHeaderByID(ctx, originHeaderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, sexampleheader.ErrNoHeaderFound) {
			return nil
		}
		return err
	}
	deltaHeader.HeaderKey = originHeader.HeaderKey
	deltaHeader.Enable = originHeader.Enable
	deltaHeader.Description = originHeader.Description
	deltaHeader.Value = originHeader.Value
	return c.ehs.UpdateHeader(ctx, deltaHeader)
}

func (c RequestRPC) AssertList(ctx context.Context, req *connect.Request[requestv1.AssertListRequest]) (*connect.Response[requestv1.AssertListResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	allAsserts, err := c.as.GetAssertsOrdered(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if example has a version parent
	example, err := c.iaes.GetApiExample(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exampleHasVersionParent := example.VersionParentID != nil

	// Filter to include appropriate asserts based on example type
	var visibleAsserts []massert.Assert
	for _, assert := range allAsserts {
		deltaType := assert.DetermineDeltaType(exampleHasVersionParent)
		// For delta examples (with version parent), include both origin and delta asserts
		// For regular examples, only include origin asserts
		if deltaType == massert.AssertSourceOrigin ||
			(exampleHasVersionParent && deltaType == massert.AssertSourceDelta) {
			visibleAsserts = append(visibleAsserts, assert)
		}
	}

	var rpcAssserts []*requestv1.AssertListItem
	for _, a := range visibleAsserts {
		rpcAssert, err := tassert.SerializeAssertModelToRPCItem(a)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rpcAssserts = append(rpcAssserts, rpcAssert)
	}

	resp := &requestv1.AssertListResponse{
		ExampleId: exID.Bytes(),
		Items:     rpcAssserts,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) AssertCreate(ctx context.Context, req *connect.Request[requestv1.AssertCreateRequest]) (*connect.Response[requestv1.AssertCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcAssert := requestv1.Assert{
		Condition: req.Msg.GetCondition(),
	}

	var deltaParentIDPtr *idwrap.IDWrap
	assert := tassert.SerializeAssertRPCToModelWithoutID(&rpcAssert, exID, deltaParentIDPtr)
	assert.Enable = true
	assert.ID = idwrap.NewNow()
	err = c.as.AppendAssert(ctx, assert)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertCreateResponse{AssertId: assert.ID.Bytes()}), nil
}

func (c RequestRPC) AssertUpdate(ctx context.Context, req *connect.Request[requestv1.AssertUpdateRequest]) (*connect.Response[requestv1.AssertUpdateResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	assertDB, err := c.as.GetAssert(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	originExample, err := c.iaes.GetApiExample(ctx, assertDB.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	deltaMetas, err := collectDeltaExampleMetas(ctx, c.iaes, originExample)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	store := assertiondelta.NewStore(c.as)
	updateInput := assertiondelta.ApplyUpdateInput{
		Origin: assertiondelta.ExampleMeta{
			ID:               originExample.ID,
			HasVersionParent: originExample.VersionParentID != nil,
		},
		Delta:     deltaMetas,
		AssertID:  assertID,
		Condition: tcondition.DeserializeConditionRPCToModel(req.Msg.GetCondition()),
		Enable:    assertDB.Enable,
	}

	if _, err := assertiondelta.ApplyUpdate(ctx, store, updateInput); err != nil {
		if errors.Is(err, assertiondelta.ErrOriginMismatch) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertUpdateResponse{}), nil
}

func (c RequestRPC) AssertDelete(ctx context.Context, req *connect.Request[requestv1.AssertDeleteRequest]) (*connect.Response[requestv1.AssertDeleteResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	assertDB, err := c.as.GetAssert(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	originExample, err := c.iaes.GetApiExample(ctx, assertDB.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	deltaMetas, err := collectDeltaExampleMetas(ctx, c.iaes, originExample)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	store := assertiondelta.NewStore(c.as)
	deleteInput := assertiondelta.ApplyDeleteInput{
		Origin: assertiondelta.ExampleMeta{
			ID:               originExample.ID,
			HasVersionParent: originExample.VersionParentID != nil,
		},
		Delta:    deltaMetas,
		AssertID: assertID,
	}
	if _, err := assertiondelta.ApplyDelete(ctx, store, deleteInput); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertDeleteResponse{}), nil
}

func (c RequestRPC) AssertDeltaList(ctx context.Context, req *connect.Request[requestv1.AssertDeltaListRequest]) (*connect.Response[requestv1.AssertDeltaListResponse], error) {
	// Parse both example IDs
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions for both examples
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Check if delta example has a version parent
	deltaExample, err := c.iaes.GetApiExample(ctx, deltaExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	deltaExampleHasVersionParent := deltaExample.VersionParentID != nil

	// Get asserts from both origin and delta examples
	originAsserts, err := c.as.GetAssertByExampleID(ctx, originExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	deltaAsserts, err := c.as.GetAssertByExampleID(ctx, deltaExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Combine all asserts and build maps for lookup
	allAsserts := append(originAsserts, deltaAsserts...)
	assertMap := make(map[idwrap.IDWrap]massert.Assert)
	originMap := make(map[idwrap.IDWrap]*requestv1.Assert)

	// Build maps
	for _, assert := range allAsserts {
		assertMap[assert.ID] = assert
		rpcAssert, err := tassert.SerializeAssertModelToRPC(assert)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		originMap[assert.ID] = rpcAssert
	}

	// First pass: identify which origins are replaced by delta/mixed asserts
	processedOrigins := make(map[idwrap.IDWrap]bool)
	for _, assert := range allAsserts {
		deltaType := assert.DetermineDeltaType(deltaExampleHasVersionParent)
		if (deltaType == massert.AssertSourceDelta || deltaType == massert.AssertSourceMixed) && assert.DeltaParentID != nil {
			processedOrigins[*assert.DeltaParentID] = true
		}
	}

	// Collect origin asserts that need delta entries created
	var originAssertsNeedingDeltas []massert.Assert
	for _, assert := range originAsserts { // Only check origin asserts
		if !processedOrigins[assert.ID] { // If not already processed by a delta
			originAssertsNeedingDeltas = append(originAssertsNeedingDeltas, assert)
		}
	}

	// Create delta entries for origin asserts that don't have them
	newDeltaAsserts := make(map[idwrap.IDWrap]massert.Assert)
	if len(originAssertsNeedingDeltas) > 0 {
		var deltaAssertsToCreate []massert.Assert
		for _, originAssert := range originAssertsNeedingDeltas {
			deltaAssert := massert.Assert{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				DeltaParentID: &originAssert.ID,
				Condition:     originAssert.Condition,
				Enable:        originAssert.Enable,
				Prev:          originAssert.Prev,
				Next:          originAssert.Next,
			}
			deltaAssertsToCreate = append(deltaAssertsToCreate, deltaAssert)
			newDeltaAsserts[originAssert.ID] = deltaAssert
		}

		// Bulk create the delta asserts
		if len(deltaAssertsToCreate) > 0 {
			err = c.as.CreateBulkAssert(ctx, deltaAssertsToCreate)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			// Add newly created asserts to allAsserts so they're processed in the main loop
			allAsserts = append(allAsserts, deltaAssertsToCreate...)
		}
	}

	// Second pass: create result entries
	var rpcAsserts []*requestv1.AssertDeltaListItem
	for _, assert := range allAsserts {
		// Only include asserts that belong to the delta example
		if assert.ExampleID.Compare(deltaExampleID) != 0 {
			continue
		}

		deltaType := assert.DetermineDeltaType(deltaExampleHasVersionParent)
		if (deltaType == massert.AssertSourceDelta || deltaType == massert.AssertSourceMixed) && assert.DeltaParentID != nil {
			// This is a delta assert - check if it's been modified from its parent
			var origin *requestv1.Assert
			var actualSourceKind deltav1.SourceKind

			if originRPC, exists := originMap[*assert.DeltaParentID]; exists {
				origin = originRPC

				// Compare with parent to determine if modified
				// The parent should be from the origin example, not the delta example
				if parentAssert, exists := assertMap[*assert.DeltaParentID]; exists {
					// For asserts, we need to compare the condition fields
					// Since Condition is a complex type, we'll do a deep comparison
					parentCondition := tcondition.SeralizeConditionModelToRPC(parentAssert.Condition)
					currentCondition := tcondition.SeralizeConditionModelToRPC(assert.Condition)

					conditionsMatch := true
					if (parentCondition == nil && currentCondition != nil) ||
						(parentCondition != nil && currentCondition == nil) {
						conditionsMatch = false
					} else if parentCondition != nil && currentCondition != nil {
						// Compare the condition fields - check if both have comparisons
						if parentCondition.Comparison != nil && currentCondition.Comparison != nil {
							conditionsMatch = parentCondition.Comparison.Expression == currentCondition.Comparison.Expression
						} else {
							conditionsMatch = (parentCondition.Comparison == nil) == (currentCondition.Comparison == nil)
						}
					}

					if conditionsMatch && assert.Enable == parentAssert.Enable {
						// Values match parent - this is an unmodified delta (ORIGIN)
						actualSourceKind = deltav1.SourceKind_SOURCE_KIND_ORIGIN
					} else {
						// Values differ from parent - this is a modified delta (MIXED)
						// deltaType should be AssertSourceMixed when the assert has a DeltaParentID
						// and the example has a version parent
						actualSourceKind = deltav1.SourceKind_SOURCE_KIND_MIXED
					}
				} else {
					// Parent not found, treat as modified
					actualSourceKind = deltaType.ToSourceKind()
				}
			} else {
				// No origin found, use the delta type
				actualSourceKind = deltaType.ToSourceKind()
			}

			// Build the response based on the source kind
			var rpcAssert *requestv1.AssertDeltaListItem
			if actualSourceKind == deltav1.SourceKind_SOURCE_KIND_ORIGIN && origin != nil {
				// For ORIGIN items, use the parent's condition to reflect inheritance
				rpcAssert = &requestv1.AssertDeltaListItem{
					AssertId:  assert.ID.Bytes(),
					Condition: origin.Condition,
					Origin:    origin,
					Source:    &actualSourceKind,
				}
			} else {
				// For DELTA/MIXED items, use the delta's condition
				rpcAssert = &requestv1.AssertDeltaListItem{
					AssertId:  assert.ID.Bytes(),
					Condition: tcondition.SeralizeConditionModelToRPC(assert.Condition),
					Origin:    origin,
					Source:    &actualSourceKind,
				}
			}
			rpcAsserts = append(rpcAsserts, rpcAssert)
		} else if deltaType == massert.AssertSourceMixed {
			// This is already a mixed assert, keep it as is
			var origin *requestv1.Assert
			if assert.DeltaParentID != nil {
				if originRPC, exists := originMap[*assert.DeltaParentID]; exists {
					origin = originRPC
				}
			}

			sourceKind := deltaType.ToSourceKind()
			rpcAssert := &requestv1.AssertDeltaListItem{
				AssertId:  assert.ID.Bytes(),
				Condition: tcondition.SeralizeConditionModelToRPC(assert.Condition),
				Origin:    origin,
				Source:    &sourceKind,
			}
			rpcAsserts = append(rpcAsserts, rpcAssert)
		}
		// Skip origin asserts that have been processed (replaced by delta/mixed)
	}

	// Newly created delta asserts are now processed in the main loop above

	// Sort rpcAsserts by ID, but if it has DeltaParentID use that ID instead
	sort.Slice(rpcAsserts, func(i, j int) bool {
		idI, _ := idwrap.NewFromBytes(rpcAsserts[i].AssertId)
		idJ, _ := idwrap.NewFromBytes(rpcAsserts[j].AssertId)

		// Determine the ID to use for sorting for item i
		sortIDI := idI
		if rpcAsserts[i].Origin != nil && len(rpcAsserts[i].Origin.AssertId) > 0 {
			if parentID, err := idwrap.NewFromBytes(rpcAsserts[i].Origin.AssertId); err == nil {
				sortIDI = parentID
			}
		}

		// Determine the ID to use for sorting for item j
		sortIDJ := idJ
		if rpcAsserts[j].Origin != nil && len(rpcAsserts[j].Origin.AssertId) > 0 {
			if parentID, err := idwrap.NewFromBytes(rpcAsserts[j].Origin.AssertId); err == nil {
				sortIDJ = parentID
			}
		}

		return sortIDI.Compare(sortIDJ) < 0
	})

	resp := &requestv1.AssertDeltaListResponse{
		ExampleId: deltaExampleID.Bytes(),
		Items:     rpcAsserts,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) AssertDeltaCreate(ctx context.Context, req *connect.Request[requestv1.AssertDeltaCreateRequest]) (*connect.Response[requestv1.AssertDeltaCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get origin example ID from request
	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Check permissions for origin example as well
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcAssert := requestv1.Assert{
		Condition: req.Msg.GetCondition(),
	}

	var deltaParentIDPtr *idwrap.IDWrap
	assert := tassert.SerializeAssertRPCToModelWithoutID(&rpcAssert, exID, deltaParentIDPtr)
	assert.Enable = true
	assert.ID = idwrap.NewNow()

	// Check if assert_id is provided in request
	if len(req.Msg.GetAssertId()) > 0 {
		// Assert ID is provided, verify it exists and use as delta parent
		parentAssertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the parent assert exists
		parentAssert, err := c.as.GetAssert(ctx, parentAssertID)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		// Verify parent assert relationship (same logic as headers and queries)
		if parentAssert.ExampleID.Compare(originExampleID) == 0 {
			// Parent belongs to origin example, use it directly
			assert.DeltaParentID = &parentAssertID
		} else {
			// Parent doesn't belong to origin example
			if parentAssert.DeltaParentID != nil {
				// This is a delta assert, use its DeltaParentID
				assert.DeltaParentID = parentAssert.DeltaParentID
			} else {
				// Check if the parent example is related to origin
				parentExample, err := c.iaes.GetApiExample(ctx, parentAssert.ExampleID)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}

				if parentExample.VersionParentID != nil && parentExample.VersionParentID.Compare(originExampleID) == 0 {
					assert.DeltaParentID = &parentAssertID
				} else {
					return nil, connect.NewError(connect.CodeInvalidArgument,
						fmt.Errorf("parent assert does not have a valid relationship to the origin example"))
				}
			}
		}
	}
	// If no assert_id provided, DeltaParentID remains nil (standalone delta)

	err = c.as.AppendAssert(ctx, assert)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertDeltaCreateResponse{AssertId: assert.ID.Bytes()}), nil
}

func (c RequestRPC) AssertDeltaUpdate(ctx context.Context, req *connect.Request[requestv1.AssertDeltaUpdateRequest]) (*connect.Response[requestv1.AssertDeltaUpdateResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the existing assert to check its source
	existingAssert, err := c.as.GetAssert(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Always update the existing assert instead of creating a new one
	existingAssert.Condition = tcondition.DeserializeConditionRPCToModel(req.Msg.GetCondition())

	err = c.as.UpdateAssert(ctx, *existingAssert)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertDeltaUpdateResponse{}), nil
}

func (c RequestRPC) AssertDeltaDelete(ctx context.Context, req *connect.Request[requestv1.AssertDeltaDeleteRequest]) (*connect.Response[requestv1.AssertDeltaDeleteResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.as.DeleteAssert(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.AssertDeltaDeleteResponse{}), nil
}

func (c RequestRPC) AssertDeltaReset(ctx context.Context, req *connect.Request[requestv1.AssertDeltaResetRequest]) (*connect.Response[requestv1.AssertDeltaResetResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the delta assert
	deltaAssert, err := c.as.GetAssert(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If the assert has a parent, restore values from parent
	if deltaAssert.DeltaParentID != nil {
		// Restore values from parent
		parentAssert, err := c.as.GetAssert(ctx, *deltaAssert.DeltaParentID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Restore delta assert fields to match parent and set source to origin
		deltaAssert.Condition = parentAssert.Condition
		deltaAssert.Enable = parentAssert.Enable

		err = c.as.UpdateAssert(ctx, *deltaAssert)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		// If no parent, use the original reset behavior (clear fields)
		err = c.as.ResetAssertDelta(ctx, assertID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&requestv1.AssertDeltaResetResponse{}), nil
}

// TODO: implement move RPC
func (c RequestRPC) QueryMove(ctx context.Context, req *connect.Request[requestv1.QueryMoveRequest]) (*connect.Response[requestv1.QueryMoveResponse], error) {
	return connect.NewResponse(&requestv1.QueryMoveResponse{}), nil
}

// TODO: implement move RPC
func (c RequestRPC) QueryDeltaMove(ctx context.Context, req *connect.Request[requestv1.QueryDeltaMoveRequest]) (*connect.Response[requestv1.QueryDeltaMoveResponse], error) {
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	targetQueryID, err := idwrap.NewFromBytes(req.Msg.GetTargetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	pos := req.Msg.GetPosition()
	if pos == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("position must be specified"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	ord := queryOrderStore{s: c.qov}
	if cnt, err := ord.Count(ctx, deltaExampleID); err == nil && cnt == 0 {
		origin, err := c.eqs.GetExampleQueriesByExampleID(ctx, originExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		r := ""
		for _, q := range origin {
			nr := orank.Between(r, "")
			if r == "" {
				nr = orank.First()
			}
			if err := ord.InsertIgnore(ctx, deltaExampleID, int8(overcore.RefKindOrigin), q.ID, nr, 0); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			r = nr
		}
	}
	after := (pos == resourcesv1.MovePosition_MOVE_POSITION_AFTER)
	if err := overcore.Move(ctx, ord, deltaExampleID, queryID, targetQueryID, after); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.QueryDeltaMoveResponse{}), nil
}

func (c RequestRPC) HeaderMove(ctx context.Context, req *connect.Request[requestv1.HeaderMoveRequest]) (*connect.Response[requestv1.HeaderMoveResponse], error) {
	// Parse request parameters
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid example ID: %w", err))
	}

	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid header ID: %w", err))
	}

	targetHeaderID, err := idwrap.NewFromBytes(req.Msg.GetTargetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target header ID: %w", err))
	}

	position := req.Msg.GetPosition()
	if position == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("position must be specified"))
	}

	// Check permissions for the example
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Prevent moving header relative to itself
	if headerID.Compare(targetHeaderID) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot move header relative to itself"))
	}

	// Determine after/before pointers based on position
	var afterHeaderID, beforeHeaderID *idwrap.IDWrap
	if position == resourcesv1.MovePosition_MOVE_POSITION_AFTER {
		afterHeaderID = &targetHeaderID
	} else {
		beforeHeaderID = &targetHeaderID
	}

	// Use HeaderService to perform the move
	err = c.ehs.MoveHeader(ctx, headerID, afterHeaderID, beforeHeaderID, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to move header: %w", err))
	}

	return connect.NewResponse(&requestv1.HeaderMoveResponse{}), nil
}

func (c RequestRPC) HeaderDeltaMove(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaMoveRequest]) (*connect.Response[requestv1.HeaderDeltaMoveResponse], error) {
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	targetHeaderID, err := idwrap.NewFromBytes(req.Msg.GetTargetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	pos := req.Msg.GetPosition()
	if pos == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("position must be specified"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	// Derive origin example from delta example when available
	var originExampleID *idwrap.IDWrap
	if ex, err := c.iaes.GetApiExample(ctx, deltaExampleID); err == nil && ex.VersionParentID != nil {
		originExampleID = ex.VersionParentID
	}
	// Seed if needed
	ord := headerOrderStore{s: c.hov}
	if cnt, err := ord.Count(ctx, deltaExampleID); err == nil && cnt == 0 {
		// Prefer seeding from origin example if available, otherwise seed from the same example (origin view)
		seedEx := deltaExampleID
		if originExampleID != nil {
			seedEx = *originExampleID
		}
		origin, err := c.ehs.GetHeaderByExampleIDOrdered(ctx, seedEx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		r := ""
		for _, h := range origin {
			nr := orank.Between(r, "")
			if r == "" {
				nr = orank.First()
			}
			if err := ord.InsertIgnore(ctx, deltaExampleID, int8(overcore.RefKindOrigin), h.ID, nr, 0); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			r = nr
		}
	}
	after := (pos == resourcesv1.MovePosition_MOVE_POSITION_AFTER)
	// Map DB delta IDs to their origin parents for overlay move if needed
	if hdr, err := c.ehs.GetHeaderByID(ctx, headerID); err == nil && hdr.DeltaParentID != nil {
		headerID = *hdr.DeltaParentID
	}
	if thdr, err := c.ehs.GetHeaderByID(ctx, targetHeaderID); err == nil && thdr.DeltaParentID != nil {
		targetHeaderID = *thdr.DeltaParentID
	}
	if err := overcore.Move(ctx, ord, deltaExampleID, headerID, targetHeaderID, after); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeltaMoveResponse{}), nil
}
