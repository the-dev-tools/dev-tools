package rimportv2

import (
	"context"

	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventsync"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// publishEvents publishes real-time sync events for imported entities.
// Uses the eventsync package for dependency-based ordering, ensuring
// entities are published in the correct order for frontend TanStack DB.
//
// Event order (computed automatically by eventsync.Dependencies):
//  1. Flow event (root - no dependencies)
//  2. Environment event (root - no dependencies)
//  3. Flow file events (depend on Flow)
//  4. Node events (depend on Flow, ordered by graph level)
//  5. Edge events (depend on Node)
//  6. HTTP events (depend on Node)
//  7. HTTP file events (depend on HTTP)
//  8. HTTP child events: headers, params, bodies, asserts (depend on HTTP)
//  9. Environment Variable events (depend on Environment)
func (h *ImportV2RPC) publishEvents(ctx context.Context, results *ImportResults) {
	batch := eventsync.NewEventBatch()

	// Add Flow event
	if results.Flow != nil {
		flow := results.Flow // capture for closure
		batch.AddSimple(eventsync.KindFlow, func() {
			flowPB := &flowv1.Flow{
				FlowId: flow.ID.Bytes(),
				Name:   flow.Name,
			}
			if flow.Duration != 0 {
				d := flow.Duration
				flowPB.Duration = &d
			}

			h.FlowStream.Publish(rflowv2.FlowTopic{WorkspaceID: flow.WorkspaceID}, rflowv2.FlowEvent{
				Type: "insert",
				Flow: flowPB,
			})
		})
	}

	// Add Environment events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindEnvironment, h.EnvStream, renv.EnvironmentTopic{WorkspaceID: results.WorkspaceID}, results.CreatedEnvs, func(env menv.Env) renv.EnvironmentEvent {
		return renv.EnvironmentEvent{
			Type:        "insert",
			Environment: converter.ToAPIEnvironment(env),
		}
	})

	// Add File events (both flow files and other files)
	// The eventsync package ensures flow files (KindFlowFile) are published before other files (KindHTTPFile)
	for _, file := range results.Files {
		if results.DeduplicatedFiles[file.ID] {
			continue
		}

		kind := eventsync.KindHTTPFile
		if file.ContentType == mfile.ContentTypeFlow {
			kind = eventsync.KindFlowFile
		} else if file.ContentType == mfile.ContentTypeFolder {
			kind = eventsync.KindFolder
		}

		batch.AddSimple(kind, func() {
			h.FileStream.Publish(rfile.FileTopic{WorkspaceID: file.WorkspaceID}, rfile.FileEvent{
				Type: "create",
				File: converter.ToAPIFile(*file),
				Name: file.Name,
			})
		})
	}

	// Add Node events (with graph-based ordering via subOrder)
	if len(results.Nodes) > 0 {
		nodeOrder := eventsync.ComputeNodeOrder(results.Nodes, results.Edges)
		nodeLevel := make(map[string]int)
		for i, id := range nodeOrder {
			nodeLevel[id.String()] = i
		}

		for _, node := range results.Nodes {
			level := nodeLevel[node.ID.String()]
			batch.Add(eventsync.KindNode, level, func() {
				h.NodeStream.Publish(rflowv2.NodeTopic{FlowID: node.FlowID}, rflowv2.NodeEvent{
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
			})
		}
	}

	// Add Edge events (only if flow exists)
	if results.Flow != nil && len(results.Edges) > 0 {
		eventsync.AddSyncTransformSimple(batch, eventsync.KindEdge, h.EdgeStream, rflowv2.EdgeTopic{FlowID: results.Flow.ID}, results.Edges, func(edge mflow.Edge) rflowv2.EdgeEvent {
			return rflowv2.EdgeEvent{
				Type:   "insert",
				FlowID: edge.FlowID,
				Edge: &flowv1.Edge{
					EdgeId:       edge.ID.Bytes(),
					FlowId:       edge.FlowID.Bytes(),
					SourceId:     edge.SourceID.Bytes(),
					TargetId:     edge.TargetID.Bytes(),
					SourceHandle: flowv1.HandleKind(edge.SourceHandler),
				},
			}
		})
	}

	// Filter HTTP requests for deduplication
	var filteredHTTP []*mhttp.HTTP
	for _, req := range results.HTTPReqs {
		if !results.DeduplicatedHTTPReqs[req.ID] {
			filteredHTTP = append(filteredHTTP, req)
		}
	}

	// Add HTTP events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindHTTP, h.HttpStream, rhttp.HttpTopic{WorkspaceID: results.WorkspaceID}, filteredHTTP, func(httpReq *mhttp.HTTP) rhttp.HttpEvent {
		return rhttp.HttpEvent{
			Type:    "insert",
			IsDelta: httpReq.IsDelta,
			Http:    converter.ToAPIHttp(*httpReq),
		}
	})

	// Add HTTP Header events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindHTTPHeader, h.HttpHeaderStream, rhttp.HttpHeaderTopic{WorkspaceID: results.WorkspaceID}, results.HTTPHeaders, func(header *mhttp.HTTPHeader) rhttp.HttpHeaderEvent {
		return rhttp.HttpHeaderEvent{
			Type:       "insert",
			IsDelta:    header.IsDelta,
			HttpHeader: converter.ToAPIHttpHeader(*header),
		}
	})

	// Add HTTP SearchParam events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindHTTPParam, h.HttpSearchParamStream, rhttp.HttpSearchParamTopic{WorkspaceID: results.WorkspaceID}, results.HTTPSearchParams, func(param *mhttp.HTTPSearchParam) rhttp.HttpSearchParamEvent {
		return rhttp.HttpSearchParamEvent{
			Type:            "insert",
			IsDelta:         param.IsDelta,
			HttpSearchParam: converter.ToAPIHttpSearchParamFromMHttp(*param),
		}
	})

	// Add HTTP BodyForm events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindHTTPBodyForm, h.HttpBodyFormStream, rhttp.HttpBodyFormTopic{WorkspaceID: results.WorkspaceID}, results.HTTPBodyForms, func(form *mhttp.HTTPBodyForm) rhttp.HttpBodyFormEvent {
		return rhttp.HttpBodyFormEvent{
			Type:         "insert",
			IsDelta:      form.IsDelta,
			HttpBodyForm: converter.ToAPIHttpBodyFormDataFromMHttp(*form),
		}
	})

	// Add HTTP BodyUrlEncoded events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindHTTPBodyURL, h.HttpBodyUrlEncodedStream, rhttp.HttpBodyUrlEncodedTopic{WorkspaceID: results.WorkspaceID}, results.HTTPBodyUrlEncoded, func(encoded *mhttp.HTTPBodyUrlencoded) rhttp.HttpBodyUrlEncodedEvent {
		return rhttp.HttpBodyUrlEncodedEvent{
			Type:               "insert",
			IsDelta:            encoded.IsDelta,
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncodedFromMHttp(*encoded),
		}
	})

	// Add HTTP BodyRaw events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindHTTPBodyRaw, h.HttpBodyRawStream, rhttp.HttpBodyRawTopic{WorkspaceID: results.WorkspaceID}, results.HTTPBodyRaws, func(raw *mhttp.HTTPBodyRaw) rhttp.HttpBodyRawEvent {
		return rhttp.HttpBodyRawEvent{
			Type:        "insert",
			IsDelta:     raw.IsDelta,
			HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(*raw),
		}
	})

	// Add HTTP Assert events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindHTTPAssert, h.HttpAssertStream, rhttp.HttpAssertTopic{WorkspaceID: results.WorkspaceID}, results.HTTPAsserts, func(assert *mhttp.HTTPAssert) rhttp.HttpAssertEvent {
		return rhttp.HttpAssertEvent{
			Type:       "insert",
			IsDelta:    assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(*assert),
		}
	})

	// Add Environment Variable events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindEnvVariable, h.EnvVarStream, renv.EnvironmentVariableTopic{WorkspaceID: results.WorkspaceID}, results.CreatedVars, func(v menv.Variable) renv.EnvironmentVariableEvent {
		return renv.EnvironmentVariableEvent{
			Type:     "insert",
			Variable: converter.ToAPIEnvironmentVariable(v),
		}
	})

	// Add Environment Variable update events
	eventsync.AddSyncTransformSimple(batch, eventsync.KindEnvVariable, h.EnvVarStream, renv.EnvironmentVariableTopic{WorkspaceID: results.WorkspaceID}, results.UpdatedVars, func(v menv.Variable) renv.EnvironmentVariableEvent {
		return renv.EnvironmentVariableEvent{
			Type:     "update",
			Variable: converter.ToAPIEnvironmentVariable(v),
		}
	})

	// Publish all events in dependency-sorted order
	_ = batch.Publish(ctx)
}
