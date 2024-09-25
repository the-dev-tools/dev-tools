package ritemapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/internal/api/ritemfolder"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyraw"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/service/sbodyform"
	"dev-tools-backend/pkg/service/sbodyraw"
	"dev-tools-backend/pkg/service/sbodyurl"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sexampleheader"
	"dev-tools-backend/pkg/service/sexamplequery"
	"dev-tools-backend/pkg/service/sexampleresp"
	"dev-tools-backend/pkg/service/sexamplerespheader"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tbodyform"
	"dev-tools-backend/pkg/translate/tbodyurl"
	"dev-tools-backend/pkg/translate/texample"
	"dev-tools-backend/pkg/translate/texampleresp"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/titemapi"
	"dev-tools-backend/pkg/zstdcompress"
	bodyv1 "dev-tools-services/gen/body/v1"
	itemapiv1 "dev-tools-services/gen/itemapi/v1"
	"dev-tools-services/gen/itemapi/v1/itemapiv1connect"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
	"errors"
	"sort"

	"connectrpc.com/connect"
)

type ItemApiRPC struct {
	DB   *sql.DB
	ias  *sitemapi.ItemApiService
	ifs  *sitemfolder.ItemFolderService
	cs   *scollection.CollectionService
	us   *suser.UserService
	iaes *sitemapiexample.ItemApiExampleService

	// Sub
	hs *sexampleheader.HeaderService
	qs *sexamplequery.ExampleQueryService

	// Body
	brs  *sbodyraw.BodyRawService
	bfs  *sbodyform.BodyFormService
	bufs *sbodyurl.BodyURLEncodedService

	// ExampleResp
	ers  *sexampleresp.ExampleRespService
	erhs *sexamplerespheader.ExampleRespHeaderService
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	ias, err := sitemapi.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ifs, err := sitemfolder.New(ctx, db)
	if err != nil {
		return nil, err
	}

	cs, err := scollection.New(ctx, db)
	if err != nil {
		return nil, err
	}

	us, err := suser.New(ctx, db)
	if err != nil {
		return nil, err
	}

	iaes, err := sitemapiexample.New(ctx, db)
	if err != nil {
		return nil, err
	}

	hs, err := sexampleheader.New(ctx, db)
	if err != nil {
		return nil, err
	}

	qs, err := sexamplequery.New(ctx, db)
	if err != nil {
		return nil, err
	}

	brs, err := sbodyraw.New(ctx, db)
	if err != nil {
		return nil, err
	}

	bfs, err := sbodyform.New(ctx, db)
	if err != nil {
		return nil, err
	}

	bufs, err := sbodyurl.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ers, err := sexampleresp.New(ctx, db)
	if err != nil {
		return nil, err
	}

	erhs, err := sexamplerespheader.New(ctx, db)
	if err != nil {
		return nil, err
	}

	var options []connect.HandlerOption
	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	server := &ItemApiRPC{
		DB:   db,
		ias:  ias,
		ifs:  ifs,
		cs:   cs,
		us:   us,
		iaes: iaes,
		// Sub
		hs: hs,
		qs: qs,
		// Body
		brs:  brs,
		bfs:  bfs,
		bufs: bufs,
		// example
		ers:  ers,
		erhs: erhs,
	}

	path, handler := itemapiv1connect.NewItemApiServiceHandler(server, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemApiRPC) CreateApiCall(ctx context.Context, req *connect.Request[itemapiv1.CreateApiCallRequest]) (*connect.Response[itemapiv1.CreateApiCallResponse], error) {
	itemApiReq, err := titemapi.SeralizeRPCToModelWithoutID(req.Msg.Data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := collection.CheckOwnerCollection(ctx, *c.cs, *c.us, itemApiReq.CollectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	if itemApiReq.ParentID != nil {
		ok, err := ritemfolder.CheckOwnerFolder(ctx, *c.ifs, *c.cs, *c.us, *itemApiReq.ParentID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("not found"))
		}
	}

	// TODO: add ordering it should append into end

	itemApiReq.ID = idwrap.NewNow()

	err = c.ias.CreateItemApi(ctx, itemApiReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	example := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    itemApiReq.ID,
		CollectionID: itemApiReq.CollectionID,
		IsDefault:    true,
		Name:         "Default",
	}
	err = c.iaes.CreateApiExample(ctx, example)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &itemapiv1.CreateApiCallResponse{
		Id:   itemApiReq.ID.String(),
		Name: req.Msg.GetName(),
	}
	return connect.NewResponse(respRaw), nil
}

func (c *ItemApiRPC) GetApiCall(ctx context.Context, req *connect.Request[itemapiv1.GetApiCallRequest]) (*connect.Response[itemapiv1.GetApiCallResponse], error) {
	apiUlid, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isDefaultExample := false
	var exampleIDPtr *idwrap.IDWrap = nil
	rawExampleID := req.Msg.GetExampleId()
	if rawExampleID == "" {
		isDefaultExample = true
	} else {
		exampleID, err := idwrap.NewWithParse(req.Msg.GetExampleId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		exampleIDPtr = &exampleID
	}

	item, err := c.ias.GetItemApi(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	isOwner, err := CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	defaultExample, err := c.iaes.GetDefaultApiExample(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	defaultID := defaultExample.ID
	var examplePtr *mitemapiexample.ItemApiExample = nil
	if isDefaultExample {
		examplePtr = defaultExample
	} else {
		examplePtr, err = c.iaes.GetApiExample(ctx, *exampleIDPtr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// TODO: it is overfetching the data change it
	examples, err := c.iaes.GetApiExamples(ctx, apiUlid)
	if err != nil && err == sitemapiexample.ErrNoItemApiExampleFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	headers, err := c.hs.GetHeaderByExampleID(ctx, examplePtr.ID)
	if err != nil && err == sexampleheader.ErrNoHeaderFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	queries, err := c.qs.GetExampleQueriesByExampleID(ctx, examplePtr.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: it is overfetching the data change it
	metaExamplesRPC := make([]*itemapiexamplev1.ApiExampleMeta, len(examples))

	// TODO: simplify this
	for i, example := range examples {
		metaExamplesRPC[i] = &itemapiexamplev1.ApiExampleMeta{
			Id:   example.ID.String(),
			Name: example.Name,
		}
	}

	bodyPtr := &bodyv1.Body{}
	switch examplePtr.BodyType {
	case mitemapiexample.BodyTypeRaw:
		body, err := c.brs.GetBodyRawByExampleID(ctx, examplePtr.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		data := body.Data
		if body.CompressType == mbodyraw.CompressTypeZstd {
			body.Data, err = zstdcompress.Decompress(data)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		bodyPtr = &bodyv1.Body{
			Value: &bodyv1.Body_Raw{
				Raw: &bodyv1.BodyRaw{
					BodyBytes: body.Data,
				},
			},
		}
	case mitemapiexample.BodyTypeForm:
		body, err := c.bfs.GetBodyFormsByExampleID(ctx, examplePtr.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		sort.Slice(body, func(i, j int) bool {
			return body[i].ID.Compare(body[j].ID) < 0
		})
		bodyPtr = &bodyv1.Body{
			Value: &bodyv1.Body_Forms{
				Forms: &bodyv1.BodyFormArray{
					Items: tgeneric.MassConvert(body, tbodyform.SerializeFormModelToRPC),
				},
			},
		}
	case mitemapiexample.BodyTypeUrlencoded:
		body, err := c.bufs.GetBodyURLEncodedByExampleID(ctx, examplePtr.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		sort.Slice(body, func(i, j int) bool {
			return body[i].ID.Compare(body[j].ID) < 0
		})
		bodyPtr = &bodyv1.Body{
			Value: &bodyv1.Body_UrlEncodeds{
				UrlEncodeds: &bodyv1.BodyUrlEncodedArray{
					Items: tgeneric.MassConvert(body, tbodyurl.SerializeURLModelToRPC),
				},
			},
		}
	}

	var resp *itemapiexamplev1.ApiExampleResponse = nil
	exampleResp, err := c.ers.GetExampleRespByExampleID(ctx, examplePtr.ID)
	if err != nil && err != sexampleresp.ErrNoRespFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if exampleResp != nil {
		respHeaders, err := c.erhs.GetHeaderByRespID(ctx, examplePtr.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		resp, err = texampleresp.SeralizeModelToRPC(*exampleResp, respHeaders)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	exampleRPC := texample.SerializeModelToRPC(*examplePtr, queries, headers, bodyPtr, resp)

	apiCall := titemapi.DeseralizeModelToRPC(item, defaultID, metaExamplesRPC)
	return connect.NewResponse(&itemapiv1.GetApiCallResponse{ApiCall: apiCall, Example: exampleRPC}), nil
}

func (c *ItemApiRPC) UpdateApiCall(ctx context.Context, req *connect.Request[itemapiv1.UpdateApiCallRequest]) (*connect.Response[itemapiv1.UpdateApiCallResponse], error) {
	apiCall, err := titemapi.SeralizeRPCToModel(req.Msg.GetApiCall())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, apiCall.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	checkOwner, err := collection.CheckOwnerCollection(ctx, *c.cs, *c.us, apiCall.CollectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !checkOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	if apiCall.ParentID != nil {
		checkfolder, err := c.ifs.GetFolder(ctx, *apiCall.ParentID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if checkfolder.CollectionID.Compare(apiCall.CollectionID) != 0 {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
		}
	}

	err = c.ias.UpdateItemApi(ctx, apiCall)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiv1.UpdateApiCallResponse{}), nil
}

func (c *ItemApiRPC) MoveApiCall(ctx context.Context, req *connect.Request[itemapiv1.MoveApiCallRequest]) (*connect.Response[itemapiv1.MoveApiCallResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *ItemApiRPC) DeleteApiCall(ctx context.Context, req *connect.Request[itemapiv1.DeleteApiCallRequest]) (*connect.Response[itemapiv1.DeleteApiCallResponse], error) {
	ulidID, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	// TODO: need a check for ownerID
	err = c.ias.DeleteItemApi(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiv1.DeleteApiCallResponse{}), nil
}

func CheckOwnerApi(ctx context.Context, ias sitemapi.ItemApiService, cs scollection.CollectionService, us suser.UserService, apiID idwrap.IDWrap) (bool, error) {
	api, err := ias.GetItemApi(ctx, apiID)
	if err != nil {
		return false, err
	}
	isOwner, err := collection.CheckOwnerCollection(ctx, cs, us, api.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}
