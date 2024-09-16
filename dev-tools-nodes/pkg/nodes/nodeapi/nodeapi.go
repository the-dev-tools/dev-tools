package nodeapi

import (
	"bytes"
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodemaster"
	"errors"
	"log"
	"net/http"
)

const (
	VarResponseKey = "response"
)

func SendRestApiRequest(nm *mnodemaster.NodeMaster) error {
	currentNode := nm.CurrentNode
	apiData, ok := currentNode.Data.(*mnodedata.NodeApiRestData)
	if !ok {
		return errors.New("invalid data type")
	}

	bodyReader := bytes.NewBuffer(apiData.Body)

	req, err := http.NewRequest(apiData.Method, apiData.Url, bodyReader)
	if err != nil {
		return err
	}

	for _, v := range apiData.Headers {
		req.Header.Add(v.HeaderKey, v.Value)
	}

	queries := req.URL.Query()
	if len(queries) != 0 {
		for _, v := range apiData.Query {
			queries.Add(v.QueryKey, v.Value)
		}
		req.URL.RawQuery = queries.Encode()
	}

	client := nm.HttpClient
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error: %v", err)
		// TODO: Add error saving to masternode
		nm.Vars["err"] = err
	}

	nodemaster.SetVar(nm, VarResponseKey, resp, currentNode.ID)
	nm.Vars[VarResponseKey] = resp

	nextNode, ok := currentNode.Edges.OutNodes[mnodemaster.EdgeSuccess]
	if !ok {
		nm.NextNodeID = ""
	}

	nodemaster.SetNextNode(nm, nextNode)

	return nil
}

func GetHttpVarResponse(nm *mnodemaster.NodeMaster) (*http.Response, error) {
	rawResponse, err := nodemaster.GetVar(nm, VarResponseKey)
	if err != nil {
		return nil, err
	}
	response, ok := rawResponse.(*http.Response)
	if !ok {
		return nil, errors.New("invalid data type")
	}
	return response, nil
}
