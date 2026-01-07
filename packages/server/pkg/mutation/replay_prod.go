//go:build !dev

package mutation

// Recorder interface for event recording.
// In production builds, this is a no-op.
type Recorder interface {
	Record(events []Event) error
}

// newRecorder returns nil in production - no allocation, no overhead.
func newRecorder() Recorder {
	return nil
}
