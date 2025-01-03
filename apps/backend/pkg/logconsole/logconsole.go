package logconsole

import (
	"context"
	"fmt"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/pkg/idwrap"
)

type LogMessage struct {
	LogID idwrap.IDWrap
	Value string
}

type LogChanMap map[idwrap.IDWrap]chan LogMessage

func NewLogChanMap() LogChanMap {
	return make(LogChanMap)
}

func SendLogMessage(ch chan LogMessage, logID idwrap.IDWrap, value string) {
	ch <- LogMessage{
		LogID: logID,
		Value: value,
	}
}

func SendMsgToUserWithContext(ctx context.Context, logChannels LogChanMap, logID idwrap.IDWrap, value string) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return err
	}
	ch, ok := logChannels[userID]
	if !ok {
		return fmt.Errorf("userID's log channel not found")
	}
	SendLogMessage(ch, logID, value)
	return nil
}
