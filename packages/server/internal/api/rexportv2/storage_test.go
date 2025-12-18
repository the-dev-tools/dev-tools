package rexportv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/testutil"
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

	storage := NewStorage(&services.Ws, &httpService, &flowService, fileService)

	require.NotNil(t, storage)
}
