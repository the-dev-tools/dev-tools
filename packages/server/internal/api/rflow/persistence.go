package rflow

import (
    "context"
    "time"
    snodeexecution "the-dev-tools/server/pkg/service/snodeexecution"
    "the-dev-tools/server/pkg/model/mnodeexecution"
)

// persistUpsert2s wraps upsertWithRetry with a 2s context timeout.
func persistUpsert2s(svc snodeexecution.NodeExecutionService, exec mnodeexecution.NodeExecution) error {
    dbCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    return upsertWithRetry(dbCtx, svc, exec)
}

