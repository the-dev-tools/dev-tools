package logconsole

import (
    "encoding/json"
    "context"
    "fmt"
    "sync"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/reference"
)

// LogLevel represents the severity level of a log message
type LogLevel int32

const (
	LogLevelUnspecified LogLevel = 0
	LogLevelWarning     LogLevel = 1
	LogLevelError       LogLevel = 2
)

type LogMessage struct {
    LogID idwrap.IDWrap
    Value string
    Level LogLevel
    // JSON contains the structured payload encoded as JSON.
    JSON  string
}

type LogChanMap struct {
	mt      *sync.Mutex
	chanMap map[idwrap.IDWrap]chan LogMessage
}

func NewLogChanMap() LogChanMap {
	chanMap := make(map[idwrap.IDWrap]chan LogMessage, 10)
	return LogChanMap{
		chanMap: chanMap,
		mt:      &sync.Mutex{},
	}
}

func NewLogChanMapWith(size int) LogChanMap {
	chanMap := make(map[idwrap.IDWrap]chan LogMessage, size)
	return LogChanMap{
		chanMap: chanMap,
		mt:      &sync.Mutex{},
	}
}

const bufferSize = 10

func (l *LogChanMap) AddLogChannel(userID idwrap.IDWrap) chan LogMessage {
	lm := make(chan LogMessage, bufferSize)
	l.mt.Lock()
	defer l.mt.Unlock()
	l.chanMap[userID] = lm
	return lm
}

func (l *LogChanMap) DeleteLogChannel(userID idwrap.IDWrap) {
	l.mt.Lock()
	defer l.mt.Unlock()
	delete(l.chanMap, userID)
}

func SendLogMessage(ch chan LogMessage, logID idwrap.IDWrap, value string, level LogLevel, refs []reference.ReferenceTreeItem) {
    // Convert refs to a single JSON object for downstream consumers.
    jsonStr := refsToJSON(refs)
    ch <- LogMessage{
        LogID: logID,
        Value: value,
        Level: level,
        JSON:  jsonStr,
    }
}

func (logChannels *LogChanMap) SendMsgToUserWithContext(ctx context.Context, logID idwrap.IDWrap, value string, level LogLevel, refs []reference.ReferenceTreeItem) error {
    logChannels.mt.Lock()
    defer logChannels.mt.Unlock()
    userID, err := mwauth.GetContextUserID(ctx)
    if err != nil {
        return err
	}
	ch, ok := logChannels.chanMap[userID]
	if !ok {
		return fmt.Errorf("userID's log channel not found")
    }
    SendLogMessage(ch, logID, value, level, refs)
    return nil
}

// refsToJSON serializes a slice of ReferenceTreeItem to a single JSON object string
// by merging each ref as a top-level key in the resulting object.
func refsToJSON(refs []reference.ReferenceTreeItem) string {
    if len(refs) == 0 {
        return ""
    }
    obj := make(map[string]any)
    for _, r := range refs {
        m := refToMap(r)
        for k, v := range m {
            obj[k] = v
        }
    }
    by, err := json.Marshal(obj)
    if err != nil {
        return ""
    }
    return string(by)
}

// refToMap converts a ReferenceTreeItem into a nested map[string]any/[]any
// keyed by the item's key.
func refToMap(ref reference.ReferenceTreeItem) map[string]any {
    out := make(map[string]any)
    switch ref.Kind {
    case reference.ReferenceKind_REFERENCE_KIND_MAP:
        m := make(map[string]any)
        for _, child := range ref.Map {
            childMap := refToMap(child)
            if child.Key.Key != "" {
                m[child.Key.Key] = childMap[child.Key.Key]
            }
        }
        if ref.Key.Key != "" {
            out[ref.Key.Key] = m
        }
    case reference.ReferenceKind_REFERENCE_KIND_ARRAY:
        arr := make([]any, len(ref.Array))
        for i := range ref.Array {
            child := ref.Array[i]
            childMap := refToMap(child)
            if len(childMap) == 1 {
                for _, v := range childMap {
                    arr[i] = v
                }
            } else {
                arr[i] = childMap
            }
        }
        if ref.Key.Key != "" {
            out[ref.Key.Key] = arr
        }
    case reference.ReferenceKind_REFERENCE_KIND_VALUE:
        out[ref.Key.Key] = ref.Value
    default:
        out[ref.Key.Key] = nil
    }
    return out
}
