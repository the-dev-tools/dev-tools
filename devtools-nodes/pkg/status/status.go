package status

import (
	"context"
	"devtools-nodes/pkg/model/mstatus"
	"log"
)

func PushNotify(statusData mstatus.NodeStatus, notifyChan chan mstatus.NodeStatus) {
	notifyChan <- statusData
}

func ProxyNotify[T any, O any](ctx context.Context, notifyChan chan T, convertHandler func(T) (O, error), sendHandler func(T, O) error, closeChan chan bool) {
	for {
		select {
		case <-closeChan:
			return
		case <-ctx.Done():
			return
		case t := <-notifyChan:

			o, err := convertHandler(t)
			if err != nil {
				log.Fatalf("convertHandler() failed: %v", err)
			}

			err = sendHandler(t, o)
			if err != nil {
				log.Fatalf("convertHandler() failed: %v", err)
			}
		}
	}
}
