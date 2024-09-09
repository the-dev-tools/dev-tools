package ritemapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/suser"
	itemapiv1 "dev-tools-services/gen/itemapi/v1"
	"dev-tools-services/gen/itemapi/v1/itemapiv1connect"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
	"errors"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ItemApiRPC struct {
	DB   *sql.DB
	ias  *sitemapi.ItemApiService
	ifs  *sitemfolder.ItemFolderService
	cs   *scollection.CollectionService
	us   *suser.UserService
	iaes *sitemapiexample.ItemApiExampleService
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

	authInterceptor := mwauth.NewAuthInterceptor(secret)
	interceptors := connect.WithInterceptors(authInterceptor)
	server := &ItemApiRPC{
		DB:   db,
		ias:  ias,
		ifs:  ifs,
		cs:   cs,
		us:   us,
		iaes: iaes,
	}

	path, handler := itemapiv1connect.NewItemApiServiceHandler(server, interceptors)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemApiRPC) CreateApiCall(ctx context.Context, req *connect.Request[itemapiv1.CreateApiCallRequest]) (*connect.Response[itemapiv1.CreateApiCallResponse], error) {
	ulidID := ulid.Make()
	collectionUlidID, err := ulid.Parse(req.Msg.GetCollectionId())
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
		ID:           ulid.Make(),
		ItemApiID:    ulidID,
		CollectionID: collectionUlidID,
		IsDefault:    true,
		Name:         "Default",
		Headers:      *mitemapiexample.NewHeadersDefault(),
		Query:        *mitemapiexample.NewQueryDefault(),
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
	ulidID, err := ulid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isDefaultExample := false
	var exampleIDPtr *ulid.ULID = nil
	rawExampleID := req.Msg.GetExampleId()
	if rawExampleID == "" {
		isDefaultExample = true
	} else {
		exampleID, err := ulid.Parse(req.Msg.GetExampleId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		exampleIDPtr = &exampleID
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	item, err := c.ias.GetItemApi(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	isOwner, err := c.CheckOwnerApi(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	var examplePtr *mitemapiexample.ItemApiExample = nil
	if isDefaultExample {
		examplePtr, err = c.iaes.GetDefaultApiExample(ctx, ulidID)
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

	examples, err := c.iaes.GetApiExamples(ctx, ulidID)
	if err != nil && err == sitemapiexample.ErrNoItemApiExampleFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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
			},
			CollectionId: item.CollectionID.String(),
			ParentId:     parentID,
			Url:          item.Url,
			Method:       item.Method,
		},
		Example: &itemapiexamplev1.ApiExample{
			Meta: &itemapiexamplev1.ApiExampleMeta{
				Id:   examplePtr.ID.String(),
				Name: examplePtr.Name,
			},
			Headers: examplePtr.GetHeaders(),
			Cookies: examplePtr.GetCookies(),
			Query:   examplePtr.GetQueryParams(),
			Body:    examplePtr.Body,
			Updated: timestamppb.New(examplePtr.Updated),
			Created: timestamppb.New(examplePtr.GetCreatedTime()),
		},
	}

	return connect.NewResponse(respRaw), nil
}

func (c *ItemApiRPC) UpdateApiCall(ctx context.Context, req *connect.Request[itemapiv1.UpdateApiCallRequest]) (*connect.Response[itemapiv1.UpdateApiCallResponse], error) {
	// TODO: add more rail guards
	apiCall := req.Msg.GetApiCall()
	meta := apiCall.GetMeta()
	ulidID, err := ulid.Parse(meta.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	CollectionID, err := ulid.Parse(apiCall.GetCollectionId())
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

	var parentUlidIDPtr *ulid.ULID = nil
	if apiCall.GetParentId() != "" {
		parentUlidID, err := ulid.Parse(req.Msg.GetApiCall().GetParentId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		checkfolder, err := c.ifs.GetItemFolder(ctx, parentUlidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if checkfolder.CollectionID != CollectionID {
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
		Method:       apiCall.GetMethod(),
	}

	err = c.ias.UpdateItemApi(ctx, itemApi)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiv1.UpdateApiCallResponse{}), nil
}

// DeleteApiCall calls collection.v1.CollectionService.DeleteApiCall.
func (c *ItemApiRPC) DeleteApiCall(ctx context.Context, req *connect.Request[itemapiv1.DeleteApiCallRequest]) (*connect.Response[itemapiv1.DeleteApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.GetId())
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

func (c *ItemApiRPC) CheckOwnerApi(ctx context.Context, apiID ulid.ULID) (bool, error) {
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
