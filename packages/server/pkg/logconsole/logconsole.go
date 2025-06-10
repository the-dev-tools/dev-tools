package logconsole

import (
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
	Refs  []reference.ReferenceTreeItem
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
	ch <- LogMessage{
		LogID: logID,
		Value: value,
		Level: level,
		Refs:  refs,
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
