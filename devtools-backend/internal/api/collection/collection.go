package collection

import (
	"context"
	"database/sql"
	"devtools-backend/internal/api"
	"devtools-backend/pkg/model/mcollection"
	"devtools-backend/pkg/model/postman/v21/mpostmancollection"
	"devtools-backend/pkg/service/scollection"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/resolver"
	collectionv1 "devtools-services/gen/collection/v1"
	"devtools-services/gen/collection/v1/collectionv1connect"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	"encoding/json"

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

func (c *CollectionService) Create(ctx context.Context, req *connect.Request[collectionv1.CreateRequest]) (*connect.Response[collectionv1.CreateResponse], error) {
	ulidID := ulid.Make()
	err := scollection.CreateCollection(c.db, ulidID, req.Msg.Name)
	if err != nil {
		return nil, err
	}

	respRaw := &collectionv1.CreateResponse{Id: ulidID.String()}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

func (c *CollectionService) Save(ctx context.Context, req *connect.Request[collectionv1.SaveRequest]) (*connect.Response[collectionv1.SaveResponse], error) {
	id := req.Msg.Id
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}
	err = scollection.UpdateCollection(c.db, ulidID, req.Msg.Name)
	if err != nil {
		return nil, err
	}
	respRaw := &collectionv1.SaveResponse{}
	resp := connect.NewResponse(respRaw)

	return resp, nil
}

func (c *CollectionService) Load(ctx context.Context, req *connect.Request[collectionv1.LoadRequest]) (*connect.Response[collectionv1.LoadResponse], error) {
	id := req.Msg.Id
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, err
	}
	collection, err := scollection.GetCollection(c.db, ulidID)
	if err != nil {
		return nil, err
	}

	var nodes []*collectionv1.CollectionNode
	for _, node := range *collection.Nodes {
		data := &nodedatav1.NodeApiCallData{
			Method:      node.Data.Method,
			Url:         node.Data.Url,
			Headers:     node.Data.Headers,
			Body:        node.Data.Body,
			QueryParams: node.Data.QueryParams,
		}

		nodes = append(nodes, &collectionv1.CollectionNode{
			Id:       ulidID.String(),
			Name:     node.Name,
			Type:     node.Type,
			ParentId: node.ParentID,
			Data:     data,
		})
	}

	respRaw := &collectionv1.LoadResponse{
		Id:    collection.ID.String(),
		Name:  collection.Name,
		Nodes: nodes,
	}
	resp := connect.NewResponse(respRaw)

	return resp, nil
}

func (c *CollectionService) Delete(ctx context.Context, req *connect.Request[collectionv1.DeleteRequest]) (*connect.Response[collectionv1.DeleteResponse], error) {
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

func (c *CollectionService) List(ctx context.Context, req *connect.Request[collectionv1.ListRequest]) (*connect.Response[collectionv1.ListResponse], error) {
	collections, err := scollection.ListCollections(c.db)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, id := range collections {
		ids = append(ids, id.String())
	}

	respRaw := &collectionv1.ListResponse{
		Ids: ids,
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

func (c *CollectionService) Move(ctx context.Context, req *connect.Request[collectionv1.MoveRequest]) (*connect.Response[collectionv1.MoveResponse], error) {
	return nil, nil
}

func CreateService(db *sql.DB, secret []byte) (*api.Service, error) {
	server := &CollectionService{
		db:     db,
		secret: secret,
	}
	path, handler := collectionv1connect.NewCollectionServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
