package logconsole

import (
	"context"
	"fmt"
	"sync"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/pkg/idwrap"
)

type LogMessage struct {
	LogID idwrap.IDWrap
	Value string
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

func SendLogMessage(ch chan LogMessage, logID idwrap.IDWrap, value string) {
	ch <- LogMessage{
		LogID: logID,
		Value: value,
	}
}

func (logChannels *LogChanMap) SendMsgToUserWithContext(ctx context.Context, logID idwrap.IDWrap, value string) error {
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
	SendLogMessage(ch, logID, value)
	return nil
}
