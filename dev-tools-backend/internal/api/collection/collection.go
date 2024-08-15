package collection

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/model/mcollection"
	"dev-tools-backend/pkg/model/mcollection/mitemapi"
	"dev-tools-backend/pkg/model/mcollection/mitemfolder"
	"dev-tools-backend/pkg/model/result/mresultapi"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/scollection/sitemapi"
	"dev-tools-backend/pkg/service/scollection/sitemfolder"
	"dev-tools-backend/pkg/service/sorg"
	"dev-tools-backend/pkg/service/sresultapi"
	"dev-tools-backend/pkg/stoken"
	"dev-tools-backend/pkg/translate/titemnest"
	"dev-tools-backend/pkg/translate/tpostman"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodes/nodeapi"
	apiresultv1 "devtools-services/gen/apiresult/v1"
	collectionv1 "devtools-services/gen/collection/v1"
	"devtools-services/gen/collection/v1/collectionv1connect"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type CollectionService struct {
	DB     *sql.DB
	secret []byte
}

type ContextKeyStr string

const (
	UserIDKeyCtx ContextKeyStr = "auth"
)

func (c CollectionService) NewAuthInterceptor() connect.UnaryInterceptorFunc {
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
func (c *CollectionService) ListCollections(ctx context.Context, req *connect.Request[collectionv1.ListCollectionsRequest]) (*connect.Response[collectionv1.ListCollectionsResponse], error) {
	orgUlid, err := mwauth.GetContextUserOrgID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	org, err := sorg.GetOrg(orgUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	simpleCollections, err := scollection.ListCollections(org.ID)
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
func (c *CollectionService) CreateCollection(ctx context.Context, req *connect.Request[collectionv1.CreateCollectionRequest]) (*connect.Response[collectionv1.CreateCollectionResponse], error) {
	ulidID := ulid.Make()

	orgUlid, err := mwauth.GetContextUserOrgID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	org, err := sorg.GetOrg(orgUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	collection := mcollection.Collection{
		ID:      ulidID,
		OwnerID: org.ID,
		Name:    req.Msg.Name,
	}
	err = scollection.CreateCollection(&collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&collectionv1.CreateCollectionResponse{
		Id:    ulidID.String(),
		Name:  req.Msg.Name,
		Items: []*collectionv1.Item{},
	}), nil
}

// GetCollection calls collection.v1.CollectionService.GetCollection.
func (c *CollectionService) GetCollection(ctx context.Context, req *connect.Request[collectionv1.GetCollectionRequest]) (*connect.Response[collectionv1.GetCollectionResponse], error) {
	id := req.Msg.Id
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerCollection(ctx, ulidID)
	if err != nil {
		log.Print("try to get collection error: ", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	collection, err := scollection.GetCollection(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	folderItems, err := sitemfolder.GetFoldersWithCollectionID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiItems, err := sitemapi.GetApisWithCollectionID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pair := titemnest.TranslateItemFolderNested(folderItems, apiItems)
	items := pair.GetItemFolders()

	respRaw := &collectionv1.GetCollectionResponse{
		Id:    collection.ID.String(),
		Name:  collection.Name,
		Items: items,
	}

	return connect.NewResponse(respRaw), nil
}

// UpdateCollection calls collection.v1.CollectionService.UpdateCollection.
func (c *CollectionService) UpdateCollection(ctx context.Context, req *connect.Request[collectionv1.UpdateCollectionRequest]) (*connect.Response[collectionv1.UpdateCollectionResponse], error) {
	id := req.Msg.Id
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	// TODO: can be merge with check
	collectionOld, err := scollection.GetCollection(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	collection := mcollection.Collection{
		ID:      ulidID,
		Name:    req.Msg.Name,
		OwnerID: collectionOld.OwnerID,
	}
	err = scollection.UpdateCollection(&collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.UpdateCollectionResponse{}), nil
}

// DeleteCollection calls collection.v1.CollectionService.DeleteCollection.
func (c *CollectionService) DeleteCollection(ctx context.Context, req *connect.Request[collectionv1.DeleteCollectionRequest]) (*connect.Response[collectionv1.DeleteCollectionResponse], error) {
	id := req.Msg.Id
	// convert id
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	err = sitemapi.DeleteApisWithCollectionID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = sitemfolder.DeleteFoldersWithCollectionID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = scollection.DeleteCollection(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&collectionv1.DeleteCollectionResponse{}), nil
}

// ImportPostman calls collection.v1.CollectionService.ImportPostman.
func (c *CollectionService) ImportPostman(ctx context.Context, req *connect.Request[collectionv1.ImportPostmanRequest]) (*connect.Response[collectionv1.ImportPostmanResponse], error) {
	orgUlid, err := mwauth.GetContextUserOrgID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	org, err := sorg.GetOrg(orgUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	postmanCollection, err := tpostman.ParsePostmanCollection(req.Msg.Data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ulidID := ulid.Make()

	collection := mcollection.Collection{
		ID:      ulidID,
		Name:    req.Msg.Name,
		OwnerID: org.ID,
	}
	err = scollection.CreateCollection(&collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: add ownerID
	items, err := tpostman.ConvertPostmanCollection(postmanCollection, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, folder := range items.Folder {
		err = sitemfolder.CreateItemFolder(&folder)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	for _, api := range items.Api {
		err = sitemapi.CreateItemApi(&api)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	respRaw := &collectionv1.ImportPostmanResponse{
		Id: ulidID.String(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

// CreateFolder calls collection.v1.CollectionService.CreateFolder.
func (c *CollectionService) CreateFolder(ctx context.Context, req *connect.Request[collectionv1.CreateFolderRequest]) (*connect.Response[collectionv1.CreateFolderResponse], error) {
	ulidID := ulid.Make()
	collectionUlidID, err := ulid.Parse(req.Msg.CollectionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// TODO: parentID
	folder := mitemfolder.ItemFolder{
		ID:           ulidID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.Name,
	}
	err = sitemfolder.CreateItemFolder(&folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.CreateFolderResponse{
		Id:   ulidID.String(),
		Name: req.Msg.Name,
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

// CreateApiCall calls collection.v1.CollectionService.CreateApiCall.
func (c *CollectionService) CreateApiCall(ctx context.Context, req *connect.Request[collectionv1.CreateApiCallRequest]) (*connect.Response[collectionv1.CreateApiCallResponse], error) {
	ulidID := ulid.Make()
	collectionUlidID, err := ulid.Parse(req.Msg.CollectionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	apiCall := &mitemapi.ItemApi{
		ID:           ulidID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.Name,
		// TODO: ParentID:
	}
	err = sitemapi.CreateItemApi(apiCall)
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
func (c *CollectionService) GetFolder(ctx context.Context, req *connect.Request[collectionv1.GetFolderRequest]) (*connect.Response[collectionv1.GetFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	folder, err := sitemfolder.GetItemFolder(ulidID)
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
func (c *CollectionService) GetApiCall(ctx context.Context, req *connect.Request[collectionv1.GetApiCallRequest]) (*connect.Response[collectionv1.GetApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	item, err := sitemapi.GetItemApi(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	isOwner, err := CheckOwnerApi(ctx, ulidID)
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
				QueryParams: item.QueryParams.QueryMap,
				Headers:     item.Headers.HeaderMap,
			},
		},
	}

	return connect.NewResponse(respRaw), nil
}

// UpdateFolder calls collection.v1.CollectionService.UpdateFolder.
func (c *CollectionService) UpdateFolder(ctx context.Context, req *connect.Request[collectionv1.UpdateFolderRequest]) (*connect.Response[collectionv1.UpdateFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Folder.Meta.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	collectionID, err := ulid.Parse(req.Msg.Folder.CollectionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	var parentUlidIDPtr *ulid.ULID = nil
	if req.Msg.Folder.ParentId != "" {
		parentUlidID, err := ulid.Parse(req.Msg.Folder.ParentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		checkfolder, err := sitemfolder.GetItemFolder(parentUlidID)
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
		Name:         req.Msg.Folder.Meta.Name,
		ParentID:     parentUlidIDPtr,
	}

	err = sitemfolder.UpdateItemFolder(&folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.UpdateFolderResponse{}), nil
}

// UpdateApiCall calls collection.v1.CollectionService.UpdateApiCall.
func (c *CollectionService) UpdateApiCall(ctx context.Context, req *connect.Request[collectionv1.UpdateApiCallRequest]) (*connect.Response[collectionv1.UpdateApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.ApiCall.Meta.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	CollectionID, err := ulid.Parse(req.Msg.ApiCall.CollectionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerApi(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	checkOwner, err := CheckOwnerCollection(ctx, CollectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !checkOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	var parentUlidIDPtr *ulid.ULID = nil
	if req.Msg.ApiCall.ParentId != "" {
		parentUlidID, err := ulid.Parse(req.Msg.ApiCall.ParentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		checkfolder, err := sitemfolder.GetItemFolder(parentUlidID)
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
		Name:         req.Msg.ApiCall.Meta.Name,
		Url:          req.Msg.ApiCall.Data.Url,
		Method:       req.Msg.ApiCall.Data.Method,
		Headers:      mitemapi.Headers{HeaderMap: req.Msg.ApiCall.Data.Headers},
		QueryParams:  mitemapi.QueryParams{QueryMap: req.Msg.ApiCall.Data.QueryParams},
		Body:         req.Msg.ApiCall.Data.Body,
	}

	err = sitemapi.UpdateItemApi(itemApi)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.UpdateApiCallResponse{}), nil
}

// DeleteFolder calls collection.v1.CollectionService.DeleteFolder.
func (c *CollectionService) DeleteFolder(ctx context.Context, req *connect.Request[collectionv1.DeleteFolderRequest]) (*connect.Response[collectionv1.DeleteFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	err = sitemfolder.DeleteItemFolder(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.DeleteFolderResponse{}), nil
}

// DeleteApiCall calls collection.v1.CollectionService.DeleteApiCall.
func (c *CollectionService) DeleteApiCall(ctx context.Context, req *connect.Request[collectionv1.DeleteApiCallRequest]) (*connect.Response[collectionv1.DeleteApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerApi(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	// TODO: need a check for ownerID
	err = sitemapi.DeleteItemApi(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.DeleteApiCallResponse{}), nil
}

// RunApiCall calls collection.v1.CollectionService.RunApiCall.
func (c *CollectionService) RunApiCall(ctx context.Context, req *connect.Request[collectionv1.RunApiCallRequest]) (*connect.Response[collectionv1.RunApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerApi(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	itemApiCall, err := sitemapi.GetItemApi(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiCallNodeData := mnodedata.NodeApiRestData{
		Url:         itemApiCall.Url,
		Method:      itemApiCall.Method,
		Headers:     itemApiCall.Headers.HeaderMap,
		QueryParams: itemApiCall.QueryParams.QueryMap,
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

	err = sresultapi.CreateResultApi(result)
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

func CreateService(db *sql.DB, secret []byte) (*api.Service, error) {
	authInterceptor := mwauth.NewAuthInterceptor(secret)
	orgInterceptor := mwauth.NewOrgInterceptor()
	interceptors := connect.WithInterceptors(authInterceptor, orgInterceptor)
	server := &CollectionService{
		DB:     db,
		secret: secret,
	}

	path, handler := collectionv1connect.NewCollectionServiceHandler(server, interceptors)
	return &api.Service{Path: path, Handler: handler}, nil
}

func CheckOwnerCollection(ctx context.Context, collectionID ulid.ULID) (bool, error) {
	orgID, err := mwauth.GetContextUserOrgID(ctx)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	isOwner, err := scollection.CheckOwner(collectionID, orgID)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	if !isOwner {
		return false, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	return isOwner, nil
}

func CheckOwnerFolder(ctx context.Context, folderID ulid.ULID) (bool, error) {
	folder, err := sitemfolder.GetItemFolder(folderID)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	isOwner, err := CheckOwnerCollection(ctx, folder.CollectionID)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}
	return isOwner, nil
}

func CheckOwnerApi(ctx context.Context, apiID ulid.ULID) (bool, error) {
	api, err := sitemapi.GetItemApi(apiID)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}
	isOwner, err := CheckOwnerCollection(ctx, api.CollectionID)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}
	return isOwner, nil
}
