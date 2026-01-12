package nai

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
)

func TestVariableTools(t *testing.T) {
	ctx := context.Background()
	tracker := tracking.NewVariableTracker()
	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"existing": "value",
		},
		ReadWriteLock:   &sync.RWMutex{},
		VariableTracker: tracker,
	}

	t.Run("get_variable", func(t *testing.T) {
		res, err := handleGetVariable(ctx, req, `{"key": "existing"}`)
		assert.NoError(t, err)
		assert.Equal(t, `"value"`, res)

		res, err = handleGetVariable(ctx, req, `{"key": "missing"}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("set_variable", func(t *testing.T) {
		res, err := handleSetVariable(ctx, req, `{"key": "new", "value": 123}`)
		assert.NoError(t, err)
		assert.Contains(t, res, "Successfully")

		val, err := node.ReadVarRaw(req, "new")
		assert.NoError(t, err)
		assert.Equal(t, float64(123), val) // json.Unmarshal defaults to float64 for numbers
	})
}
