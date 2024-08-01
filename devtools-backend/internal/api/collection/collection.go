package collection

import (
	"context"
	"database/sql"
	"devtools-backend/internal/api"
	"devtools-backend/pkg/model/mcollection"
	"devtools-backend/pkg/model/postman/v21/mpostmancollection"
	"devtools-backend/pkg/service/scollection"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodes/nodeapi"
	"devtools-nodes/pkg/resolver"
	collectionv1 "devtools-services/gen/collection/v1"
	"devtools-services/gen/collection/v1/collectionv1connect"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type CollectionService struct {
	db     *sql.DB
	secret []byte
}

func ConvertPostmanCollection(collection mpostmancollection.Collection, collectionID ulid.ULID, ownerID string) []mcollection.CollectionNode {
	var collectionNodes []mcollection.CollectionNode

	for _, item := range collection.Items {
		queryParams := make(map[string]string)

		for _, v := range item.Request.URL.Query {
			queryParams[v.Key] = v.Value
		}

		headers := make(map[string]string)
		for _, v := range item.Request.Header {
			if v.Disabled {
				continue
			}
			headers[v.Key] = v.Value
		}

		data := &mnodedata.NodeApiRestData{
			Url:         item.Request.URL.Raw,
			Method:      item.Request.Method,
			Headers:     headers,
			QueryParams: queryParams,
			Body:        []byte(item.Request.Body.Raw),
		}

		ulidID := ulid.Make()

		node := mcollection.CollectionNode{
			CollectionID: collectionID,
			ID:           ulidID,
			Name:         item.Name,
			Type:         resolver.ApiCallRest, // TODO: Change to something more meaningful.
			Data:         data,
		}

		collectionNodes = append(collectionNodes, node)

	}

	return collectionNodes
}

func (c *CollectionService) CreateCollection(ctx context.Context, req *connect.Request[collectionv1.CreateCollectionRequest]) (*connect.Response[collectionv1.CreateCollectionResponse], error) {
	ulidID := ulid.Make()
	err := scollection.CreateCollection(c.db, ulidID, req.Msg.Name)
	if err != nil {
		return nil, err
	}

	respRaw := &collectionv1.CreateCollectionResponse{Name: ulidID.String()}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

func (c *CollectionService) UpdateCollection(ctx context.Context, req *connect.Request[collectionv1.UpdateCollectionRequest]) (*connect.Response[collectionv1.UpdateCollectionResponse], error) {
	id := req.Msg.Id
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	err = scollection.UpdateCollection(c.db, ulidID, req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	respRaw := &collectionv1.UpdateCollectionResponse{}
	resp := connect.NewResponse(respRaw)

	return resp, nil
}

func (c *CollectionService) GetCollection(ctx context.Context, req *connect.Request[collectionv1.GetCollectionRequest]) (*connect.Response[collectionv1.GetCollectionResponse], error) {
	id := req.Msg.Id
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	collection, err := scollection.GetCollection(c.db, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.GetCollectionResponse{
		Id:   collection.ID.String(),
		Name: collection.Name,
	}
	resp := connect.NewResponse(respRaw)

	return resp, nil
}

func (c *CollectionService) DeleteCollection(ctx context.Context, req *connect.Request[collectionv1.DeleteCollectionRequest]) (*connect.Response[collectionv1.DeleteCollectionResponse], error) {
	id := req.Msg.Id
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}

	err = scollection.DeleteCollection(c.db, ulidID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (c *CollectionService) ListCollections(ctx context.Context, req *connect.Request[collectionv1.ListCollectionsRequest]) (*connect.Response[collectionv1.ListCollectionsResponse], error) {
	collections, names, err := scollection.ListCollections(c.db)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, id := range collections {
		ids = append(ids, id.String())
	}

	respRaw := &collectionv1.ListCollectionsResponse{
		Ids:   ids,
		Names: names,
	}

	resp := connect.NewResponse(respRaw)

	return resp, nil
}

func (c *CollectionService) ImportPostman(ctx context.Context, req *connect.Request[collectionv1.ImportPostmanRequest]) (*connect.Response[collectionv1.ImportPostmanResponse], error) {
	var postmanCollection mpostmancollection.Collection

	err := json.Unmarshal(req.Msg.Data, &postmanCollection)
	if err != nil {
		return nil, err
	}

	ulidID := ulid.Make()
	err = scollection.CreateCollection(c.db, ulidID, req.Msg.Name)
	if err != nil {
		return nil, err
	}

	// TODO: make owner id later with user
	nodes := ConvertPostmanCollection(postmanCollection, ulidID, "some-owner-id")
	for _, node := range nodes {
		err = scollection.CreateCollectionNode(c.db, node)
		if err != nil {
			return nil, err
		}
	}

	resp := connect.NewResponse(&collectionv1.ImportPostmanResponse{Id: ulidID.String()})
	return resp, nil
}

// ListNodes calls collection.v1.CollectionService.ListNodes.
func (c *CollectionService) ListNodes(ctx context.Context, req *connect.Request[collectionv1.ListNodesRequest]) (*connect.Response[collectionv1.ListNodesResponse], error) {
	id := req.Msg.CollectionId
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}

	nodeIds, err := scollection.GetCollectionNodeWithCollectionID(c.db, ulidID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(nodeIds))
	for _, id := range nodeIds {
		ids = append(ids, id.String())
	}

	return connect.NewResponse(&collectionv1.ListNodesResponse{Ids: ids}), nil
}

// CreateNode calls collection.v1.CollectionService.CreateNode.
func (c *CollectionService) CreateNode(ctx context.Context, req *connect.Request[collectionv1.CreateNodeRequest]) (*connect.Response[collectionv1.CreateNodeResponse], error) {
	id := req.Msg.CollectionId
	collectionUlidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}
	nodeUlidID := ulid.Make()

	data := &mnodedata.NodeApiRestData{
		Url:         req.Msg.Data.Url,
		Method:      req.Msg.Data.Method,
		Headers:     req.Msg.Data.Headers,
		QueryParams: req.Msg.Data.QueryParams,
		Body:        req.Msg.Data.Body,
	}

	node := mcollection.CollectionNode{
		ID:           nodeUlidID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.Name,
		Type:         req.Msg.Type,
		Data:         data,
	}

	err = scollection.CreateCollectionNode(c.db, node)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&collectionv1.CreateNodeResponse{Id: nodeUlidID.String()}), err
}

// GetNode calls collection.v1.CollectionService.GetNode.
func (c *CollectionService) GetNode(ctx context.Context, req *connect.Request[collectionv1.GetNodeRequest]) (*connect.Response[collectionv1.GetNodeResponse], error) {
	nodeID := req.Msg.Id
	ulidID, err := ulid.Parse(nodeID)
	if err != nil {
		return nil, err
	}

	node, err := scollection.GetCollectionNode(c.db, ulidID)
	if err != nil {
		return nil, err
	}

	nodeData := &nodedatav1.NodeApiCallData{
		Url:         node.Data.Url,
		Method:      node.Data.Method,
		Headers:     node.Data.Headers,
		QueryParams: node.Data.QueryParams,
		Body:        node.Data.Body,
	}

	respNode := &collectionv1.CollectionNode{
		Id:   node.ID.String(),
		Name: node.Name,
		Type: node.Type,
		Data: nodeData,
	}

	return connect.NewResponse(&collectionv1.GetNodeResponse{Node: respNode}), nil
}

// GetNodeBulk calls collection.v1.CollectionService.GetNodeBulk.
func (c *CollectionService) GetNodeBulk(ctx context.Context, req *connect.Request[collectionv1.GetNodeBulkRequest]) (*connect.Response[collectionv1.GetNodeBulkResponse], error) {
	nodeIds := req.Msg.Ids
	nodeIdsUlid := make([]ulid.ULID, 0, len(nodeIds))
	for _, id := range nodeIds {
		ulidID, err := ulid.Parse(id)
		if err != nil {
			return nil, err
		}
		nodeIdsUlid = append(nodeIdsUlid, ulidID)
	}

	if len(nodeIdsUlid) == 0 {
		return connect.NewResponse(&collectionv1.GetNodeBulkResponse{}), nil
	}

	var nodes []*mcollection.CollectionNode
	for _, id := range nodeIdsUlid {
		node, err := scollection.GetCollectionNode(c.db, id)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	var respNodes []*collectionv1.CollectionNode
	for _, node := range nodes {
		nodeData := &nodedatav1.NodeApiCallData{
			Url:         node.Data.Url,
			Method:      node.Data.Method,
			Headers:     node.Data.Headers,
			QueryParams: node.Data.QueryParams,
			Body:        node.Data.Body,
		}

		respNode := &collectionv1.CollectionNode{
			Id:   node.ID.String(),
			Name: node.Name,
			Type: node.Type,
			Data: nodeData,
		}
		respNodes = append(respNodes, respNode)
	}

	return connect.NewResponse(&collectionv1.GetNodeBulkResponse{Nodes: respNodes}), nil
}

// UpdateNode calls collection.v1.CollectionService.UpdateNode.
func (c *CollectionService) UpdateNode(ctx context.Context, req *connect.Request[collectionv1.UpdateNodeRequest]) (*connect.Response[collectionv1.UpdateNodeResponse], error) {
	id := req.Msg.Id
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}

	node, err := scollection.GetCollectionNode(c.db, ulidID)
	if err != nil {
		return nil, err
	}

	data := &mnodedata.NodeApiRestData{
		Url:         req.Msg.Data.Url,
		Method:      req.Msg.Data.Method,
		Headers:     req.Msg.Data.Headers,
		QueryParams: req.Msg.Data.QueryParams,
		Body:        req.Msg.Data.Body,
	}

	err = scollection.UpdateCollectionNode(c.db, ulidID, req.Msg.Name, node.Type, req.Msg.ParentId, data)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&collectionv1.UpdateNodeResponse{}), nil
}

// DeleteNode calls collection.v1.CollectionService.DeleteNode.
func (c *CollectionService) DeleteNode(ctx context.Context, req *connect.Request[collectionv1.DeleteNodeRequest]) (*connect.Response[collectionv1.DeleteNodeResponse], error) {
	id := req.Msg.Id
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}

	err = scollection.DeleteCollectionNode(c.db, ulidID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&collectionv1.DeleteNodeResponse{}), err
}

// MoveNode calls collection.v1.CollectionService.MoveNode.
func (c *CollectionService) MoveNode(ctx context.Context, req *connect.Request[collectionv1.MoveNodeRequest]) (*connect.Response[collectionv1.MoveNodeResponse], error) {
	id := req.Msg.Id // node id
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}
	parentUlidID, err := ulid.Parse(req.Msg.ParentId)
	if err != nil {
		return nil, err
	}
	collectionUlidID, err := ulid.Parse(req.Msg.CollectionId)
	if err != nil {
		return nil, err
	}

	err = scollection.MoveCollectionNode(c.db, ulidID, parentUlidID, collectionUlidID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// RunNode calls collection.v1.CollectionService.RunNode.
func (c *CollectionService) RunNode(ctx context.Context, req *connect.Request[collectionv1.RunNodeRequest]) (*connect.Response[collectionv1.RunNodeResponse], error) {
	id := req.Msg.Id
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}

	node, err := scollection.GetCollectionNode(c.db, ulidID)
	if err != nil {
		return nil, err
	}

	tempNode := &mnode.Node{
		ID:      node.ID.String(),
		Type:    resolver.ApiCallRest,
		Data:    node.Data,
		GroupID: "some-group",
		OwnerID: "some-owner-id",
	}

	nm := &mnodemaster.NodeMaster{
		CurrentNode: tempNode,
		Logger:      nil, // TODO: add logger
		HttpClient:  http.DefaultClient,
	}

	startTime := time.Now()
	err = nodeapi.SendRestApiRequest(nm)
	if err != nil {
		return nil, err
	}

	timeTaken := time.Since(startTime)

	apiRespAny, ok := nm.Vars[nodeapi.VarResponseKey]
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("cannot find response"))
	}

	apiResp, ok := apiRespAny.(*http.Response)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("cannto cast to http.Response"))
	}

	bodyBytes := make([]byte, apiResp.ContentLength)
	_, err = apiResp.Body.Read(bodyBytes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := connect.NewResponse(&collectionv1.RunNodeResponse{Status: int32(apiResp.StatusCode), Body: bodyBytes, Duration: timeTaken.Nanoseconds()})
	return resp, nil
}

func CreateService(db *sql.DB, secret []byte) (*api.Service, error) {
	server := &CollectionService{
		db:     db,
		secret: secret,
	}
	path, handler := collectionv1connect.NewCollectionServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
