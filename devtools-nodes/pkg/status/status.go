package status

import "devtools-nodes/pkg/model/mstatus"

func PushNotify(statusData interface{}, notifyChan chan mstatus.NodeStatus) {
	notifyChan <- statusData.(mstatus.NodeStatus)
}
