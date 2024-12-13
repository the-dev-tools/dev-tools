package rnode

import (
	"database/sql"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snoderequest"
)

type NodeServiceRPC struct {
	DB  *sql.DB
	nis snodeif.NodeIfService
	nrs snoderequest.NodeRequestService
	nlf snodefor.NodeForService
}
