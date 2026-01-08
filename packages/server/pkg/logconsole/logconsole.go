//nolint:revive // exported
package logconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"sync"
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
	Name  string
	Level LogLevel
	// JSON contains the structured payload encoded as JSON.
	JSON string
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

func SendLogMessage(ch chan LogMessage, logID idwrap.IDWrap, name string, level LogLevel, payload map[string]any) {
	ch <- LogMessage{
		LogID: logID,
		Name:  name,
		Level: level,
		JSON:  payloadToJSON(payload),
	}
}

func (logChannels *LogChanMap) SendMsgToUserWithContext(ctx context.Context, logID idwrap.IDWrap, name string, level LogLevel, payload map[string]any) error {
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
	SendLogMessage(ch, logID, name, level, payload)
	return nil
}

func payloadToJSON(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	by, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(by)
}
