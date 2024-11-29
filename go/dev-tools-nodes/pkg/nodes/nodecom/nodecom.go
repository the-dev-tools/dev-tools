package nodecom

import "dev-tools-nodes/pkg/model/mnodemaster"

func SendEmail(mn *mnodemaster.NodeMaster) error {
	mn.NextNodeID = mn.CurrentNode.Edges.OutNodes[mnodemaster.EdgeSuccess]

	return nil
}
