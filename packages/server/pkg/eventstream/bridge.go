//nolint:revive // exported
package eventstream

import "context"

// StreamToClient bridges a SyncStreamer to a client sending function.
// It handles the subscription, snapshot, and the continuous loop.
// This helper removes the need to write the select/case loop in every RPC handler.
func StreamToClient[Topic any, Payload any, Response any](
	ctx context.Context,
	streamer SyncStreamer[Topic, Payload],
	snapshot SnapshotProvider[Topic, Payload],
	filter TopicFilter[Topic],
	convert func(Payload) *Response,
	send func(*Response) error,
) error {
	events, err := streamer.Subscribe(ctx, filter, WithSnapshot(snapshot))
	if err != nil {
		return err
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			if msg := convert(evt.Payload); msg != nil {
				if err := send(msg); err != nil {
					return err
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
