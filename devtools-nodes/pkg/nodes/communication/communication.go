package communication

import "github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"

type EmailData struct {
	To string `json:"to"`
}

func SendEmail(mn *mnodemaster.NodeMaster) error {
	mn.NextNodeID = mn.CurrentNode.Edges.OutNodes[mnodemaster.EdgeSuccess]

	return nil
}
