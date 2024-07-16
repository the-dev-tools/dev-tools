package api

import (
	"bytes"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodemaster"
	"errors"
	"net/http"
)

type RestApiData struct {
	Url         string            `json:"url"`
	QueryParams map[string]string `json:"queryParams"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
}

const (
	VarResponseKey = "response"
)

func SendRestApiRequest(nm *mnodemaster.NodeMaster) error {
	currentNode := nm.CurrentNode
	apiData, ok := currentNode.Data.(*RestApiData)
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

	client := nm.HttpClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	nodemaster.SetVar(nm, VarResponseKey, resp, currentNode.ID)
	nm.Vars[VarResponseKey] = resp

	nextNode, ok := currentNode.Edges.OutNodes[mnodemaster.EdgeSuccess]
	if !ok {
		nm.NextNodeID = ""
	}

	nm.NextNodeID = nextNode

	return nil
}
