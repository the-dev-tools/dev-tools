package rbody

import (
    "context"
    "database/sql"
    "errors"
    "sort"
    "the-dev-tools/server/internal/api"
    "the-dev-tools/server/internal/api/ritemapiexample"
    "the-dev-tools/server/pkg/compress"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/movable"
    "the-dev-tools/server/pkg/model/mbodyform"
    "the-dev-tools/server/pkg/model/mbodyraw"
    "the-dev-tools/server/pkg/permcheck"
    "the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/suser"
    overlayurlenc "the-dev-tools/server/pkg/overlay/urlenc"
	"the-dev-tools/server/pkg/translate/tbodyform"
	"the-dev-tools/server/pkg/translate/tbodyurl"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"the-dev-tools/server/pkg/zstdcompress"
    bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
    resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
    "the-dev-tools/spec/dist/buf/go/collection/item/body/v1/bodyv1connect"

	"connectrpc.com/connect"
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
        bfs:  bfs,
        bues: bues,
        brs:  brs,
        overlay: overlayurlenc.New(db, bues),
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
        if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
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
	// Get body forms from both origin and delta examples
	originBodyForms, err := c.bfs.GetBodyFormsByExampleID(ctx, originExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	deltaBodyForms, err := c.bfs.GetBodyFormsByExampleID(ctx, deltaExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if delta example has version parent
	deltaExampleIsDelta, err := c.isExampleDelta(ctx, deltaExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Combine all body forms and build maps for lookup
	allBodyForms := append(originBodyForms, deltaBodyForms...)
	bodyFormMap := make(map[idwrap.IDWrap]mbodyform.BodyForm)
	originMap := make(map[idwrap.IDWrap]*bodyv1.BodyForm)
	replacedOrigins := make(map[idwrap.IDWrap]bool)

	for _, bodyForm := range allBodyForms {
		bodyFormMap[bodyForm.ID] = bodyForm
		originMap[bodyForm.ID] = tbodyform.SerializeFormModelToRPC(bodyForm)

		// Determine the delta type for this body form
		exampleIsDelta := bodyForm.ExampleID.Compare(deltaExampleID) == 0 && deltaExampleIsDelta
		deltaType := bodyForm.DetermineDeltaType(exampleIsDelta)

		// Mark origin body forms that have been replaced by mixed body forms
		if deltaType == mbodyform.BodyFormSourceMixed && bodyForm.DeltaParentID != nil {
			replacedOrigins[*bodyForm.DeltaParentID] = true
		}
	}

	// Create result slice maintaining order from allBodyForms
	var rpcBodyForms []*bodyv1.BodyFormDeltaListItem
	for _, bodyForm := range allBodyForms {
		// Determine the delta type for this body form
		exampleIsDelta := bodyForm.ExampleID.Compare(deltaExampleID) == 0 && deltaExampleIsDelta
		deltaType := bodyForm.DetermineDeltaType(exampleIsDelta)

		// Skip origin body forms that have been replaced by mixed body forms
		if deltaType == mbodyform.BodyFormSourceOrigin && replacedOrigins[bodyForm.ID] {
			continue
		}

		sourceKind := deltaType.ToSourceKind()
		var origin *bodyv1.BodyForm
		var key, value, description string
		var enabled bool

		if deltaType == mbodyform.BodyFormSourceOrigin {
			// For origin items, put the data in origin field and leave main fields empty
			origin = tbodyform.SerializeFormModelToRPC(bodyForm)
			key = ""
			value = ""
			description = ""
			enabled = false
		} else {
			// For delta/mixed items, use the current values and find the origin if it has a parent
			key = bodyForm.BodyKey
			value = bodyForm.Value
			description = bodyForm.Description
			enabled = bodyForm.Enable

			if bodyForm.DeltaParentID != nil {
				if originRPC, exists := originMap[*bodyForm.DeltaParentID]; exists {
					origin = originRPC
				}
			}
		}

		rpcBodyForm := &bodyv1.BodyFormDeltaListItem{
			BodyId:      bodyForm.ID.Bytes(),
			Key:         key,
			Enabled:     enabled,
			Value:       value,
			Description: description,
			Origin:      origin,
			Source:      &sourceKind,
		}
		rpcBodyForms = append(rpcBodyForms, rpcBodyForm)
	}

	// Sort rpcBodyForms by ID, but if it has DeltaParentID use that ID instead
	sort.Slice(rpcBodyForms, func(i, j int) bool {
		idI, _ := idwrap.NewFromBytes(rpcBodyForms[i].BodyId)
		idJ, _ := idwrap.NewFromBytes(rpcBodyForms[j].BodyId)

		// Determine the ID to use for sorting for item i
		sortIDI := idI
		if rpcBodyForms[i].Origin != nil && len(rpcBodyForms[i].Origin.BodyId) > 0 {
			if parentID, err := idwrap.NewFromBytes(rpcBodyForms[i].Origin.BodyId); err == nil {
				sortIDI = parentID
			}
		}

		// Determine the ID to use for sorting for item j
		sortIDJ := idJ
		if rpcBodyForms[j].Origin != nil && len(rpcBodyForms[j].Origin.BodyId) > 0 {
			if parentID, err := idwrap.NewFromBytes(rpcBodyForms[j].Origin.BodyId); err == nil {
				sortIDJ = parentID
			}
		}

		return sortIDI.Compare(sortIDJ) < 0
	})

	resp := &bodyv1.BodyFormDeltaListResponse{
		ExampleId: deltaExampleID.Bytes(),
		Items:     rpcBodyForms,
	}
	return connect.NewResponse(resp), nil
}

func (c *BodyRPC) BodyFormDeltaCreate(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaCreateRequest]) (*connect.Response[bodyv1.BodyFormDeltaCreateResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
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

	bodyForm, err := tbodyform.SeralizeFormRPCToModelWithoutID(rpcBody, exampleID, deltaParentIDPtr)
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

	return connect.NewResponse(&bodyv1.BodyFormDeltaCreateResponse{BodyId: bodyForm.ID.Bytes()}), nil
}

func (c *BodyRPC) BodyFormDeltaUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaUpdateRequest]) (*connect.Response[bodyv1.BodyFormDeltaUpdateResponse], error) {
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

	// Check if example has version parent
	exampleIsDelta, err := c.isExampleDelta(ctx, bodyForm.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Determine the delta type for this body form
	deltaType := bodyForm.DetermineDeltaType(exampleIsDelta)

	// If it's an origin bodyform, create a mixed bodyform instead
	if deltaType == mbodyform.BodyFormSourceOrigin {
		updated.DeltaParentID = &bodyForm.ID
		updated.ID = idwrap.NewNow()

		if err := c.bfs.CreateBodyForm(ctx, updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		updated.DeltaParentID = bodyForm.DeltaParentID

		if err := c.bfs.UpdateBodyForm(ctx, updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&bodyv1.BodyFormDeltaUpdateResponse{}), nil
}

func (c *BodyRPC) BodyFormDeltaDelete(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaDeleteRequest]) (*connect.Response[bodyv1.BodyFormDeltaDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.bfs.DeleteBodyForm(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyFormDeltaDeleteResponse{}), nil
}

func (c *BodyRPC) BodyFormDeltaReset(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaResetRequest]) (*connect.Response[bodyv1.BodyFormDeltaResetResponse], error) {
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

	if bodyForm.DeltaParentID != nil {
		// Get parent bodyform and restore values
		parentBodyFormPtr, err := c.bfs.GetBodyForm(ctx, *bodyForm.DeltaParentID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		parentBodyForm := *parentBodyFormPtr

		bodyForm.BodyKey = parentBodyForm.BodyKey
		bodyForm.Value = parentBodyForm.Value
		bodyForm.Enable = parentBodyForm.Enable
	} else {
		// Clear the values
		bodyForm.BodyKey = ""
		bodyForm.Value = ""
		bodyForm.Enable = false
	}

	if err := c.bfs.UpdateBodyForm(ctx, &bodyForm); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyFormDeltaResetResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaList(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaListRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaListResponse], error) {
    // Parse both example IDs
    deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    // Permissions
    if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil { return nil, rpcErr }
    if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID)); rpcErr != nil { return nil, rpcErr }
    // Overlay: seed + list
    if err := c.overlay.EnsureSeeded(ctx, deltaExampleID, originExampleID); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    items, err := c.overlay.List(ctx, deltaExampleID, originExampleID)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaListResponse{ ExampleId: deltaExampleID.Bytes(), Items: items }), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaCreate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaCreateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaCreateResponse], error) {
    exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    ok, err := ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    if !ok { return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found")) }
    id, err := c.overlay.CreateDelta(ctx, exampleID)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    var keyPtr, valPtr, descPtr *string
    var enPtr *bool
    if k := req.Msg.GetKey(); k != "" { keyPtr = &k }
    if v := req.Msg.GetValue(); v != "" { valPtr = &v }
    if d := req.Msg.GetDescription(); d != "" { descPtr = &d }
    if req.Msg.GetEnabled() { e := true; enPtr = &e }
    if keyPtr != nil || valPtr != nil || descPtr != nil || enPtr != nil {
        if err := c.overlay.Update(ctx, exampleID, id, keyPtr, valPtr, descPtr, enPtr); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    }
    return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaCreateResponse{ BodyId: id.Bytes() }), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaUpdateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaUpdateResponse], error) {
    ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
    if err != nil {
        return nil, connect.NewError(connect.CodeInvalidArgument, err)
    }
    // Overlay only
    var ex idwrap.IDWrap
    if ex2, ok, _ := c.overlay.ResolveExampleForBodyID(ctx, ID); ok { ex = ex2 } else { return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for update")) }
    if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil { return nil, rpcErr }
    // Use pointer fields to preserve partial updates
    if err := c.overlay.Update(ctx, ex, ID, req.Msg.Key, req.Msg.Value, req.Msg.Description, req.Msg.Enabled); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaUpdateResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaDelete(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaDeleteRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaDeleteResponse], error) {
    ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
    if err != nil {
        return nil, connect.NewError(connect.CodeInvalidArgument, err)
    }
    // Overlay only
    ex, ok, _ := c.overlay.ResolveExampleForBodyID(ctx, ID)
    if !ok { return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for delete")) }
    if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex)); rpcErr != nil { return nil, rpcErr }
    if err := c.overlay.Delete(ctx, ex, ID); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaDeleteResponse{}), nil
    rpcErr := permcheck.CheckPerm(CheckOwnerBodyUrlEncoded(ctx, c.bues, c.iaes, c.cs, c.us, ID))
    if rpcErr != nil {
        return nil, rpcErr
    }
    err = c.bues.DeleteBodyURLEncoded(ctx, ID)
	if err != nil {
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
    if !ok2 { return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot resolve example for reset")) }
    if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, ex2)); rpcErr != nil { return nil, rpcErr }
    if err := c.overlay.Reset(ctx, ex2, ID); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaResetResponse{}), nil
    rpcErr := permcheck.CheckPerm(CheckOwnerBodyUrlEncoded(ctx, c.bues, c.iaes, c.cs, c.us, ID))
    if rpcErr != nil {
        return nil, rpcErr
    }
    err = c.bues.ResetBodyURLEncodedDelta(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaResetResponse{}), nil
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
func (c *BodyRPC) BodyFormMove(ctx context.Context, req *connect.Request[bodyv1.BodyFormMoveRequest]) (*connect.Response[bodyv1.BodyFormMoveResponse], error) {
	return connect.NewResponse(&bodyv1.BodyFormMoveResponse{}), nil
}

// TODO: implement move RPC
func (c *BodyRPC) BodyFormDeltaMove(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeltaMoveRequest]) (*connect.Response[bodyv1.BodyFormDeltaMoveResponse], error) {
	return connect.NewResponse(&bodyv1.BodyFormDeltaMoveResponse{}), nil
}

// TODO: implement move RPC
func (c *BodyRPC) BodyUrlEncodedMove(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedMoveRequest]) (*connect.Response[bodyv1.BodyUrlEncodedMoveResponse], error) {
    exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    bodyID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    targetID, err := idwrap.NewFromBytes(req.Msg.GetTargetBodyId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    pos := req.Msg.GetPosition()
    if pos == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED { return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified")) }
    if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID)); rpcErr != nil { return nil, rpcErr }
    if bodyID.Compare(targetID) == 0 { return connect.NewResponse(&bodyv1.BodyUrlEncodedMoveResponse{}), nil }
    // Use Movable repository directly: compute desired index outside TX and apply inside
    repo := c.bues.Repository()
    items, err := repo.GetItemsByParent(ctx, exampleID, movable.RequestListTypeBodyUrlEncoded)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    // find target index
    targetIdx := -1
    for i, it := range items { if it.ID.Compare(targetID) == 0 { targetIdx = i; break } }
    if targetIdx < 0 { return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target not found")) }
    desired := targetIdx
    if pos == resourcesv1.MovePosition_MOVE_POSITION_AFTER { desired = targetIdx + 1 }
    tx, err := c.DB.BeginTx(ctx, nil)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    defer func(){ _ = tx.Rollback() }()
    if txRepo, ok := any(repo).(interface{ TX(tx *sql.Tx) movable.MovableRepository }); ok {
        repo = txRepo.TX(tx).(*sbodyurl.BodyUrlEncodedMovableRepository)
    }
    if err := repo.UpdatePosition(ctx, tx, bodyID, movable.RequestListTypeBodyUrlEncoded, desired); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    if err := tx.Commit(); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    return connect.NewResponse(&bodyv1.BodyUrlEncodedMoveResponse{}), nil
}

// TODO: implement move RPC
func (c *BodyRPC) BodyUrlEncodedDeltaMove(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaMoveRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaMoveResponse], error) {
    deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    bodyID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    targetID, err := idwrap.NewFromBytes(req.Msg.GetTargetBodyId())
    if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err) }
    pos := req.Msg.GetPosition()
    if pos == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED { return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified")) }
    if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID)); rpcErr != nil { return nil, rpcErr }
    if rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID)); rpcErr != nil { return nil, rpcErr }
    // Overlay only
    if bodyID.Compare(targetID) == 0 { return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaMoveResponse{}), nil }
    if err := c.overlay.EnsureSeeded(ctx, deltaExampleID, originExampleID); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    after := pos == resourcesv1.MovePosition_MOVE_POSITION_AFTER
    if err := c.overlay.Move(ctx, deltaExampleID, originExampleID, bodyID, targetID, after); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaMoveResponse{}), nil
    // Ensure delta proxies via service
    bodyID, err = c.bues.EnsureDeltaProxy(ctx, deltaExampleID, bodyID)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    targetID, err = c.bues.EnsureDeltaProxy(ctx, deltaExampleID, targetID)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    if bodyID.Compare(targetID) == 0 { return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaMoveResponse{}), nil }
    repo := c.bues.Repository()
    items, err := repo.GetItemsByParent(ctx, deltaExampleID, movable.RequestListTypeBodyUrlEncodedDeltas)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    targetIdx := -1
    for i, it := range items { if it.ID.Compare(targetID) == 0 { targetIdx = i; break } }
    if targetIdx < 0 { return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target not found")) }
    desired := targetIdx; if pos == resourcesv1.MovePosition_MOVE_POSITION_AFTER { desired = targetIdx + 1 }
    tx, err := c.DB.BeginTx(ctx, nil)
    if err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    defer func(){ _ = tx.Rollback() }()
    if txRepo, ok := any(repo).(interface{ TX(tx *sql.Tx) movable.MovableRepository }); ok { repo = txRepo.TX(tx).(*sbodyurl.BodyUrlEncodedMovableRepository) }
    if err := repo.UpdatePosition(ctx, tx, bodyID, movable.RequestListTypeBodyUrlEncodedDeltas, desired); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    if err := tx.Commit(); err != nil { return nil, connect.NewError(connect.CodeInternal, err) }
    return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaMoveResponse{}), nil
}

// URL-encoded ordering is handled fully in sbodyurl service.
