package rflow

import (
    "context"
    "strings"
    "time"
    snodeexecution "the-dev-tools/server/pkg/service/snodeexecution"
    "the-dev-tools/server/pkg/model/mnodeexecution"
)

// upsertWithRetry attempts to upsert a node execution with small retries for transient DB lock/busy errors.
func upsertWithRetry(ctx context.Context, svc snodeexecution.NodeExecutionService, exec mnodeexecution.NodeExecution) error {
    // Try immediate, then exponential backoff for transient sqlite busy/locked
    backoffs := []time.Duration{0, 10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond, 80 * time.Millisecond, 160 * time.Millisecond, 320 * time.Millisecond}
    var lastErr error
    for i, d := range backoffs {
        if d > 0 {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(d):
            }
        }
        if err := svc.UpsertNodeExecution(ctx, exec); err != nil {
            lastErr = err
            // Retry on transient lock/busy conditions
            msg := err.Error()
            if !(strings.Contains(msg, "locked") || strings.Contains(msg, "busy")) {
                return err
            }
            // If this was the last attempt, return error
            if i == len(backoffs)-1 {
                return err
            }
            continue
        }
        return nil
    }
    return lastErr
}

