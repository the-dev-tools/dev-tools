package rexportv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
)

// TestNewStorage tests the storage constructor
func TestNewStorage(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	services := base.GetBaseServices()
	logger := base.Logger()

	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.NewFlowService(base.Queries)
	fileService := sfile.New(base.Queries, logger)

	storage := NewStorage(&services.WorkspaceService, &httpService, &flowService, fileService)

	require.NotNil(t, storage)
}
