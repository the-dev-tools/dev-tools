package rbody

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/ritemapiexample"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyraw"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/sbodyform"
	"dev-tools-backend/pkg/service/sbodyraw"
	"dev-tools-backend/pkg/service/sbodyurl"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tbodyform"
	"dev-tools-backend/pkg/translate/tbodyurl"
	"dev-tools-backend/pkg/zstdcompress"
	bodyv1 "dev-tools-spec/dist/buf/go/collection/item/body/v1"
	"dev-tools-spec/dist/buf/go/collection/item/body/v1/bodyv1connect"
	"errors"

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
	path, handler := bodyv1connect.NewRequestServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *BodyRPC) BodyFormItemList(ctx context.Context, req *connect.Request[bodyv1.BodyFormItemListRequest]) (*connect.Response[bodyv1.BodyFormItemListResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c BodyRPC) BodyFormItemCreate(ctx context.Context, req *connect.Request[bodyv1.BodyFormItemCreateRequest]) (*connect.Response[bodyv1.BodyFormItemCreateResponse], error) {
	ExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcBody := &bodyv1.BodyFormItem{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}

	bodyForm, err := tbodyform.SeralizeFormRPCToModelWithoutID(rpcBody, ExampleID)
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

	return connect.NewResponse(&bodyv1.BodyFormItemCreateResponse{BodyId: bodyForm.ID.Bytes()}), nil
}

func (c BodyRPC) BodyFormItemUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyFormItemUpdateRequest]) (*connect.Response[bodyv1.BodyFormItemUpdateResponse], error) {
	rpcBody := &bodyv1.BodyFormItem{
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

	rpcErr := permcheck.CheckPerm(CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, BodyForm.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.bfs.UpdateBodyForm(ctx, BodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyFormItemUpdateResponse{}), nil
}

func (c BodyRPC) BodyFormItemDelete(ctx context.Context, req *connect.Request[bodyv1.BodyFormItemDeleteRequest]) (*connect.Response[bodyv1.BodyFormItemDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no body form found"))
	}

	err = c.bfs.DeleteBodyForm(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyFormItemDeleteResponse{}), nil
}

func (c *BodyRPC) BodyUrlEncodedItemList(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedItemListRequest]) (*connect.Response[bodyv1.BodyUrlEncodedItemListResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c BodyRPC) BodyUrlEncodedItemCreate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedItemCreateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedItemCreateResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyData := &bodyv1.BodyUrlEncodedItem{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	bodyUrl, err := tbodyurl.SeralizeURLRPCToModelWithoutID(bodyData, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bodyUrl.ID = idwrap.NewNow()

	ok, err := ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, bodyUrl.ExampleID)
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

	return connect.NewResponse(&bodyv1.BodyUrlEncodedItemCreateResponse{BodyId: bodyUrl.ID.Bytes()}), nil
}

func (c BodyRPC) BodyUrlEncodedItemUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedItemUpdateRequest]) (*connect.Response[bodyv1.BodyUrlEncodedItemUpdateResponse], error) {
	bodyData := &bodyv1.BodyUrlEncodedItem{
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
	ok, err := CheckOwnerBodyUrlEncoded(ctx, c.bues, c.iaes, c.cs, c.us, bodyURL.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no body form found"))
	}
	ok, err = ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, bodyURL.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found"))
	}

	err = c.bues.UpdateBodyURLEncoded(ctx, bodyURL)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyUrlEncodedItemUpdateResponse{}), nil
}

func (c BodyRPC) BodyUrlEncodedItemDelete(ctx context.Context, req *connect.Request[bodyv1.BodyUrlEncodedItemDeleteRequest]) (*connect.Response[bodyv1.BodyUrlEncodedItemDeleteResponse], error) {
	ID, err := idwrap.NewFromBytes(req.Msg.GetBodyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.bues.DeleteBodyURLEncoded(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.BodyUrlEncodedItemDeleteResponse{}), nil
}

func (c BodyRPC) BodyRawGet(ctx context.Context, req *connect.Request[bodyv1.BodyRawGetRequest]) (*connect.Response[bodyv1.BodyRawGetResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c BodyRPC) BodyRawUpdate(ctx context.Context, req *connect.Request[bodyv1.BodyRawUpdateRequest]) (*connect.Response[bodyv1.BodyRawUpdateResponse], error) {
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
	bodyRawID, err := c.brs.GetBodyRawByExampleID(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rawBody := mbodyraw.ExampleBodyRaw{
		ID:           bodyRawID.ID,
		CompressType: mbodyraw.CompressTypeNone,
		Data:         req.Msg.GetData(),
	}
	if len(rawBody.Data) > zstdcompress.CompressThreshold {
		rawBody.CompressType = mbodyraw.CompressTypeZstd
		rawBody.Data = zstdcompress.Compress(rawBody.Data)
	}

	err = c.brs.UpdateBodyRawBody(ctx, rawBody)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.BodyRawUpdateResponse{}), nil
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
