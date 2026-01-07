//go:build dev

package mutation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Recorder interface for event recording.
type Recorder interface {
	Record(events []Event) error
}

// fileRecorder writes events to JSONL files for replay/debugging.
type fileRecorder struct {
	dir  string
	mu   sync.Mutex
	file *os.File
	day  string
}

// newRecorder creates a file-based recorder for dev builds.
func newRecorder() Recorder {
	dir := os.Getenv("DEVTOOLS_REPLAY_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "devtools-replay")
	}
	_ = os.MkdirAll(dir, 0755)
	return &fileRecorder{dir: dir}
}

func (r *fileRecorder) Record(events []Event) error {
	if len(events) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if r.day != today {
		if r.file != nil {
			_ = r.file.Close()
		}
		f, err := os.OpenFile(
			filepath.Join(r.dir, today+".jsonl"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			0644,
		)
		if err != nil {
			return err
		}
		r.file = f
		r.day = today
	}

	batch := replayBatch{
		TS:     time.Now().UnixMilli(),
		Events: make([]replayEvent, len(events)),
	}
	for i, evt := range events {
		batch.Events[i] = replayEvent{
			Entity:      uint16(evt.Entity),
			Op:          uint8(evt.Op),
			ID:          evt.ID.String(),
			WorkspaceID: evt.WorkspaceID.String(),
			IsDelta:     evt.IsDelta,
		}
	}

	data, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	_, err = r.file.Write(append(data, '\n'))
	return err
}

type replayBatch struct {
	TS     int64         `json:"ts"`
	Events []replayEvent `json:"e"`
}

type replayEvent struct {
	Entity      uint16 `json:"t"`
	Op          uint8  `json:"op"`
	ID          string `json:"id"`
	WorkspaceID string `json:"ws"`
	IsDelta     bool   `json:"d,omitempty"`
}
