package rbody

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/internal/api/ritemapiexample"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyform"
	"dev-tools-backend/pkg/service/sbodyform"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tbodyform"
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

	bfs sbodyform.BodyFormService
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

	exampleID, err := idwrap.NewWithParse(bodyData.GetExampleId())
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

	bodyForm := mbodyform.BodyForm{
		ID:          idwrap.NewNow(),
		ExampleID:   exampleID,
		BodyKey:     bodyData.GetKey(),
		Description: bodyData.GetDescription(),
		Enable:      bodyData.GetEnabled(),
		Value:       bodyData.GetValue(),
	}

	err = c.bfs.CreateBodyForm(ctx, &bodyForm)
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
	BodyID, err := idwrap.NewWithParse(bodyData.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := CheckOwnerBodyForm(ctx, c.bfs, c.iaes, c.cs, c.us, BodyID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no body form found"))
	}

	bodyModel, err := tbodyform.SerializeFormRPCtoModel(bodyData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = c.bfs.UpdateBodyForm(ctx, bodyModel)
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

	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c BodyRPC) CreateBodyUrlEncoded(ctx context.Context, req *connect.Request[bodyv1.CreateBodyUrlEncodedRequest]) (*connect.Response[bodyv1.CreateBodyUrlEncodedResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c BodyRPC) UpdateBodyUrlEncoded(ctx context.Context, req *connect.Request[bodyv1.UpdateBodyUrlEncodedRequest]) (*connect.Response[bodyv1.UpdateBodyUrlEncodedResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c BodyRPC) DeleteBodyUrlEncoded(ctx context.Context, req *connect.Request[bodyv1.DeleteBodyUrlEncodedRequest]) (*connect.Response[bodyv1.DeleteBodyUrlEncodedResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
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
