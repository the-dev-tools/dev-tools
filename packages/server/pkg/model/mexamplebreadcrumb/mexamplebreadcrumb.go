package mexamplebreadcrumb

import (
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
)

type ExampleBreadcrumbKind = uint8

const (
	EXAMPLE_BREADCRUMB_KIND_UNSPECIFIED ExampleBreadcrumbKind = iota
	EXAMPLE_BREADCRUMB_KIND_COLLECTION
	EXAMPLE_BREADCRUMB_KIND_FOLDER
	EXAMPLE_BREADCRUMB_KIND_ENDPOINT
	EXAMPLE_BREADCRUMB_KIND_EXAMPLE
)

type ExampleBreadcrumb struct {
	Kind       ExampleBreadcrumbKind
	Collection *mcollection.Collection
	Folder     *mitemfolder.ItemFolder
	Endpoint   *mitemapi.ItemApi
	Example    *mitemapiexample.ItemApiExample
}
