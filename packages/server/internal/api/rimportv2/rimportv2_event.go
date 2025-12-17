package rimportv2

import (
	"context"

	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mvar"
	environmentv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// publishEvents publishes real-time sync events for imported entities
func (h *ImportV2RPC) publishEvents(ctx context.Context, results *ImportResults) {
	// Publish Flow event FIRST if present
	if results.Flow != nil {
		flowPB := &flowv1.Flow{
			FlowId: results.Flow.ID.Bytes(),
			Name:   results.Flow.Name,
		}
		if results.Flow.Duration != 0 {
			d := results.Flow.Duration
			flowPB.Duration = &d
		}

		h.FlowStream.Publish(rflowv2.FlowTopic{WorkspaceID: results.Flow.WorkspaceID}, rflowv2.FlowEvent{
			Type: "insert",
			Flow: flowPB,
		})

		// Publish Nodes events
		if len(results.Nodes) > 0 {
			grouped := make(map[rflowv2.NodeTopic][]rflowv2.NodeEvent)
			for _, node := range results.Nodes {
				nodePB := &flowv1.Node{
					NodeId: node.ID.Bytes(),
					FlowId: node.FlowID.Bytes(),
					Name:   node.Name,
					Kind:   converter.ToAPINodeKind(node.NodeKind),
					Position: &flowv1.Position{
						X: float32(node.PositionX),
						Y: float32(node.PositionY),
					},
				}
				topic := rflowv2.NodeTopic{FlowID: node.FlowID}
				grouped[topic] = append(grouped[topic], rflowv2.NodeEvent{
					Type: "insert",
					Node: nodePB,
				})
			}
			for topic, events := range grouped {
				h.NodeStream.Publish(topic, events...)
			}
		}

		// Publish Edges events
		if len(results.Edges) > 0 {
			grouped := make(map[rflowv2.EdgeTopic][]rflowv2.EdgeEvent)
			for _, edge := range results.Edges {
				edgePB := &flowv1.Edge{
					EdgeId:       edge.ID.Bytes(),
					FlowId:       edge.FlowID.Bytes(),
					SourceId:     edge.SourceID.Bytes(),
					TargetId:     edge.TargetID.Bytes(),
					SourceHandle: flowv1.HandleKind(edge.SourceHandler),
					Kind:         flowv1.EdgeKind(edge.Kind),
				}
				topic := rflowv2.EdgeTopic{FlowID: edge.FlowID}
				grouped[topic] = append(grouped[topic], rflowv2.EdgeEvent{
					Type: "insert",
					Edge: edgePB,
				})
			}
			for topic, events := range grouped {
				h.EdgeStream.Publish(topic, events...)
			}
		}

		// Publish NoOpNodes events
		if len(results.NoOpNodes) > 0 {
			// NoOp nodes are typically scoped to a flow, but let's be safe and group
			// Note: The original code assumed results.Flow.ID, but NoOpNode has FlowNodeID which implies it belongs to a flow.
			// However, mnnoop.NoOpNode doesn't explicitly store FlowID in the struct passed here usually?
			// Let's look at the struct definition if needed.
			// Assuming they belong to results.Flow since they are part of the import results for that flow.
			events := make([]rflowv2.NoOpEvent, len(results.NoOpNodes))
			for i, noOpNode := range results.NoOpNodes {
				noOpPB := &flowv1.NodeNoOp{
					NodeId: noOpNode.FlowNodeID.Bytes(),
					Kind:   converter.ToAPINodeNoOpKind(noOpNode.Type),
				}
				events[i] = rflowv2.NoOpEvent{
					Type:   "insert",
					FlowID: results.Flow.ID,
					Node:   noOpPB,
				}
			}
			h.NoopStream.Publish(rflowv2.NoOpTopic{FlowID: results.Flow.ID}, events...)
		}
	}
	
	// Publish HTTP events
	if len(results.HTTPReqs) > 0 {
		grouped := make(map[rhttp.HttpTopic][]rhttp.HttpEvent)
		for _, httpReq := range results.HTTPReqs {
			topic := rhttp.HttpTopic{WorkspaceID: httpReq.WorkspaceID}
			grouped[topic] = append(grouped[topic], rhttp.HttpEvent{
				Type:    "insert",
				IsDelta: httpReq.IsDelta,
				Http:    converter.ToAPIHttp(*httpReq),
			})
		}
		for topic, events := range grouped {
			h.HttpStream.Publish(topic, events...)
		}
	}

	// Publish File events
	if len(results.Files) > 0 {
		grouped := make(map[rfile.FileTopic][]rfile.FileEvent)
		for _, file := range results.Files {
			topic := rfile.FileTopic{WorkspaceID: file.WorkspaceID}
			grouped[topic] = append(grouped[topic], rfile.FileEvent{
				Type: "create",
				File: converter.ToAPIFile(*file),
				Name: file.Name,
			})
		}
		for topic, events := range grouped {
			h.FileStream.Publish(topic, events...)
		}
	}

	// Publish Header events
	if len(results.HTTPHeaders) > 0 {
		topic := rhttp.HttpHeaderTopic{WorkspaceID: results.WorkspaceID}
		events := make([]rhttp.HttpHeaderEvent, len(results.HTTPHeaders))
		for i, header := range results.HTTPHeaders {
			events[i] = rhttp.HttpHeaderEvent{
				Type:       "insert",
				IsDelta:    header.IsDelta,
				HttpHeader: converter.ToAPIHttpHeader(*header),
			}
		}
		h.HttpHeaderStream.Publish(topic, events...)
	}

	// Publish SearchParam events
	if len(results.HTTPSearchParams) > 0 {
		topic := rhttp.HttpSearchParamTopic{WorkspaceID: results.WorkspaceID}
		events := make([]rhttp.HttpSearchParamEvent, len(results.HTTPSearchParams))
		for i, param := range results.HTTPSearchParams {
			events[i] = rhttp.HttpSearchParamEvent{
				Type:            "insert",
				IsDelta:         param.IsDelta,
				HttpSearchParam: converter.ToAPIHttpSearchParamFromMHttp(*param),
			}
		}
		h.HttpSearchParamStream.Publish(topic, events...)
	}

	// Publish BodyForm events
	if len(results.HTTPBodyForms) > 0 {
		topic := rhttp.HttpBodyFormTopic{WorkspaceID: results.WorkspaceID}
		events := make([]rhttp.HttpBodyFormEvent, len(results.HTTPBodyForms))
		for i, form := range results.HTTPBodyForms {
			events[i] = rhttp.HttpBodyFormEvent{
				Type:         "insert",
				IsDelta:      form.IsDelta,
				HttpBodyForm: converter.ToAPIHttpBodyFormDataFromMHttp(*form),
			}
		}
		h.HttpBodyFormStream.Publish(topic, events...)
	}

	// Publish BodyUrlEncoded events
	if len(results.HTTPBodyUrlEncoded) > 0 {
		topic := rhttp.HttpBodyUrlEncodedTopic{WorkspaceID: results.WorkspaceID}
		events := make([]rhttp.HttpBodyUrlEncodedEvent, len(results.HTTPBodyUrlEncoded))
		for i, encoded := range results.HTTPBodyUrlEncoded {
			events[i] = rhttp.HttpBodyUrlEncodedEvent{
				Type:               "insert",
				IsDelta:            encoded.IsDelta,
				HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncodedFromMHttp(*encoded),
			}
		}
		h.HttpBodyUrlEncodedStream.Publish(topic, events...)
	}

	// Publish BodyRaw events
	if len(results.HTTPBodyRaws) > 0 {
		topic := rhttp.HttpBodyRawTopic{WorkspaceID: results.WorkspaceID}
		events := make([]rhttp.HttpBodyRawEvent, len(results.HTTPBodyRaws))
		for i, raw := range results.HTTPBodyRaws {
			events[i] = rhttp.HttpBodyRawEvent{
				Type:        "insert",
				IsDelta:     raw.IsDelta,
				HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(*raw),
			}
		}
		h.HttpBodyRawStream.Publish(topic, events...)
	}

	// Publish Assert events
	if len(results.HTTPAsserts) > 0 {
		topic := rhttp.HttpAssertTopic{WorkspaceID: results.WorkspaceID}
		events := make([]rhttp.HttpAssertEvent, len(results.HTTPAsserts))
		for i, assert := range results.HTTPAsserts {
			events[i] = rhttp.HttpAssertEvent{
				Type:       "insert",
				IsDelta:    assert.IsDelta,
				HttpAssert: converter.ToAPIHttpAssert(*assert),
			}
		}
		h.HttpAssertStream.Publish(topic, events...)
	}

	// Publish Environment events (if a default environment was created during domain variable import)
	if len(results.CreatedEnvs) > 0 && h.EnvStream != nil {
		for _, env := range results.CreatedEnvs {
			h.EnvStream.Publish(renv.EnvironmentTopic{WorkspaceID: env.WorkspaceID}, renv.EnvironmentEvent{
				Type:        "insert",
				Environment: toAPIEnvironment(env),
			})
		}
	}

	// Publish Environment Variable events (for domain-to-variable mappings created during import)
	if len(results.CreatedVars) > 0 && h.EnvVarStream != nil {
		for _, v := range results.CreatedVars {
			h.EnvVarStream.Publish(renv.EnvironmentVariableTopic{WorkspaceID: results.WorkspaceID, EnvironmentID: v.EnvID}, renv.EnvironmentVariableEvent{
				Type:     "insert",
				Variable: toAPIEnvironmentVariable(v),
			})
		}
	}
}

// toAPIEnvironment converts internal environment model to API type
func toAPIEnvironment(env menv.Env) *environmentv1.Environment {
	return &environmentv1.Environment{
		EnvironmentId: env.ID.Bytes(),
		WorkspaceId:   env.WorkspaceID.Bytes(),
		Name:          env.Name,
		Description:   env.Description,
		IsGlobal:      env.Type == menv.EnvGlobal,
		Order:         float32(env.Order),
	}
}

// toAPIEnvironmentVariable converts internal variable model to API type
func toAPIEnvironmentVariable(v mvar.Var) *environmentv1.EnvironmentVariable {
	return &environmentv1.EnvironmentVariable{
		EnvironmentVariableId: v.ID.Bytes(),
		EnvironmentId:         v.EnvID.Bytes(),
		Key:                   v.VarKey,
		Enabled:               v.Enabled,
		Value:                 v.Value,
		Description:           v.Description,
		Order:                 float32(v.Order),
	}
}
