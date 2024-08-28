package collection

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/dbtime"
	"dev-tools-backend/pkg/model/mcollection"
	"dev-tools-backend/pkg/model/mcollection/mitemapi"
	"dev-tools-backend/pkg/model/mcollection/mitemfolder"
	"dev-tools-backend/pkg/model/result/mresultapi"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/scollection/sitemapi"
	"dev-tools-backend/pkg/service/scollection/sitemfolder"
	"dev-tools-backend/pkg/service/sresultapi"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/stoken"
	"dev-tools-backend/pkg/translate/titemnest"
	"dev-tools-backend/pkg/translate/tpostman"
	"dev-tools-nodes/pkg/model/mnode"
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodes/nodeapi"
	apiresultv1 "dev-tools-services/gen/apiresult/v1"
	collectionv1 "dev-tools-services/gen/collection/v1"
	"dev-tools-services/gen/collection/v1/collectionv1connect"
	nodedatav1 "dev-tools-services/gen/nodedata/v1"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CollectionServiceRPC struct {
	DB     *sql.DB
	cs     scollection.CollectionService
	ws     sworkspace.WorkspaceService
	us     suser.UserService
	ias    sitemapi.ItemApiService
	ifs    sitemfolder.ItemFolderService
	ras    sresultapi.ResultApiService
	secret []byte
}

type ContextKeyStr string

const (
	UserIDKeyCtx ContextKeyStr = "auth"
)

func (c CollectionServiceRPC) NewAuthInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			headerValue := req.Header().Get(stoken.TokenHeaderKey)
			if headerValue == "" {
				// Check token in handlers.
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					errors.New("no token provided"),
				)
			}

			tokenRaw := strings.Split(headerValue, "Bearer ")
			if len(tokenRaw) != 2 {
				return nil, connect.NewError(
					connect.CodeUnauthenticated, errors.New("invalid token"))
			}

			token, err := stoken.ValidateJWT(tokenRaw[1], stoken.AccessToken, c.secret)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			claims, err := stoken.GetClaims(token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}

			ulidID, err := ulid.Parse(claims.Subject)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}

			CtxWithValue := context.WithValue(ctx, UserIDKeyCtx, ulidID)

			return next(CtxWithValue, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

// ListCollections calls collection.v1.CollectionService.ListCollections.
func (c *CollectionServiceRPC) ListCollections(ctx context.Context, req *connect.Request[collectionv1.ListCollectionsRequest]) (*connect.Response[collectionv1.ListCollectionsResponse], error) {
	workspaceUlid, err := ulid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerWorkspace(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no workspace found"))
	}

	org, err := c.ws.Get(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	simpleCollections, err := c.cs.ListCollections(ctx, org.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	metaCollections := make([]*collectionv1.CollectionMeta, 0, len(simpleCollections))
	for _, collection := range simpleCollections {
		metaCollections = append(metaCollections, &collectionv1.CollectionMeta{
			Id:   collection.ID.String(),
			Name: collection.Name,
		})
	}

	respRaw := &collectionv1.ListCollectionsResponse{
		MetaCollections: metaCollections,
	}
	return connect.NewResponse(respRaw), nil
}

// CreateCollection calls collection.v1.CollectionService.CreateCollection.
func (c *CollectionServiceRPC) CreateCollection(ctx context.Context, req *connect.Request[collectionv1.CreateCollectionRequest]) (*connect.Response[collectionv1.CreateCollectionResponse], error) {
	workspaceUlid, err := ulid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerWorkspace(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		// INFO: don't send leaked information to the client
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no workspace found"))
	}
	name := req.Msg.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is empty"))
	}
	ulidID := ulid.Make()
	dbTimeNow := dbtime.DBNow()
	collection := mcollection.Collection{
		ID:      ulidID,
		OwnerID: workspaceUlid,
		Name:    name,
		Created: dbTimeNow,
		Updated: dbTimeNow,
	}
	err = c.cs.CreateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&collectionv1.CreateCollectionResponse{
		Id:    ulidID.String(),
		Name:  name,
		Items: []*collectionv1.Item{},
	}), nil
}

// GetCollection calls collection.v1.CollectionService.GetCollection.
func (c *CollectionServiceRPC) GetCollection(ctx context.Context, req *connect.Request[collectionv1.GetCollectionRequest]) (*connect.Response[collectionv1.GetCollectionResponse], error) {
	id := req.Msg.GetId()
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerCollection(ctx, ulidID)
	if err != nil {
		log.Print("try to get collection error: ", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	collection, err := c.cs.GetCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	folderItems, err := c.ifs.GetFoldersWithCollectionID(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiItems, err := c.ias.GetApisWithCollectionID(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pair, err := titemnest.TranslateItemFolderNested(folderItems, apiItems)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	items := pair.GetItemFolders()

	respRaw := &collectionv1.GetCollectionResponse{
		Id:      collection.ID.String(),
		Name:    collection.Name,
		Items:   items,
		Created: timestamppb.New(collection.Created),
		Updated: timestamppb.New(collection.Updated),
	}

	return connect.NewResponse(respRaw), nil
}

// UpdateCollection calls collection.v1.CollectionService.UpdateCollection.
func (c *CollectionServiceRPC) UpdateCollection(ctx context.Context, req *connect.Request[collectionv1.UpdateCollectionRequest]) (*connect.Response[collectionv1.UpdateCollectionResponse], error) {
	id := req.Msg.GetId()
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	// TODO: can be merge with check
	collectionOld, err := c.cs.GetCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	collection := mcollection.Collection{
		ID:      ulidID,
		Name:    req.Msg.GetName(),
		OwnerID: collectionOld.OwnerID,
	}
	err = c.cs.UpdateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.UpdateCollectionResponse{}), nil
}

// DeleteCollection calls collection.v1.CollectionService.DeleteCollection.
func (c *CollectionServiceRPC) DeleteCollection(ctx context.Context, req *connect.Request[collectionv1.DeleteCollectionRequest]) (*connect.Response[collectionv1.DeleteCollectionResponse], error) {
	id := req.Msg.GetId()
	// convert id
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	err = c.cs.DeleteCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&collectionv1.DeleteCollectionResponse{}), nil
}

// ImportPostman calls collection.v1.CollectionService.ImportPostman.
func (c *CollectionServiceRPC) ImportPostman(ctx context.Context, req *connect.Request[collectionv1.ImportPostmanRequest]) (*connect.Response[collectionv1.ImportPostmanResponse], error) {
	orgUlid, err := ulid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	isOwner, err := c.CheckOwnerWorkspace(ctx, orgUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	org, err := c.ws.Get(ctx, orgUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	postmanCollection, err := tpostman.ParsePostmanCollection(req.Msg.GetData())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ulidID := ulid.Make()

	collection := mcollection.Collection{
		ID:      ulidID,
		Name:    req.Msg.GetName(),
		OwnerID: org.ID,
	}
	err = c.cs.CreateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: add ownerID
	items, err := tpostman.ConvertPostmanCollection(postmanCollection, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = c.ifs.CreateItemApiBulk(ctx, items.Folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = c.ias.CreateItemApiBulk(ctx, items.Api)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.ImportPostmanResponse{
		Id: ulidID.String(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

// CreateFolder calls collection.v1.CollectionService.CreateFolder.
func (c *CollectionServiceRPC) CreateFolder(ctx context.Context, req *connect.Request[collectionv1.CreateFolderRequest]) (*connect.Response[collectionv1.CreateFolderResponse], error) {
	ulidID := ulid.Make()
	collectionUlidID, err := ulid.Parse(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := c.CheckOwnerCollection(ctx, collectionUlidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	// TODO: parentID
	folder := mitemfolder.ItemFolder{
		ID:           ulidID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.GetName(),
	}
	err = c.ifs.CreateItemFolder(ctx, &folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.CreateFolderResponse{
		Id:   ulidID.String(),
		Name: req.Msg.GetName(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

// CreateApiCall calls collection.v1.CollectionService.CreateApiCall.
func (c *CollectionServiceRPC) CreateApiCall(ctx context.Context, req *connect.Request[collectionv1.CreateApiCallRequest]) (*connect.Response[collectionv1.CreateApiCallResponse], error) {
	ulidID := ulid.Make()
	collectionUlidID, err := ulid.Parse(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := c.CheckOwnerCollection(ctx, collectionUlidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	apiCall := &mitemapi.ItemApi{
		ID:           ulidID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.GetName(),
		// TODO: ParentID:
	}
	err = c.ias.CreateItemApi(ctx, apiCall)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.CreateApiCallResponse{
		Id: ulidID.String(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

// GetFolder calls collection.v1.CollectionService.GetFolder.
func (c *CollectionServiceRPC) GetFolder(ctx context.Context, req *connect.Request[collectionv1.GetFolderRequest]) (*connect.Response[collectionv1.GetFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	folder, err := c.ifs.GetItemFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.GetFolderResponse{
		Folder: &collectionv1.Folder{
			Meta: &collectionv1.FolderMeta{
				Id:   folder.ID.String(),
				Name: folder.Name,
			},
			Items: []*collectionv1.Item{},
		},
	}

	return connect.NewResponse(respRaw), nil
}

// GetApiCall calls collection.v1.CollectionService.GetApiCall.
func (c *CollectionServiceRPC) GetApiCall(ctx context.Context, req *connect.Request[collectionv1.GetApiCallRequest]) (*connect.Response[collectionv1.GetApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.GetId())
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

	var parentID string
	if item.ParentID == nil {
		parentID = ""
	} else {
		parentID = item.ParentID.String()
	}

	respRaw := &collectionv1.GetApiCallResponse{
		ApiCall: &collectionv1.ApiCall{
			Meta: &collectionv1.ApiCallMeta{
				Id:   item.ID.String(),
				Name: item.Name,
			},
			CollectionId: item.CollectionID.String(),
			ParentId:     parentID,
			Data: &nodedatav1.NodeApiCallData{
				Url:         item.Url,
				Method:      item.Method,
				QueryParams: item.Query.QueryMap,
				Headers:     item.Headers.HeaderMap,
			},
		},
	}

	return connect.NewResponse(respRaw), nil
}

// UpdateFolder calls collection.v1.CollectionService.UpdateFolder.
func (c *CollectionServiceRPC) UpdateFolder(ctx context.Context, req *connect.Request[collectionv1.UpdateFolderRequest]) (*connect.Response[collectionv1.UpdateFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.GetFolder().GetMeta().GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	collectionID, err := ulid.Parse(req.Msg.GetFolder().GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	var parentUlidIDPtr *ulid.ULID = nil
	if req.Msg.GetFolder().GetParentId() != "" {
		parentUlidID, err := ulid.Parse(req.Msg.GetFolder().GetParentId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		checkfolder, err := c.ifs.GetItemFolder(ctx, parentUlidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if checkfolder.CollectionID != collectionID {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
		}
		parentUlidIDPtr = &parentUlidID
	}

	folder := mitemfolder.ItemFolder{
		ID:           ulidID,
		CollectionID: collectionID,
		Name:         req.Msg.GetFolder().GetMeta().GetName(),
		ParentID:     parentUlidIDPtr,
	}

	err = c.ifs.UpdateItemFolder(ctx, &folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.UpdateFolderResponse{}), nil
}

// UpdateApiCall calls collection.v1.CollectionService.UpdateApiCall.
func (c *CollectionServiceRPC) UpdateApiCall(ctx context.Context, req *connect.Request[collectionv1.UpdateApiCallRequest]) (*connect.Response[collectionv1.UpdateApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.GetApiCall().GetMeta().GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	CollectionID, err := ulid.Parse(req.Msg.GetApiCall().GetCollectionId())
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

	checkOwner, err := c.CheckOwnerCollection(ctx, CollectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !checkOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	var parentUlidIDPtr *ulid.ULID = nil
	if req.Msg.GetApiCall().GetParentId() != "" {
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
		Name:         req.Msg.GetApiCall().GetMeta().GetName(),
		Url:          req.Msg.GetApiCall().GetData().GetUrl(),
		Method:       req.Msg.GetApiCall().GetData().GetMethod(),
		Headers:      mitemapi.Headers{HeaderMap: req.Msg.GetApiCall().GetData().GetHeaders()},
		Query:        mitemapi.Query{QueryMap: req.Msg.GetApiCall().GetData().GetQueryParams()},
		Body:         req.Msg.GetApiCall().GetData().GetBody(),
	}

	err = c.ias.UpdateItemApi(ctx, itemApi)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.UpdateApiCallResponse{}), nil
}

// DeleteFolder calls collection.v1.CollectionService.DeleteFolder.
func (c *CollectionServiceRPC) DeleteFolder(ctx context.Context, req *connect.Request[collectionv1.DeleteFolderRequest]) (*connect.Response[collectionv1.DeleteFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := c.CheckOwnerFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	err = c.ifs.DeleteItemFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.DeleteFolderResponse{}), nil
}

// DeleteApiCall calls collection.v1.CollectionService.DeleteApiCall.
func (c *CollectionServiceRPC) DeleteApiCall(ctx context.Context, req *connect.Request[collectionv1.DeleteApiCallRequest]) (*connect.Response[collectionv1.DeleteApiCallResponse], error) {
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

	return connect.NewResponse(&collectionv1.DeleteApiCallResponse{}), nil
}

// RunApiCall calls collection.v1.CollectionService.RunApiCall.
func (c *CollectionServiceRPC) RunApiCall(ctx context.Context, req *connect.Request[collectionv1.RunApiCallRequest]) (*connect.Response[collectionv1.RunApiCallResponse], error) {
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

	itemApiCall, err := c.ias.GetItemApi(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiCallNodeData := mnodedata.NodeApiRestData{
		Url:         itemApiCall.Url,
		Method:      itemApiCall.Method,
		Headers:     itemApiCall.Headers.HeaderMap,
		QueryParams: itemApiCall.Query.QueryMap,
		Body:        itemApiCall.Body,
	}

	node := mnode.Node{
		ID:   ulidID.String(),
		Type: mnodemaster.ApiCallRest,
		Data: &apiCallNodeData,
	}

	runApiVars := make(map[string]interface{}, 0)

	nm := &mnodemaster.NodeMaster{
		CurrentNode: &node,
		HttpClient:  http.DefaultClient,
		Vars:        runApiVars,
	}

	now := time.Now()
	err = nodeapi.SendRestApiRequest(nm)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}
	lapse := time.Since(now)

	httpResp, err := nodeapi.GetHttpVarResponse(nm)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	bodyData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	result := &mresultapi.MResultAPI{
		ID:          ulid.Make(),
		TriggerType: mresultapi.TRIGGER_TYPE_COLLECTION,
		TriggerBy:   ulidID,
		Name:        itemApiCall.Name,
		Time:        time.Now(),
		Duration:    time.Duration(lapse),
		HttpResp: mresultapi.HttpResp{
			StatusCode: httpResp.StatusCode,
			Proto:      httpResp.Proto,
			ProtoMajor: httpResp.ProtoMajor,
			ProtoMinor: httpResp.ProtoMinor,
			Header:     httpResp.Header,
			Body:       bodyData,
		},
	}

	err = c.ras.CreateResultApi(ctx, result)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	headers := make(map[string]string, 0)
	for key, values := range httpResp.Header {
		headers[key] = strings.Join(values, ",")
	}

	return connect.NewResponse(&collectionv1.RunApiCallResponse{
		Result: &apiresultv1.Result{
			Id:       result.ID.String(),
			Name:     result.Name,
			Time:     result.Time.Unix(),
			Duration: result.Duration.Milliseconds(),
			Response: &apiresultv1.Result_HttpResponse{
				HttpResponse: &apiresultv1.HttpResponse{
					StatusCode: int32(result.HttpResp.StatusCode),
					Proto:      result.HttpResp.Proto,
					ProtoMajor: int32(result.HttpResp.ProtoMajor),
					ProtoMinor: int32(result.HttpResp.ProtoMinor),
					Header:     headers,
					Body:       result.HttpResp.Body,
				},
			},
		},
	}), nil
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	collectionService, err := scollection.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ias, err := sitemapi.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ifs, err := sitemfolder.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ws, err := sworkspace.New(ctx, db)
	if err != nil {
		return nil, err
	}

	us, err := suser.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ras, err := sresultapi.New(ctx, db)
	if err != nil {
		return nil, err
	}

	authInterceptor := mwauth.NewAuthInterceptor(secret)
	interceptors := connect.WithInterceptors(authInterceptor)
	server := &CollectionServiceRPC{
		DB:     db,
		secret: secret,
		cs:     *collectionService,
		ias:    *ias,
		ifs:    *ifs,
		ws:     *ws,
		us:     *us,
		ras:    *ras,
	}

	path, handler := collectionv1connect.NewCollectionServiceHandler(server, interceptors)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *CollectionServiceRPC) CheckOwnerWorkspace(ctx context.Context, workspaceID ulid.ULID) (bool, error) {
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	ok, err := c.us.CheckUserBelongsToWorkspace(ctx, userUlid, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			// INFO: this mean that workspace not belong to user
			// So for avoid information leakage, we should return not found
			return false, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
		return false, err
	}
	return ok, nil
}

func (c *CollectionServiceRPC) CheckOwnerCollection(ctx context.Context, collectionID ulid.ULID) (bool, error) {
	workspaceID, err := c.cs.GetOwner(ctx, collectionID)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	return c.CheckOwnerWorkspace(ctx, workspaceID)
}

func (c *CollectionServiceRPC) CheckOwnerFolder(ctx context.Context, folderID ulid.ULID) (bool, error) {
	folder, err := c.ifs.GetItemFolder(ctx, folderID)
	if err != nil {
		return false, err
	}

	isOwner, err := c.CheckOwnerCollection(ctx, folder.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}

func (c *CollectionServiceRPC) CheckOwnerApi(ctx context.Context, apiID ulid.ULID) (bool, error) {
	api, err := c.ias.GetItemApi(ctx, apiID)
	if err != nil {
		return false, err
	}
	isOwner, err := c.CheckOwnerCollection(ctx, api.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}
