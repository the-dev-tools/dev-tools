package rbody

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/movable"
	overcore "the-dev-tools/server/pkg/overlay/core"
	orank "the-dev-tools/server/pkg/overlay/rank"
	overlayurlenc "the-dev-tools/server/pkg/overlay/urlenc"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	soverlayform "the-dev-tools/server/pkg/service/soverlayform"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/tbodyform"
	"the-dev-tools/server/pkg/translate/tbodyurl"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"the-dev-tools/server/pkg/zstdcompress"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/body/v1/bodyv1connect"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resource/v1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BodyRPC struct {
	DB *sql.DB

	cs   scollection.CollectionService
	iaes sitemapiexample.ItemApiExampleService
	us   suser.UserService

	bfs  sbodyform.BodyFormService
	bues sbodyurl.BodyURLEncodedService
	brs  sbodyraw.BodyRawService

	// overlay (optional)
	overlay *overlayurlenc.Service
	fov     *soverlayform.Service
}

// --- Overlay form adapters (minimal) ---
type formOrderStore struct{ s *soverlayform.Service }
type formStateStore struct{ s *soverlayform.Service }
type formDeltaStore struct{ s *soverlayform.Service }

type formStateAccessor interface {
	Get(ctx context.Context, ex, origin idwrap.IDWrap) (overcore.StateRow, bool, error)
	Upsert(ctx context.Context, ex, origin idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error
}

func (o formOrderStore) Count(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	return o.s.Count(ctx, ex)
}
func (o formOrderStore) SelectAsc(ctx context.Context, ex idwrap.IDWrap) ([]overcore.OrderRow, error) {
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
func (o formOrderStore) LastRank(ctx context.Context, ex idwrap.IDWrap) (string, bool, error) {
	return o.s.LastRank(ctx, ex)
}
func (o formOrderStore) MaxRevision(ctx context.Context, ex idwrap.IDWrap) (int64, error) {
	return o.s.MaxRevision(ctx, ex)
}
func (o formOrderStore) InsertIgnore(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap, rank string, rev int64) error {
	return o.s.InsertIgnore(ctx, ex, rk, id, rank, rev)
}
func (o formOrderStore) Upsert(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap, rank string, rev int64) error {
	return o.s.Upsert(ctx, ex, rk, id, rank, rev)
}
func (o formOrderStore) DeleteByRef(ctx context.Context, ex idwrap.IDWrap, id idwrap.IDWrap) error {
	return o.s.DeleteByRef(ctx, ex, id)
}
func (o formOrderStore) Exists(ctx context.Context, ex idwrap.IDWrap, rk int8, id idwrap.IDWrap) (bool, error) {
	return o.s.Exists(ctx, ex, rk, id)
}

func (s formStateStore) Upsert(ctx context.Context, ex, origin idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error {
	return s.s.UpsertState(ctx, ex, origin, suppressed, key, val, desc, enabled)
}
func (s formStateStore) Get(ctx context.Context, ex, origin idwrap.IDWrap) (overcore.StateRow, bool, error) {
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
func (s formStateStore) ClearOverrides(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.ClearStateOverrides(ctx, ex, origin)
}
func (s formStateStore) Suppress(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.SuppressState(ctx, ex, origin)
}
func (s formStateStore) Unsuppress(ctx context.Context, ex, origin idwrap.IDWrap) error {
	return s.s.UnsuppressState(ctx, ex, origin)
}

func (d formDeltaStore) Insert(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return d.s.InsertDelta(ctx, ex, id, key, value, desc, enabled)
}
func (d formDeltaStore) Update(ctx context.Context, ex, id idwrap.IDWrap, key, value, desc string, enabled bool) error {
	return d.s.UpdateDelta(ctx, ex, id, key, value, desc, enabled)
}
func (d formDeltaStore) Get(ctx context.Context, ex, id idwrap.IDWrap) (string, string, string, bool, bool, error) {
	return d.s.GetDelta(ctx, ex, id)
}
func (d formDeltaStore) Exists(ctx context.Context, ex, id idwrap.IDWrap) (bool, error) {
	return d.s.ExistsDelta(ctx, ex, id)
}
func (d formDeltaStore) Delete(ctx context.Context, ex, id idwrap.IDWrap) error {
	return d.s.DeleteDelta(ctx, ex, id)
}

func New(db *sql.DB, cs scollection.CollectionService, iaes sitemapiexample.ItemApiExampleService,
	us suser.UserService, bfs sbodyform.BodyFormService, bues sbodyurl.BodyURLEncodedService, brs sbodyraw.BodyRawService,
) BodyRPC {
	return BodyRPC{
		DB: db,
		// root
		cs:   cs,
		iaes: iaes,
		us:   us,
		// body services
		bfs:     bfs,
		bues:    bues,
		brs:     brs,
		overlay: overlayurlenc.New(db, bues),
		fov:     func() *soverlayform.Service { s, _ := soverlayform.New(db); return s }(),
	}
}

func CreateService(srv BodyRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := bodyv1connect.NewBodyServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// isExampleDelta checks if an example has a VersionParentID (making it a delta example)
func (c *BodyRPC) isExampleDelta(ctx context.Context, exampleID idwrap.IDWrap) (bool, error) {
	example, err := c.iaes.GetApiExample(ctx, exampleID)
	if err != nil {
		return false, err
	}
	return example.VersionParentID != nil, nil
}

func (c *BodyRPC) BodyFormList(ctx context.Context, req *connect.Request[bodyv1.BodyFormListRequest]) (*connect.Response[bodyv1.BodyFormListResponse], error) {
	ExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	bodyForms, err := c.bfs.GetBodyFormsByExampleID(ctx, ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcBodyForms := tgeneric.MassConvert(bodyForms, tbodyform.SerializeFormModelToRPCItem)
	return connect.NewResponse(&bodyv1.BodyFormListResponse{
		ExampleId: req.Msg.ExampleId,
		Items:     rpcBodyForms,
	}), nil
}

func (c *BodyRPC) BodyFormCreate(ctx context.Context, req *connect.Request[bodyv1.BodyFormCreateRequest]) (*connect.Response[bodyv1.BodyFormCreateResponse], error) {
	ExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcBody := &bodyv1.BodyForm{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}

	var deltaParentIDPtr *idwrap.IDWrap

	bodyForm, err := tbodyform.SeralizeFormRPCToModelWithoutID(rpcBody, ExampleID, deltaParentIDPtr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyForm.ID = idwrap.NewNow()

	ok, err := ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, bodyForm.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found"))
	}
	err = c.bfs.CreateBodyForm(ctx, bodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyFormCreateResponse{BodyId: bodyForm.ID.Bytes()}), nil
}

func (c *BodyRPC) BodyFormUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyFormUpdateRequest]) (*connect.Response[bodyv1.BodyFormUpdateResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	bodyFormPtr, err := c.bfs.GetBodyForm(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	bodyForm := *bodyFormPtr

	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, bodyForm.ExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcBody := &bodyv1.BodyForm{
		BodyId:      req.Msg.GetBodyId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	updated, err := tbodyform.SerializeFormRPCtoModel(rpcBody, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	updated.ExampleID = bodyForm.ExampleID

	if err := c.bfs.UpdateBodyForm(ctx, updated); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if example has version parent
	exampleIsDelta, err := c.isExampleDelta(ctx, bodyForm.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Propagate changes to delta bodyforms if this is an origin bodyform
	if bodyForm.DetermineDeltaType(exampleIsDelta) == mbodyform.BodyFormSourceOrigin {
		if err := c.propagateBodyFormChangesToDeltas(ctx, *updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&bodyv1.BodyFormUpdateResponse{}), nil
}

func (c *BodyRPC) BodyFormDelete(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeleteRequest]) (*connect.Response[bodyv1.BodyFormDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	bodyFormPtr, err := c.bfs.GetBodyForm(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	bodyForm := *bodyFormPtr

	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, bodyForm.ExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	if err := c.bfs.DeleteBodyForm(ctx, ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if example has version parent
	exampleIsDelta, err := c.isExampleDelta(ctx, bodyForm.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If deleting an origin bodyform, also delete associated delta bodyforms
	if bodyForm.DetermineDeltaType(exampleIsDelta) == mbodyform.BodyFormSourceOrigin {
		if err := c.deleteDeltaBodyFormsForOrigin(ctx, bodyForm); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&bodyv1.BodyFormDeleteResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedList(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedListRequest]) (*connect.Response[bodyv1.BodyUrlEncodedListResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	// Ordered via service
	bodyURLs, err := c.bues.GetBodyURLEncodedByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(bodyURLs) == 0 {
		bodyURLs, err = c.bues.GetBodyURLEncodedByExampleID(ctx, exampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	items := tgeneric.MassConvert(bodyURLs, tbodyurl.SerializeURLModelToRPCItem)
	return connect.NewResponse(&bodyv1.BodyUrlEncodedListResponse{Items: items, ExampleId: req.Msg.ExampleId}), nil
}

func (c BodyRPC) BodyUrlEncodedCreate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedCreateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedCreateResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyData := &bodyv1.BodyUrlEncoded{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	var deltaParentIDPtr *idwrap.IDWrap

	bodyUrl, err := tbodyurl.SeralizeURLRPCToModelWithoutID(bodyData, exampleID, deltaParentIDPtr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyUrl.ID = idwrap.NewNow()

	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	// Preflight: plan append using Movable planner (verification only)
	repo := c.bues.Repository()
	if _, err := movable.BuildAppendPlanFromRepo(ctx, repo, exampleID, movable.RequestListTypeBodyUrlEncoded, bodyUrl.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = c.bues.CreateBodyURLEncoded(ctx, bodyUrl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Link new row at the tail of the example list via service
	if err := c.bues.AppendAtEnd(ctx, exampleID, bodyUrl.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyUrlEncodedCreateResponse{BodyId: bodyUrl.ID.Bytes()}), nil
}

func (c BodyRPC) BodyUrlEncodedUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedUpdateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedUpdateResponse], error) {
	bodyData := &bodyv1.BodyUrlEncoded{
		BodyId:      req.Msg.GetBodyId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	bodyURL, err := tbodyurl.SerializeURLRPCtoModel(bodyData, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyURLDB, err := c.bues.GetBodyURLEncoded(ctx, bodyURL.ID)
	if err != nil {
		return nil, err
	}
	bodyURL.ExampleID = bodyURLDB.ExampleID
	rpcErr := permcheck.CheckPerm(CheckOwnerBodyUrlEncoded(ctx, c.bues, c.iaes, c.cs, c.us, bodyURL.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, bodyURL.ExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.bues.UpdateBodyURLEncoded(ctx, bodyURL)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyUrlEncodedUpdateResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedDelete(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeleteRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	bodyURLEncodedPtr, err := c.bues.GetBodyURLEncoded(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	bodyURLEncoded := *bodyURLEncodedPtr

	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, bodyURLEncoded.ExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	if err := c.bues.DeleteBodyURLEncoded(ctx, ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: Add delta deletion functionality after code regeneration
	// If deleting an origin body URL encoded, also delete associated delta body URL encodeds
	// if bodyURLEncoded.Source == mbodyurl.BodyURLEncodedSourceOrigin {
	//	if err := c.deleteDeltaBodyURLEncodedForOrigin(ctx, bodyURLEncoded); err != nil {
	//		return nil, connect.NewError(connect.CodeInternal, err)
	//	}
	// }

	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeleteResponse{}), nil
}

func (c BodyRPC) BodyRawGet(ctx context.Context, req *connect.Request[bodyv1.BodyRawGetRequest]) (*connect.Response[bodyv1.BodyRawGetResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	bodyRaw, err := c.brs.GetBodyRawByExampleID(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var bodyRawData []byte
	if bodyRaw.CompressType == compress.CompressTypeNone {
		bodyRawData = bodyRaw.Data
	}
	switch bodyRaw.CompressType {
	case compress.CompressTypeNone:
		bodyRawData = bodyRaw.Data
	case compress.CompressTypeZstd:
		bodyRawData, err = zstdcompress.Decompress(bodyRaw.Data)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case compress.CompressTypeGzip:
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("gzip not supported"))
	}
	return connect.NewResponse(&bodyv1.BodyRawGetResponse{ExampleId: req.Msg.GetExampleId(), Data: bodyRawData}), nil
}

func (c *BodyRPC) BodyRawUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyRawUpdateRequest]) (*connect.Response[bodyv1.BodyRawUpdateResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	bodyRawID, err := c.brs.GetBodyRawByExampleID(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rawBody := mbodyraw.ExampleBodyRaw{
		ID:           bodyRawID.ID,
		CompressType: compress.CompressTypeNone,
		Data:         req.Msg.GetData(),
	}
	if len(rawBody.Data) > zstdcompress.CompressThreshold {
		rawBody.CompressType = compress.CompressTypeZstd
		rawBody.Data = zstdcompress.Compress(rawBody.Data)
	}

	err = c.brs.UpdateBodyRawBody(ctx, rawBody)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyRawUpdateResponse{}), nil
}

func (c *BodyRPC) BodyFormDeltaList(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaListRequest]) (*connect.Response[bodyv1.BodyFormDeltaListResponse], error) {
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
	// seed order if none
	if cnt, err := c.fov.Count(ctx, deltaExampleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if cnt == 0 {
		origin, err := c.bfs.GetBodyFormsByExampleID(ctx, originExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		r := ""
		for _, f := range origin {
			nr := orank.Between(r, "")
			if r == "" {
				nr = orank.First()
			}
			if err := c.fov.InsertIgnore(ctx, deltaExampleID, int8(overcore.RefKindOrigin), f.ID, nr, 0); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			r = nr
		}
	}
	ord := formOrderStore{s: c.fov}
	st := formStateStore{s: c.fov}
	dl := formDeltaStore{s: c.fov}

	originForms, err := c.bfs.GetBodyFormsByExampleID(ctx, originExampleID)
	if err != nil {
		if errors.Is(err, sbodyform.ErrNoBodyFormFound) {
			originForms = nil
		} else {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	deltaForms, err := c.bfs.GetBodyFormsByExampleID(ctx, deltaExampleID)
	if err != nil {
		if errors.Is(err, sbodyform.ErrNoBodyFormFound) {
			deltaForms = nil
		} else {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	originValuesByID := make(map[idwrap.IDWrap]overcore.Values, len(originForms))
	originFormByID := make(map[idwrap.IDWrap]mbodyform.BodyForm, len(originForms))
	for _, f := range originForms {
		originFormByID[f.ID] = f
		originValuesByID[f.ID] = overcore.Values{Key: f.BodyKey, Value: f.Value, Description: f.Description, Enabled: f.Enable}
	}

	if len(deltaForms) > 0 {
		if err := seedMissingFormStateFromDelta(ctx, st, deltaForms, originFormByID, deltaExampleID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	originVals := func(ctx context.Context, ids []idwrap.IDWrap) (map[idwrap.IDWrap]overcore.Values, error) {
		if len(ids) == 0 {
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
		var origin *bodyv1.BodyForm
		if m.Origin != nil {
			origin = &bodyv1.BodyForm{BodyId: m.ID.Bytes(), Key: m.Origin.Key, Enabled: m.Origin.Enabled, Value: m.Origin.Value, Description: m.Origin.Description}
		}
		src := m.Source
		return &bodyv1.BodyFormDeltaListItem{BodyId: m.ID.Bytes(), Key: m.Values.Key, Enabled: m.Values.Enabled, Value: m.Values.Value, Description: m.Values.Description, Origin: origin, Source: &src}
	}
	fetch := func(ctx context.Context, ex idwrap.IDWrap) ([]mbodyform.BodyForm, error) { return nil, nil }
	extract := func(f mbodyform.BodyForm) overcore.Values { return overcore.Values{} }
	itemsAny, err := overcore.List[mbodyform.BodyForm](ctx, ord, st, dl, fetch, extract, originVals, build, deltaExampleID, originExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*bodyv1.BodyFormDeltaListItem, 0, len(itemsAny))
	for _, it := range itemsAny {
		out = append(out, it.(*bodyv1.BodyFormDeltaListItem))
	}
	return connect.NewResponse(&bodyv1.BodyFormDeltaListResponse{ExampleId: deltaExampleID.Bytes(), Items: out}), nil
}

func (c *BodyRPC) BodyFormDeltaCreate(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaCreateRequest]) (*connect.Response[bodyv1.BodyFormDeltaCreateResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	ord := formOrderStore{s: c.fov}
	dl := formDeltaStore{s: c.fov}
	st := formStateStore{s: c.fov}
	id, err := overcore.CreateDelta(ctx, ord, dl, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	vals := overcore.Values{Key: req.Msg.GetKey(), Value: req.Msg.GetValue(), Description: req.Msg.GetDescription(), Enabled: req.Msg.GetEnabled()}
	if err := overcore.Update(ctx, st, dl, exampleID, id, &vals); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyFormDeltaCreateResponse{BodyId: id.Bytes()}), nil
}

func (c *BodyRPC) BodyFormDeltaUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaUpdateRequest]) (*connect.Response[bodyv1.BodyFormDeltaUpdateResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ex, ok, _ := c.fov.ResolveExampleByDeltaID(ctx, ID)
	if !ok {
		ex, ok, _ = c.fov.ResolveExampleByOrderRefID(ctx, ID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for update"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	st := formStateStore{s: c.fov}
	dl := formDeltaStore{s: c.fov}
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
	if err := overcore.Update(ctx, st, dl, ex, ID, vals); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyFormDeltaUpdateResponse{}), nil
}

func (c *BodyRPC) BodyFormDeltaDelete(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaDeleteRequest]) (*connect.Response[bodyv1.BodyFormDeltaDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ex, ok, _ := c.fov.ResolveExampleByDeltaID(ctx, ID)
	if !ok {
		ex, ok, _ = c.fov.ResolveExampleByOrderRefID(ctx, ID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for delete"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	ord := formOrderStore{s: c.fov}
	st := formStateStore{s: c.fov}
	dl := formDeltaStore{s: c.fov}
	if err := overcore.Delete(ctx, ord, st, dl, ex, ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyFormDeltaDeleteResponse{}), nil
}

func (c *BodyRPC) BodyFormDeltaReset(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaResetRequest]) (*connect.Response[bodyv1.BodyFormDeltaResetResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ex, ok, _ := c.fov.ResolveExampleByDeltaID(ctx, ID)
	if !ok {
		ex, ok, _ = c.fov.ResolveExampleByOrderRefID(ctx, ID)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for reset"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	st := formStateStore{s: c.fov}
	dl := formDeltaStore{s: c.fov}
	hasDeltaRow, err := dl.Exists(ctx, ex, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := overcore.Reset(ctx, st, dl, ex, ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !hasDeltaRow {
		if err := c.syncBodyFormDeltaFromOrigin(ctx, ex, ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&bodyv1.BodyFormDeltaResetResponse{}), nil
}

func (c *BodyRPC) syncBodyFormDeltaFromOrigin(ctx context.Context, deltaExampleID, originBodyID idwrap.IDWrap) error {
	bodyForms, err := c.bfs.GetBodyFormsByExampleID(ctx, deltaExampleID)
	if err != nil {
		if errors.Is(err, sbodyform.ErrNoBodyFormFound) {
			return nil
		}
		return err
	}
	var deltaForm mbodyform.BodyForm
	found := false
	for _, form := range bodyForms {
		if form.DeltaParentID != nil && form.DeltaParentID.Compare(originBodyID) == 0 {
			deltaForm = form
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	originForm, err := c.bfs.GetBodyForm(ctx, originBodyID)
	if err != nil {
		if errors.Is(err, sbodyform.ErrNoBodyFormFound) {
			return nil
		}
		return err
	}
	deltaForm.BodyKey = originForm.BodyKey
	deltaForm.Enable = originForm.Enable
	deltaForm.Description = originForm.Description
	deltaForm.Value = originForm.Value
	return c.bfs.UpdateBodyForm(ctx, &deltaForm)
}

func (c *BodyRPC) BodyUrlEncodedDeltaList(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaListRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaListResponse], error) {
	// Parse both example IDs
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Permissions
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	// Overlay: seed + list
	if err := c.overlay.EnsureSeeded(ctx, deltaExampleID, originExampleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	items, err := c.overlay.List(ctx, deltaExampleID, originExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaListResponse{ExampleId: deltaExampleID.Bytes(), Items: items}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaCreate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaCreateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaCreateResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found"))
	}
	id, err := c.overlay.CreateDelta(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var keyPtr, valPtr, descPtr *string
	var enPtr *bool
	if k := req.Msg.GetKey(); k != "" {
		keyPtr = &k
	}
	if v := req.Msg.GetValue(); v != "" {
		valPtr = &v
	}
	if d := req.Msg.GetDescription(); d != "" {
		descPtr = &d
	}
	if req.Msg.GetEnabled() {
		e := true
		enPtr = &e
	}
	if keyPtr != nil || valPtr != nil || descPtr != nil || enPtr != nil {
		if err := c.overlay.Update(ctx, exampleID, id, keyPtr, valPtr, descPtr, enPtr); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaCreateResponse{BodyId: id.Bytes()}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaUpdateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaUpdateResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Overlay only
	var ex idwrap.IDWrap
	if ex2, ok, _ := c.overlay.ResolveExampleForBodyID(ctx, ID); ok {
		ex = ex2
	} else {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for update"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	// Use pointer fields to preserve partial updates
	if err := c.overlay.Update(ctx, ex, ID, req.Msg.Key, req.Msg.Value, req.Msg.Description, req.Msg.Enabled); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaUpdateResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaDelete(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaDeleteRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Overlay only
	ex, ok, _ := c.overlay.ResolveExampleForBodyID(ctx, ID)
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for delete"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil {
		return nil, rpcErr
	}
	if err := c.overlay.Delete(ctx, ex, ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaDeleteResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaReset(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaResetRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaResetResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Overlay only
	ex2, ok2, _ := c.overlay.ResolveExampleForBodyID(ctx, ID)
	if !ok2 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for reset"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex2)); rpcErr != nil {
		return nil, rpcErr
	}
	if err := c.overlay.Reset(ctx, ex2, ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := c.syncBodyUrlEncodedDeltaFromOrigin(ctx, ex2, ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaResetResponse{}), nil
}

func (c *BodyRPC) syncBodyUrlEncodedDeltaFromOrigin(ctx context.Context, deltaExampleID, originBodyID idwrap.IDWrap) error {
	items, err := c.bues.GetBodyURLEncodedByExampleID(ctx, deltaExampleID)
	if err != nil {
		if errors.Is(err, sbodyurl.ErrNoBodyUrlEncodedFound) || errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	var deltaItem mbodyurl.BodyURLEncoded
	found := false
	for _, item := range items {
		if item.DeltaParentID != nil && item.DeltaParentID.Compare(originBodyID) == 0 {
			deltaItem = item
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	originItem, err := c.bues.GetBodyURLEncoded(ctx, originBodyID)
	if err != nil {
		if errors.Is(err, sbodyurl.ErrNoBodyUrlEncodedFound) || errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	deltaItem.BodyKey = originItem.BodyKey
	deltaItem.Enable = originItem.Enable
	deltaItem.Description = originItem.Description
	deltaItem.Value = originItem.Value
	return c.bues.UpdateBodyURLEncoded(ctx, &deltaItem)
}

func CheckOwnerBodyForm(ctx context.Context, bfs sbodyform.BodyFormService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, bodyFormUlid idwrap.IDWrap) (bool, error) {
	bodyForm, err := bfs.GetBodyForm(ctx, bodyFormUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, bodyForm.ExampleID)
}

func CheckOwnerBodyUrlEncoded(ctx context.Context, bues sbodyurl.BodyURLEncodedService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, bodyUrlUlid idwrap.IDWrap) (bool, error) {
	bodyUrl, err := bues.GetBodyURLEncoded(ctx, bodyUrlUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, bodyUrl.ExampleID)
}

func seedMissingFormStateFromDelta(ctx context.Context, st formStateAccessor, deltaForms []mbodyform.BodyForm, originForms map[idwrap.IDWrap]mbodyform.BodyForm, deltaExampleID idwrap.IDWrap) error {
	for _, form := range deltaForms {
		if form.DeltaParentID == nil {
			continue
		}

		base, ok := originForms[*form.DeltaParentID]
		if !ok {
			continue
		}

		existing, exists, err := st.Get(ctx, deltaExampleID, base.ID)
		if err != nil {
			return err
		}
		if exists {
			if existing.Suppressed {
				continue
			}
			if existing.Key != nil || existing.Val != nil || existing.Desc != nil || existing.Enabled != nil {
				continue
			}
		}

		keyPtr := formStringPtrIfDifferent(base.BodyKey, form.BodyKey)
		valPtr := formStringPtrIfDifferent(base.Value, form.Value)
		descPtr := formStringPtrIfDifferent(base.Description, form.Description)
		enabledPtr := formBoolPtrIfDifferent(base.Enable, form.Enable)

		if keyPtr == nil && valPtr == nil && descPtr == nil && enabledPtr == nil {
			continue
		}

		if err := st.Upsert(ctx, deltaExampleID, base.ID, false, keyPtr, valPtr, descPtr, enabledPtr); err != nil {
			return err
		}
	}

	return nil
}

func formStringPtrIfDifferent(origin, next string) *string {
	if origin == next {
		return nil
	}
	v := next
	return &v
}

func formBoolPtrIfDifferent(origin, next bool) *bool {
	if origin == next {
		return nil
	}
	v := next
	return &v
}

// Helper functions
func (s *BodyRPC) propagateBodyFormChangesToDeltas(ctx context.Context, originBodyForm mbodyform.BodyForm) error {
	// Find all delta bodyforms that reference this origin bodyform
	deltaBodyForms, err := s.bfs.GetBodyFormsByDeltaParentID(ctx, originBodyForm.ID)
	if err != nil {
		return err
	}

	for _, deltaBodyForm := range deltaBodyForms {
		// Check if example has version parent
		exampleIsDelta, err := s.isExampleDelta(ctx, deltaBodyForm.ExampleID)
		if err != nil {
			return err
		}

		// Determine the delta type for this body form
		deltaType := deltaBodyForm.DetermineDeltaType(exampleIsDelta)

		// Only update if it's still a delta (not mixed)
		if deltaType == mbodyform.BodyFormSourceDelta {
			deltaBodyForm.BodyKey = originBodyForm.BodyKey
			deltaBodyForm.Value = originBodyForm.Value
			deltaBodyForm.Enable = originBodyForm.Enable

			if err := s.bfs.UpdateBodyForm(ctx, &deltaBodyForm); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *BodyRPC) deleteDeltaBodyFormsForOrigin(ctx context.Context, originBodyForm mbodyform.BodyForm) error {
	deltaBodyForms, err := s.bfs.GetBodyFormsByDeltaParentID(ctx, originBodyForm.ID)
	if err != nil {
		return err
	}

	for _, deltaBodyForm := range deltaBodyForms {
		if err := s.bfs.DeleteBodyForm(ctx, deltaBodyForm.ID); err != nil {
			return err
		}
	}

	return nil
}

// TODO: implement move RPC
func (c *BodyRPC) BodyFormMove(ctx context.Context, req *connect.Request[bodyv1.BodyFormMoveRequest]) (*connect.Response[emptypb.Empty], error) {
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// TODO: implement move RPC
func (c *BodyRPC) BodyFormDeltaMove(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaMoveRequest]) (*connect.Response[emptypb.Empty], error) {
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	targetID, err := idwrap.NewFromBytes(req.Msg.GetTargetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	pos := req.Msg.GetPosition()
	if pos == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	if cnt, err := c.fov.Count(ctx, deltaExampleID); err == nil && cnt == 0 {
		origin, err := c.bfs.GetBodyFormsByExampleID(ctx, originExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		r := ""
		for _, f := range origin {
			nr := orank.Between(r, "")
			if r == "" {
				nr = orank.First()
			}
			if err := c.fov.InsertIgnore(ctx, deltaExampleID, int8(overcore.RefKindOrigin), f.ID, nr, 0); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			r = nr
		}
	}
	after := pos == resourcesv1.MovePosition_MOVE_POSITION_AFTER
	ord := formOrderStore{s: c.fov}
	if err := overcore.Move(ctx, ord, deltaExampleID, bodyID, targetID, after); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// TODO: implement move RPC
func (c *BodyRPC) BodyUrlEncodedMove(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedMoveRequest]) (*connect.Response[emptypb.Empty], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	targetID, err := idwrap.NewFromBytes(req.Msg.GetTargetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	pos := req.Msg.GetPosition()
	if pos == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	if bodyID.Compare(targetID) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}
	// Use Movable repository directly: compute desired index outside TX and apply inside
	repo := c.bues.Repository()
	items, err := repo.GetItemsByParent(ctx, exampleID, movable.RequestListTypeBodyUrlEncoded)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// find target index
	targetIdx := -1
	for i, it := range items {
		if it.ID.Compare(targetID) == 0 {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target not found"))
	}
	desired := targetIdx
	if pos == resourcesv1.MovePosition_MOVE_POSITION_AFTER {
		desired = targetIdx + 1
	}
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer func() { _ = tx.Rollback() }()
	if txRepo, ok := any(repo).(interface {
		TX(tx *sql.Tx) movable.MovableRepository
	}); ok {
		repo = txRepo.TX(tx).(*sbodyurl.BodyUrlEncodedMovableRepository)
	}
	if err := repo.UpdatePosition(ctx, tx, bodyID, movable.RequestListTypeBodyUrlEncoded, desired); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// TODO: implement move RPC
func (c *BodyRPC) BodyUrlEncodedDeltaMove(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaMoveRequest]) (*connect.Response[emptypb.Empty], error) {
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	targetID, err := idwrap.NewFromBytes(req.Msg.GetTargetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	pos := req.Msg.GetPosition()
	if pos == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified"))
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID)); rpcErr != nil {
		return nil, rpcErr
	}
	// Overlay only
	if bodyID.Compare(targetID) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}
	if err := c.overlay.EnsureSeeded(ctx, deltaExampleID, originExampleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	after := pos == resourcesv1.MovePosition_MOVE_POSITION_AFTER
	if err := c.overlay.Move(ctx, deltaExampleID, originExampleID, bodyID, targetID, after); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// URL-encoded ordering is handled fully in sbodyurl service.
