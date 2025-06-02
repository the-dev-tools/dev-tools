package rbody

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/tbodyform"
	"the-dev-tools/server/pkg/translate/tbodyurl"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"the-dev-tools/server/pkg/zstdcompress"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
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
	}
}

func CreateService(srv BodyRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := bodyv1connect.NewBodyServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
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

func (c BodyRPC) BodyFormCreate(ctx context.Context, req *connect.Request[bodyv1.BodyFormCreateRequest]) (*connect.Response[bodyv1.BodyFormCreateResponse], error) {
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

func (c BodyRPC) BodyFormUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyFormUpdateRequest]) (*connect.Response[bodyv1.BodyFormUpdateResponse], error) {
	rpcBody := &bodyv1.BodyForm{
		BodyId:      req.Msg.GetBodyId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	BodyForm, err := tbodyform.SerializeFormRPCtoModel(rpcBody, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	bodyFormDB, err := c.bfs.GetBodyForm(ctx, BodyForm.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	BodyForm.ExampleID = bodyFormDB.ExampleID

	rpcErr := permcheck.CheckPerm(CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, BodyForm.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.bfs.UpdateBodyForm(ctx, BodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyFormUpdateResponse{}), nil
}

func (c BodyRPC) BodyFormDelete(ctx context.Context, req *connect.Request[bodyv1.BodyFormDeleteRequest]) (*connect.Response[bodyv1.BodyFormDeleteResponse], error) {
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
	bodyURLs, err := c.bues.GetBodyURLEncodedByExampleID(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcBodyURLs := tgeneric.MassConvert(bodyURLs, tbodyurl.SerializeURLModelToRPCItem)
	return connect.NewResponse(&bodyv1.BodyUrlEncodedListResponse{Items: rpcBodyURLs, ExampleId: req.Msg.ExampleId}), nil
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
	err = c.bues.CreateBodyURLEncoded(ctx, bodyUrl)
	if err != nil {
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

func (c BodyRPC) BodyUrlEncodedDelete(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeleteRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerBodyUrlEncoded(ctx, c.bues, c.iaes, c.cs, c.us, ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.bues.DeleteBodyURLEncoded(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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
	return connect.NewResponse(&bodyv1.BodyRawGetResponse{Data: bodyRawData}), nil
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
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	bodyForms, err := c.bfs.GetBodyFormsByExampleID(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcBodyForms := tgeneric.MassConvert(bodyForms, tbodyform.SerializeFormModelToRPCDeltaItem)
	return connect.NewResponse(&bodyv1.BodyFormDeltaListResponse{
		ExampleId: req.Msg.GetExampleId(),
		Items:     rpcBodyForms,
	}), nil
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
	rpcBody := &bodyv1.BodyForm{
		BodyId:      req.Msg.GetBodyId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	bodyForm, err := tbodyform.SerializeFormRPCtoModel(rpcBody, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	bodyFormDB, err := c.bfs.GetBodyForm(ctx, bodyForm.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	bodyForm.ExampleID = bodyFormDB.ExampleID

	rpcErr := permcheck.CheckPerm(CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, bodyForm.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.bfs.UpdateBodyForm(ctx, bodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
	rpcErr := permcheck.CheckPerm(CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.bfs.ResetBodyFormDelta(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyFormDeltaResetResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaList(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaListRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaListResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	bodyURLs, err := c.bues.GetBodyURLEncodedByExampleID(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcBodyURLs := tgeneric.MassConvert(bodyURLs, tbodyurl.SerializeURLModelToRPCDeltaItem)
	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaListResponse{
		ExampleId: req.Msg.GetExampleId(),
		Items:     rpcBodyURLs,
	}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaCreate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaCreateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaCreateResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcBody := &bodyv1.BodyUrlEncoded{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}

	var deltaParentIDPtr *idwrap.IDWrap

	bodyUrl, err := tbodyurl.SeralizeURLRPCToModelWithoutID(rpcBody, exampleID, deltaParentIDPtr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyUrl.ID = idwrap.NewNow()

	ok, err := ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found"))
	}
	err = c.bues.CreateBodyURLEncoded(ctx, bodyUrl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaCreateResponse{BodyId: bodyUrl.ID.Bytes()}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaUpdateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaUpdateResponse], error) {
	rpcBody := &bodyv1.BodyUrlEncoded{
		BodyId:      req.Msg.GetBodyId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	bodyUrl, err := tbodyurl.SerializeURLRPCtoModel(rpcBody, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyUrlDB, err := c.bues.GetBodyURLEncoded(ctx, bodyUrl.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	bodyUrl.ExampleID = bodyUrlDB.ExampleID
	rpcErr := permcheck.CheckPerm(CheckOwnerBodyUrlEncoded(ctx, c.bues, c.iaes, c.cs, c.us, bodyUrl.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.bues.UpdateBodyURLEncoded(ctx, bodyUrl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyUrlEncodedDeltaUpdateResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedDeltaDelete(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedDeltaDeleteRequest]) (*connect.Response[bodyv1.BodyUrlEncodedDeltaDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
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
