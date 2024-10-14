package rbody

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/ritemapiexample"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyraw"
	"dev-tools-backend/pkg/service/sbodyform"
	"dev-tools-backend/pkg/service/sbodyraw"
	"dev-tools-backend/pkg/service/sbodyurl"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tbodyform"
	"dev-tools-backend/pkg/translate/tbodyurl"
	"dev-tools-backend/pkg/zstdcompress"
	bodyv1 "dev-tools-services/gen/body/v1"
	"dev-tools-services/gen/body/v1/bodyv1connect"
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
	path, handler := bodyv1connect.NewBodyServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c BodyRPC) CreateBodyForm(ctx context.Context, req *connect.Request[bodyv1.CreateBodyFormRequest]) (*connect.Response[bodyv1.CreateBodyFormResponse], error) {
	bodyData := req.Msg.GetItem()
	if bodyData == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("body form is nil"))
	}

	bodyForm, err := tbodyform.SeralizeFormRPCToModelWithoutID(bodyData)
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

	return connect.NewResponse(&bodyv1.CreateBodyFormResponse{Id: bodyForm.ID.String()}), nil
}

func (c BodyRPC) UpdateBodyForm(ctx context.Context, req *connect.Request[bodyv1.UpdateBodyFormRequest]) (*connect.Response[bodyv1.UpdateBodyFormResponse], error) {
	bodyData := req.Msg.GetItem()
	if bodyData == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("body form is nil"))
	}
	BodyForm, err := tbodyform.SerializeFormRPCtoModel(bodyData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, BodyForm.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no body form found"))
	}
	ok, err = ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, BodyForm.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found"))
	}

	err = c.bfs.UpdateBodyForm(ctx, BodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.UpdateBodyFormResponse{}), nil
}

func (c BodyRPC) DeleteBodyForm(ctx context.Context, req *connect.Request[bodyv1.DeleteBodyFormRequest]) (*connect.Response[bodyv1.DeleteBodyFormResponse], error) {
	ID, err := idwrap.NewWithParse(req.Msg.GetId())
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

	return connect.NewResponse(&bodyv1.DeleteBodyFormResponse{}), nil
}

func (c BodyRPC) CreateBodyUrlEncoded(ctx context.Context, req *connect.Request[bodyv1.CreateBodyUrlEncodedRequest]) (*connect.Response[bodyv1.CreateBodyUrlEncodedResponse], error) {
	bodyData := req.Msg.GetItem()
	if bodyData == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("body form is nil"))
	}
	bodyUrl, err := tbodyurl.SeralizeURLRPCToModelWithoutID(bodyData)
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

	return connect.NewResponse(&bodyv1.CreateBodyUrlEncodedResponse{Id: bodyUrl.ID.String()}), nil
}

func (c BodyRPC) UpdateBodyUrlEncoded(ctx context.Context, req *connect.Request[bodyv1.UpdateBodyUrlEncodedRequest]) (*connect.Response[bodyv1.UpdateBodyUrlEncodedResponse], error) {
	bodyData := req.Msg.GetItem()
	if bodyData == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("body form is nil"))
	}

	bodyURL, err := tbodyurl.SerializeURLRPCtoModel(bodyData)
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

	return connect.NewResponse(&bodyv1.UpdateBodyUrlEncodedResponse{}), nil
}

func (c BodyRPC) DeleteBodyUrlEncoded(ctx context.Context, req *connect.Request[bodyv1.DeleteBodyUrlEncodedRequest]) (*connect.Response[bodyv1.DeleteBodyUrlEncodedResponse], error) {
	ID, err := idwrap.NewWithParse(req.Msg.GetId())
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

	err = c.bues.DeleteBodyURLEncoded(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.DeleteBodyUrlEncodedResponse{}), nil
}

func (c BodyRPC) UpdateBodyRaw(ctx context.Context, req *connect.Request[bodyv1.UpdateBodyRawRequest]) (*connect.Response[bodyv1.UpdateBodyRawResponse], error) {
	exampleID, err := idwrap.NewWithParse(req.Msg.GetExampleId())
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
		Data:         req.Msg.GetBodyBytes(),
	}
	if len(rawBody.Data) > zstdcompress.CompressThreshold {
		rawBody.CompressType = mbodyraw.CompressTypeZstd
		rawBody.Data = zstdcompress.Compress(rawBody.Data)
	}

	err = c.brs.UpdateBodyRawBody(ctx, rawBody)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&bodyv1.UpdateBodyRawResponse{}), nil
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
