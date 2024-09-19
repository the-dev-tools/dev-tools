package rbody

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/internal/api/ritemapiexample"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/service/sbodyform"
	"dev-tools-backend/pkg/service/sbodyurl"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tbodyform"
	"dev-tools-backend/pkg/translate/tbodyurl"
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
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	var options []connect.HandlerOption
	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	service := &BodyRPC{
		DB: db,
	}

	path, handler := bodyv1connect.NewBodyServiceHandler(service, options...)
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

	bodyForm, err := tbodyurl.SerializeURLRPCtoModel(bodyData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	err = c.bues.CreateBodyURLEncoded(ctx, bodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bodyv1.CreateBodyUrlEncodedResponse{Id: bodyForm.ID.String()}), nil
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

	ok, err := CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, bodyURL.ID)
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
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func CheckOwnerBodyForm(ctx context.Context, bfs sbodyform.BodyFormService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, bodyFormUlid idwrap.IDWrap) (bool, error) {
	bodyForm, err := bfs.GetBodyForm(ctx, bodyFormUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, bodyForm.ExampleID)
}
