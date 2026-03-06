package flowexec

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowresult"
)

func TestLocalSessionFactory_Create(t *testing.T) {
	t.Parallel()

	factory := &LocalSessionFactory{
		Builder: nil, // Not used until Prepare
	}

	proc := flowresult.NewNoopResultProcessor(0)
	session := factory.Create(proc)

	assert.NotNil(t, session)
	_, ok := session.(*ServerSession)
	assert.True(t, ok, "factory should create ServerSession")
}

func TestLocalSessionFactory_Interface(t *testing.T) {
	t.Parallel()

	// Verify LocalSessionFactory satisfies SessionFactory at compile time
	var _ SessionFactory = (*LocalSessionFactory)(nil)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_ = logger
}
