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

	for key, value := range apiData.Headers {
		req.Header.Add(key, value)
	}

	log.Printf("Sending request to: %s", apiData.Url)
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
