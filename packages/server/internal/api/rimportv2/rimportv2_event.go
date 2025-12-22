package rimportv2

import (
	"context"

	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/converter"
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
				// We don't typically deduplicate nodes yet as they are unique to each flow,
				// but let's check for consistency if we ever do.
				topic := rflowv2.NodeTopic{FlowID: node.FlowID}
				grouped[topic] = append(grouped[topic], rflowv2.NodeEvent{
					Type: "insert",
					Node: &flowv1.Node{
						NodeId: node.ID.Bytes(),
						FlowId: node.FlowID.Bytes(),
						Name:   node.Name,
						Kind:   converter.ToAPINodeKind(node.NodeKind),
						Position: &flowv1.Position{
							X: float32(node.PositionX),
							Y: float32(node.PositionY),
						},
					},
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
                				}            }

            // Publish HTTP events
	if len(results.HTTPReqs) > 0 {
		grouped := make(map[rhttp.HttpTopic][]rhttp.HttpEvent)
		for _, httpReq := range results.HTTPReqs {
			// Skip if deduplicated
			if results.DeduplicatedHTTPReqs[httpReq.ID] {
				continue
			}
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
			// Skip if deduplicated
			if results.DeduplicatedFiles[file.ID] {
				continue
			}
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
		var events []rhttp.HttpHeaderEvent
		for _, header := range results.HTTPHeaders {
			if results.DeduplicatedHTTPReqs[header.HttpID] {
				continue
			}
			events = append(events, rhttp.HttpHeaderEvent{
				Type:       "insert",
				IsDelta:    header.IsDelta,
				HttpHeader: converter.ToAPIHttpHeader(*header),
			})
		}
		if len(events) > 0 {
			h.HttpHeaderStream.Publish(topic, events...)
		}
	}

	// Publish SearchParam events
	if len(results.HTTPSearchParams) > 0 {
		topic := rhttp.HttpSearchParamTopic{WorkspaceID: results.WorkspaceID}
		var events []rhttp.HttpSearchParamEvent
		for _, param := range results.HTTPSearchParams {
			if results.DeduplicatedHTTPReqs[param.HttpID] {
				continue
			}
			events = append(events, rhttp.HttpSearchParamEvent{
				Type:            "insert",
				IsDelta:         param.IsDelta,
				HttpSearchParam: converter.ToAPIHttpSearchParamFromMHttp(*param),
			})
		}
		if len(events) > 0 {
			h.HttpSearchParamStream.Publish(topic, events...)
		}
	}

	// Publish BodyForm events
	if len(results.HTTPBodyForms) > 0 {
		topic := rhttp.HttpBodyFormTopic{WorkspaceID: results.WorkspaceID}
		var events []rhttp.HttpBodyFormEvent
		for _, form := range results.HTTPBodyForms {
			if results.DeduplicatedHTTPReqs[form.HttpID] {
				continue
			}
			events = append(events, rhttp.HttpBodyFormEvent{
				Type:         "insert",
				IsDelta:      form.IsDelta,
				HttpBodyForm: converter.ToAPIHttpBodyFormDataFromMHttp(*form),
			})
		}
		if len(events) > 0 {
			h.HttpBodyFormStream.Publish(topic, events...)
		}
	}

	// Publish BodyUrlEncoded events
	if len(results.HTTPBodyUrlEncoded) > 0 {
		topic := rhttp.HttpBodyUrlEncodedTopic{WorkspaceID: results.WorkspaceID}
		var events []rhttp.HttpBodyUrlEncodedEvent
		for _, encoded := range results.HTTPBodyUrlEncoded {
			if results.DeduplicatedHTTPReqs[encoded.HttpID] {
				continue
			}
			events = append(events, rhttp.HttpBodyUrlEncodedEvent{
				Type:               "insert",
				IsDelta:            encoded.IsDelta,
				HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncodedFromMHttp(*encoded),
			})
		}
		if len(events) > 0 {
			h.HttpBodyUrlEncodedStream.Publish(topic, events...)
		}
	}

	// Publish BodyRaw events
	if len(results.HTTPBodyRaws) > 0 {
		topic := rhttp.HttpBodyRawTopic{WorkspaceID: results.WorkspaceID}
		var events []rhttp.HttpBodyRawEvent
		for _, raw := range results.HTTPBodyRaws {
			if results.DeduplicatedHTTPReqs[raw.HttpID] {
				continue
			}
			events = append(events, rhttp.HttpBodyRawEvent{
				Type:        "insert",
				IsDelta:     raw.IsDelta,
				HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(*raw),
			})
		}
		if len(events) > 0 {
			h.HttpBodyRawStream.Publish(topic, events...)
		}
	}

	// Publish Assert events
	if len(results.HTTPAsserts) > 0 {
		topic := rhttp.HttpAssertTopic{WorkspaceID: results.WorkspaceID}
		var events []rhttp.HttpAssertEvent
		for _, assert := range results.HTTPAsserts {
			if results.DeduplicatedHTTPReqs[assert.HttpID] {
				continue
			}
			events = append(events, rhttp.HttpAssertEvent{
				Type:       "insert",
				IsDelta:    assert.IsDelta,
				HttpAssert: converter.ToAPIHttpAssert(*assert),
			})
		}
		if len(events) > 0 {
			h.HttpAssertStream.Publish(topic, events...)
		}
	}

	// Publish Environment events (if a default environment was created during domain variable import)
	if len(results.CreatedEnvs) > 0 && h.EnvStream != nil {
		for _, env := range results.CreatedEnvs {
			h.EnvStream.Publish(renv.EnvironmentTopic{WorkspaceID: env.WorkspaceID}, renv.EnvironmentEvent{
				Type:        "insert",
				Environment: converter.ToAPIEnvironment(env),
			})
		}
	}

	// Publish Environment Variable events (for domain-to-variable mappings created during import)
	if len(results.CreatedVars) > 0 && h.EnvVarStream != nil {
		for _, v := range results.CreatedVars {
			h.EnvVarStream.Publish(renv.EnvironmentVariableTopic{WorkspaceID: results.WorkspaceID, EnvironmentID: v.EnvID}, renv.EnvironmentVariableEvent{
				Type:     "insert",
				Variable: converter.ToAPIEnvironmentVariable(v),
			})
		}
	}

	// Publish Environment Variable update events (for variables that already existed and were updated)
	if len(results.UpdatedVars) > 0 && h.EnvVarStream != nil {
		for _, v := range results.UpdatedVars {
			h.EnvVarStream.Publish(renv.EnvironmentVariableTopic{WorkspaceID: results.WorkspaceID, EnvironmentID: v.EnvID}, renv.EnvironmentVariableEvent{
				Type:     "update",
				Variable: converter.ToAPIEnvironmentVariable(v),
			})
		}
	}
}
