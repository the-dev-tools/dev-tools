package rtag

import (
	"database/sql"
	"dev-tools-backend/pkg/service/sflow"
	"dev-tools-backend/pkg/service/stag"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
)

type TagServiceRPC struct {
	DB *sql.DB
	fs sflow.FlowService
	ws sworkspace.WorkspaceService
	us suser.UserService
	ts stag.TagService
}

func New(db *sql.DB, fs sflow.FlowService, ws sworkspace.WorkspaceService, us suser.UserService, ts stag.TagService) TagServiceRPC {
	return TagServiceRPC{
		DB: db,
		fs: fs,
		ws: ws,
		us: us,
		ts: ts,
	}
}
