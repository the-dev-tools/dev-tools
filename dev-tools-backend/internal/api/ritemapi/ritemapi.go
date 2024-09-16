package ritemapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sexampleheader"
	"dev-tools-backend/pkg/service/sexamplequery"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/theader"
	"dev-tools-backend/pkg/translate/tquery"
	itemapiv1 "dev-tools-services/gen/itemapi/v1"
	"dev-tools-services/gen/itemapi/v1/itemapiv1connect"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		hs:   hs,
		qs:   qs,
	}

	path, handler := itemapiv1connect.NewItemApiServiceHandler(server, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemApiRPC) CreateApiCall(ctx context.Context, req *connect.Request[itemapiv1.CreateApiCallRequest]) (*connect.Response[itemapiv1.CreateApiCallResponse], error) {
	ulidID := idwrap.NewNow()
	collectionUlidID, err := idwrap.NewWithParse(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := collection.CheckOwnerCollection(ctx, *c.cs, *c.us, collectionUlidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	// TODO: add parent id
	apiCall := &mitemapi.ItemApi{
		ID:           ulidID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.GetName(),
	}
	err = c.ias.CreateItemApi(ctx, apiCall)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	example := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    ulidID,
		CollectionID: collectionUlidID,
		IsDefault:    true,
		Name:         "Default",
		Body:         []byte{},
	}
	err = c.iaes.CreateApiExample(ctx, example)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &itemapiv1.CreateApiCallResponse{
		Id:   ulidID.String(),
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

	isOwner, err := c.CheckOwnerApi(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	var examplePtr *mitemapiexample.ItemApiExample = nil
	if isDefaultExample {
		examplePtr, err = c.iaes.GetDefaultApiExample(ctx, apiUlid)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		examplePtr, err = c.iaes.GetApiExample(ctx, *exampleIDPtr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	var parentID string
	if item.ParentID == nil {
		parentID = ""
	} else {
		parentID = item.ParentID.String()
	}
	// TODO: it is overfetching the data change it
	examples, err := c.iaes.GetApiExamples(ctx, apiUlid)
	if err != nil && err == sitemapiexample.ErrNoItemApiExampleFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	header, err := c.hs.GetHeaderByExampleID(ctx, examplePtr.ID)
	if err != nil && err == sexampleheader.ErrNoHeaderFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	queries, err := c.qs.GetExampleQueriesByExampleID(ctx, examplePtr.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcHeaders := tgeneric.MassConvert(header, theader.SerializeHeaderModelToRPC)
	rpcQueries := tgeneric.MassConvert(queries, tquery.SerializeQueryModelToRPC)

	// TODO: it is overfetching the data change it
	metaExamplesRPC := make([]*itemapiexamplev1.ApiExampleMeta, len(examples))
	for i, example := range examples {
		metaExamplesRPC[i] = &itemapiexamplev1.ApiExampleMeta{
			Id:   example.ID.String(),
			Name: example.Name,
		}
	}

	respRaw := &itemapiv1.GetApiCallResponse{
		ApiCall: &itemapiv1.ApiCall{
			Meta: &itemapiv1.ApiCallMeta{
				Id:       item.ID.String(),
				Name:     item.Name,
				Examples: metaExamplesRPC,
				Method:   item.Method,
			},
			CollectionId: item.CollectionID.String(),
			ParentId:     parentID,
			Url:          item.Url,
		},
		Example: &itemapiexamplev1.ApiExample{
			Meta: &itemapiexamplev1.ApiExampleMeta{
				Id:   examplePtr.ID.String(),
				Name: examplePtr.Name,
			},
			Header: rpcHeaders,
			Query:  rpcQueries,
			Body: &itemapiexamplev1.Body{
				Value: &itemapiexamplev1.Body_Raw{
					Raw: &itemapiexamplev1.BodyRawData{
						BodyBytes: examplePtr.Body,
					},
				},
			},
			Updated: timestamppb.New(examplePtr.Updated),
		},
	}

	return connect.NewResponse(respRaw), nil
}

func (c *ItemApiRPC) UpdateApiCall(ctx context.Context, req *connect.Request[itemapiv1.UpdateApiCallRequest]) (*connect.Response[itemapiv1.UpdateApiCallResponse], error) {
	// TODO: add more rail guards
	apiCall := req.Msg.GetApiCall()
	meta := apiCall.GetMeta()
	ulidID, err := idwrap.NewWithParse(meta.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	CollectionID, err := idwrap.NewWithParse(apiCall.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerApi(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	checkOwner, err := collection.CheckOwnerCollection(ctx, *c.cs, *c.us, CollectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !checkOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	var parentUlidIDPtr *idwrap.IDWrap = nil
	if apiCall.GetParentId() != "" {
		parentUlidID, err := idwrap.NewWithParse(req.Msg.GetApiCall().GetParentId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		checkfolder, err := c.ifs.GetItemFolder(ctx, parentUlidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if checkfolder.CollectionID.Compare(CollectionID) != 0 {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
		}
		parentUlidIDPtr = &parentUlidID
	}

	itemApi := &mitemapi.ItemApi{
		ID:           ulidID,
		CollectionID: CollectionID,
		ParentID:     parentUlidIDPtr,
		Name:         meta.GetName(),
		Url:          apiCall.GetUrl(),
		Method:       meta.GetMethod(),
	}

	err = c.ias.UpdateItemApi(ctx, itemApi)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiv1.UpdateApiCallResponse{}), nil
}

func (c *ItemApiRPC) DeleteApiCall(ctx context.Context, req *connect.Request[itemapiv1.DeleteApiCallRequest]) (*connect.Response[itemapiv1.DeleteApiCallResponse], error) {
	ulidID, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerApi(ctx, ulidID)
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

func (c *ItemApiRPC) CheckOwnerApi(ctx context.Context, apiID idwrap.IDWrap) (bool, error) {
	api, err := c.ias.GetItemApi(ctx, apiID)
	if err != nil {
		return false, err
	}
	isOwner, err := collection.CheckOwnerCollection(ctx, *c.cs, *c.us, api.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}
