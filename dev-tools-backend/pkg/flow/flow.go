package flow

import (
	"dev-tools-nodes/pkg/model/mnode"

	"github.com/google/uuid"
)

type Flows struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Nodes    []mnode.Node      `json:"nodes"`
	OwnerID  string            `json:"ownerID"`
	GroupMap map[string]string `json:"groupMap"`
}

/*
func ConvertPostmanCollection(collection mpostmancollection.Collection, ownerID string) []mnode.Node {
	uuid, err := uuid.NewV7()
	if err != nil {
		return nil
	}

	uuidStr := uuid.String()

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
			GroupID: uuidStr,
			Edges:   edges,
		}

		lastNode = &node

		nodes = append(nodes, node)
	}

	return nodes
}

*/

func CreateFlow(name string, ownerID string) Flows {
	uuid, err := uuid.NewV7()
	if err != nil {
		return Flows{}
	}
	uuidStr := uuid.String()
	return Flows{
		ID:       uuidStr,
		Name:     name,
		Nodes:    []mnode.Node{},
		OwnerID:  ownerID,
		GroupMap: map[string]string{},
	}
}

func AddNodeToFlow(flow Flows, node mnode.Node) Flows {
	flow.Nodes = append(flow.Nodes, node)
	flow.GroupMap[node.ID] = flow.ID
	return flow
}

func RemoveNodeFromFlow(flow Flows, nodeID string) Flows {
	var newNodes []mnode.Node
	for _, v := range flow.Nodes {
		if v.ID != nodeID {
			newNodes = append(newNodes, v)
		}
	}
	delete(flow.GroupMap, nodeID)
	flow.Nodes = newNodes
	return flow
}
