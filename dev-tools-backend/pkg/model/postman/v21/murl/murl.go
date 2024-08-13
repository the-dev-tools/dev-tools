package murl

import "dev-tools-backend/pkg/model/postman/v21/mvariable"

type URL struct {
	Version   string               `json:"version,omitempty"`
	Raw       string               `json:"raw"`
	Protocol  string               `json:"protocol,omitempty"`
	Host      []string             `json:"host,omitempty"`
	Port      string               `json:"port,omitempty"`
	Variables []mvariable.Variable `json:"variable,omitempty"`
	Path      []string             `json:"path,omitempty"`
	Query     []*QueryParamter     `json:"query,omitempty"`
	Hash      string               `json:"hash,omitempty"`
}

type QueryParamter struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}
