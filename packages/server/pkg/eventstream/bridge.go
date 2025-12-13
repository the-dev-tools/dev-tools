//nolint:revive // exported
package eventstream

import (
	"context"
	"time"
)

// BulkOptions configures the batching behavior for StreamToClient.
type BulkOptions struct {
	MaxBatchSize  int
	FlushInterval time.Duration
}

// StreamToClient bridges a SyncStreamer to a client sending function.
// It handles the subscription, snapshot, and the continuous loop.
// It supports batching events before sending to the client.
func StreamToClient[Topic any, Payload any, Response any](
	ctx context.Context,
	streamer SyncStreamer[Topic, Payload],
	snapshot SnapshotProvider[Topic, Payload],
	filter TopicFilter[Topic],
	convert func([]Payload) *Response,
	send func(*Response) error,
	opts *BulkOptions,
) error {
	// Defaults
	maxBatchSize := 100
	flushInterval := 50 * time.Millisecond

	if opts != nil {
		if opts.MaxBatchSize > 0 {
			maxBatchSize = opts.MaxBatchSize
		}
		if opts.FlushInterval > 0 {
			flushInterval = opts.FlushInterval
		}
	}

	events, err := streamer.Subscribe(ctx, filter, WithSnapshot(snapshot))
	if err != nil {
		return err
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	// Pre-allocate buffer
	buffer := make([]Payload, 0, maxBatchSize)

	flush := func() error {
		if len(buffer) == 0 {
			return nil
		}
		if msg := convert(buffer); msg != nil {
			if err := send(msg); err != nil {
				return err
			}
		}
		// Clear buffer while keeping capacity
		buffer = buffer[:0]
		return nil
	}

	// Ensure we flush any remaining items on exit
	defer func() {
		_ = flush()
	}()

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			buffer = append(buffer, evt.Payload)
			if len(buffer) >= maxBatchSize {
				if err := flush(); err != nil {
					return err
				}
				// Reset ticker to avoid immediate double-flush
				ticker.Reset(flushInterval)
			}
		case <-ticker.C:
			if err := flush(); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
