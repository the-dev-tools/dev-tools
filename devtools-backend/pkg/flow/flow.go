package flow

import (
	"devtools-backend/pkg/model/postman/v21/mcollection"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/resolver"

	"github.com/google/uuid"
)

type Flows struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Nodes   []mnode.Node `json:"nodes"`
	OwnerID string       `json:"ownerID"`
}

func ConvertPostmanCollection(collection mcollection.Collection, ownerID string) *Flows {
	uuid, err := uuid.NewV7()
	if err != nil {
		return nil
	}

	flow := Flows{
		ID:   uuid.String(),
		Name: collection.Info.Name,
	}

	var nodes []mnode.Node
	var lastNode *mnode.Node

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

		data := mnodedata.NodeApiRestData{
			Url:         item.Request.URL.Raw,
			Method:      item.Request.Method,
			Headers:     headers,
			QueryParams: queryParams,
			Body:        []byte(item.Request.Body.Raw),
		}

		var edges medge.Edges
		if lastNode != nil {
			edges = medge.Edges{
				OutNodes: map[string]string{medge.DefaultSuccessEdge: lastNode.ID},
			}
		}

		node := mnode.Node{
			ID:      item.ID,
			Type:    resolver.ApiCallRest,
			Data:    data,
			OwnerID: ownerID,
			GroupID: flow.ID,
			Edges:   edges,
		}

		lastNode = &node

		nodes = append(nodes, node)
	}

	flow.Nodes = nodes

	return &flow
}
