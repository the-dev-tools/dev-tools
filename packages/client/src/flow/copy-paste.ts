import type { MessageInitShape } from '@bufbuild/protobuf';
import { enumFromJson } from '@bufbuild/protobuf';
import { useRouteContext } from '@tanstack/react-router';
import { useReactFlow } from '@xyflow/react';
import { Array, HashMap, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { use, useEffect, useRef } from 'react';
import type { NodeSchema } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { BodyKind } from '@the-dev-tools/spec/collection/item/body/v1/body_pb';
import { HeaderDeltaListItem, QueryDeltaListItem } from '@the-dev-tools/spec/collection/item/request/v1/request_pb';
import { SourceKind } from '@the-dev-tools/spec/delta/v1/delta_pb';
import { NodeKind, NodeNoOpKind } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  BodyRawGetEndpoint,
  BodyRawUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/body/v1/body.endpoints.js';
import {
  EndpointCreateEndpoint,
  EndpointGetEndpoint,
  EndpointUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/endpoint/v1/endpoint.endpoints.js';
import {
  ExampleCreateEndpoint,
  ExampleGetEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.js';
import {
  HeaderDeltaCreateEndpoint,
  HeaderDeltaListEndpoint,
  HeaderDeltaUpdateEndpoint,
  QueryDeltaCreateEndpoint,
  QueryDeltaListEndpoint,
  QueryDeltaUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.js';
import { NodeGetEndpoint } from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.js';
import { DataClient } from '~data-client';
import { Edge, useMakeEdge } from './edge';
import { FlowContext, HandleKind, HandleKindJson, HandleKindSchema } from './internal';
import { Node, useMakeNode } from './node';

export const makeCopyName = (name: string) => `${name}_copy`;

// Copy delta data (headers, query params, body) from source to target
async function copyDeltaData(
  dataClient: DataClient,
  originalDeltaExampleId: Uint8Array,
  deltaExampleId: Uint8Array,
  exampleId: Uint8Array,
) {
  // 1. Copy all header deltas
  try {
    const { items: sourceItems }: { items: HeaderDeltaListItem[] } = await dataClient.fetch(HeaderDeltaListEndpoint, {
      exampleId: originalDeltaExampleId,
      originId: exampleId,
    });

    const newItemMap = pipe(
      await dataClient.fetch(HeaderDeltaListEndpoint, {
        exampleId: deltaExampleId,
        originId: exampleId,
      }),
      (_) =>
        Array.filterMap(_.items, (_) => {
          if (!_.origin) return Option.none();
          const id = _.origin.headerId.toString();
          return Option.some([id, _] as const);
        }),
      HashMap.fromIterable,
    );

    for (const { $typeName: _, ...sourceItem } of sourceItems) {
      if (sourceItem.source === SourceKind.ORIGIN) continue;

      if (sourceItem.source === SourceKind.MIXED) {
        const newItem = pipe(
          Option.fromNullable(sourceItem.origin),
          Option.flatMap((_) => HashMap.get(newItemMap, _.headerId.toString())),
        );
        if (Option.isNone(newItem)) continue;
        await dataClient.fetch(HeaderDeltaUpdateEndpoint, { ...sourceItem, headerId: newItem.value.headerId });
      }

      if (sourceItem.source === SourceKind.DELTA) {
        await dataClient.fetch(HeaderDeltaCreateEndpoint, sourceItem);
      }
    }
  } catch (e) {
    console.error('Error copying headers:', e);
  }

  // 2. Copy all query parameter deltas
  try {
    const { items: sourceItems }: { items: QueryDeltaListItem[] } = await dataClient.fetch(QueryDeltaListEndpoint, {
      exampleId: originalDeltaExampleId,
      originId: exampleId,
    });

    const newItemMap = pipe(
      await dataClient.fetch(QueryDeltaListEndpoint, {
        exampleId: deltaExampleId,
        originId: exampleId,
      }),
      (_) =>
        Array.filterMap(_.items, (_) => {
          if (!_.origin) return Option.none();
          const id = _.origin.queryId.toString();
          return Option.some([id, _] as const);
        }),
      HashMap.fromIterable,
    );

    for (const { $typeName: _, ...sourceItem } of sourceItems) {
      if (sourceItem.source === SourceKind.ORIGIN) continue;

      if (sourceItem.source === SourceKind.MIXED) {
        const newItem = pipe(
          Option.fromNullable(sourceItem.origin),
          Option.flatMap((_) => HashMap.get(newItemMap, _.queryId.toString())),
        );
        if (Option.isNone(newItem)) continue;
        await dataClient.fetch(QueryDeltaUpdateEndpoint, { ...sourceItem, queryId: newItem.value.queryId });
      }

      if (sourceItem.source === SourceKind.DELTA) {
        await dataClient.fetch(QueryDeltaCreateEndpoint, sourceItem);
      }
    }
  } catch (e) {
    console.error('Error copying query params:', e);
  }

  // 3. Copy body data
  try {
    const exampleData = await dataClient.fetch(ExampleGetEndpoint, { exampleId });

    // Copy raw body if it exists
    if (exampleData.bodyKind === BodyKind.RAW) {
      const bodyRaw = await dataClient.fetch(BodyRawGetEndpoint, { exampleId: originalDeltaExampleId });
      if (bodyRaw.data.length > 0) {
        await dataClient.fetch(BodyRawUpdateEndpoint, {
          data: bodyRaw.data,
          exampleId: deltaExampleId,
        });
      }
    }
  } catch (e) {
    console.error('Error copying body:', e);
  }
}

interface CopyNodeProps {
  id: string;
  position: { x: number; y: number };
}

// Copy a single node with all its data and potentially its children
const useCopyNode = () => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const makeNode = useMakeNode();
  const makeEdge = useMakeEdge();

  return async ({ id, position }: CopyNodeProps) => {
    const nodeId = Ulid.fromCanonical(id).bytes;

    // Fetch full node data
    const fullNodeData = await dataClient.fetch(NodeGetEndpoint, { nodeId });
    const nodeKind = fullNodeData.kind;

    const offset = 200; // Same offset as in create.tsx
    const allNodes = [];
    const allEdges = [];

    // Handle control flow nodes that need child nodes
    if (nodeKind === NodeKind.CONDITION) {
      const [mainNode, nodeThen, nodeElse] = await Promise.all([
        makeNode({
          condition: fullNodeData.condition ?? {},
          kind: NodeKind.CONDITION,
          name: makeCopyName(fullNodeData.name || 'condition'),
          position,
        }),
        makeNode({
          kind: NodeKind.NO_OP,
          noOp: NodeNoOpKind.THEN,
          position: { x: position.x - offset, y: position.y + offset },
        }),
        makeNode({
          kind: NodeKind.NO_OP,
          noOp: NodeNoOpKind.ELSE,
          position: { x: position.x + offset, y: position.y + offset },
        }),
      ]);

      const edges = await Promise.all([
        makeEdge({
          sourceHandle: HandleKind.THEN,
          sourceId: mainNode.nodeId,
          targetId: nodeThen.nodeId,
        }),
        makeEdge({
          sourceHandle: HandleKind.ELSE,
          sourceId: mainNode.nodeId,
          targetId: nodeElse.nodeId,
        }),
      ]);

      allNodes.push(mainNode, nodeThen, nodeElse);
      allEdges.push(...edges);
    } else if (nodeKind === NodeKind.FOR || nodeKind === NodeKind.FOR_EACH) {
      const [mainNode, nodeLoop, nodeThen] = await Promise.all([
        makeNode({
          ...(nodeKind === NodeKind.FOR ? { for: fullNodeData.for ?? {} } : { forEach: fullNodeData.forEach ?? {} }),
          kind: nodeKind,
          name: makeCopyName(fullNodeData.name || (nodeKind === NodeKind.FOR ? 'for' : 'foreach')),
          position,
        }),
        makeNode({
          kind: NodeKind.NO_OP,
          noOp: NodeNoOpKind.LOOP,
          position: { x: position.x - offset, y: position.y + offset },
        }),
        makeNode({
          kind: NodeKind.NO_OP,
          noOp: NodeNoOpKind.THEN,
          position: { x: position.x + offset, y: position.y + offset },
        }),
      ]);

      const edges = await Promise.all([
        makeEdge({
          sourceHandle: HandleKind.LOOP,
          sourceId: mainNode.nodeId,
          targetId: nodeLoop.nodeId,
        }),
        makeEdge({
          sourceHandle: HandleKind.THEN,
          sourceId: mainNode.nodeId,
          targetId: nodeThen.nodeId,
        }),
      ]);

      allNodes.push(mainNode, nodeLoop, nodeThen);
      allEdges.push(...edges);
    } else {
      // For other nodes, handle based on type
      let newNode;

      if (nodeKind === NodeKind.REQUEST && fullNodeData.request) {
        // For REQUEST nodes, do exactly what users do when they select from collection
        const {
          collectionId,
          deltaEndpointId: originalDeltaEndpointId,
          deltaExampleId: originalDeltaExampleId,
          endpointId,
          exampleId,
        } = fullNodeData.request;

        // Create new delta endpoint (hidden)
        const {
          endpoint: { endpointId: deltaEndpointId },
        } = await dataClient.fetch(EndpointCreateEndpoint, {
          collectionId,
          hidden: true,
        });

        // Copy method and other endpoint properties from original delta endpoint if it exists
        if (originalDeltaEndpointId.length > 0) {
          const originalDeltaEndpoint = await dataClient.fetch(EndpointGetEndpoint, {
            endpointId: originalDeltaEndpointId,
          });

          // Copy relevant properties like method, url, etc.
          if (originalDeltaEndpoint.method || originalDeltaEndpoint.url) {
            await dataClient.fetch(EndpointUpdateEndpoint, {
              endpointId: deltaEndpointId,
              ...(originalDeltaEndpoint.method && { method: originalDeltaEndpoint.method }),
              ...(originalDeltaEndpoint.url && { url: originalDeltaEndpoint.url }),
            });
          }
        }

        // Create new delta example (hidden)
        const { exampleId: deltaExampleId } = await dataClient.fetch(ExampleCreateEndpoint, {
          endpointId: deltaEndpointId,
          hidden: true,
        });

        // Copy delta data if original has deltas
        if (originalDeltaExampleId.length > 0) {
          await copyDeltaData(dataClient, originalDeltaExampleId, deltaExampleId, exampleId);
        }

        // Create node with new deltas - just like when user selects from collection
        newNode = await makeNode({
          kind: NodeKind.REQUEST,
          name: makeCopyName(fullNodeData.name || 'request'),
          position,
          request: {
            collectionId,
            deltaEndpointId,
            deltaExampleId,
            endpointId,
            exampleId,
          },
        });
      } else {
        // For non-REQUEST nodes, duplicate normally
        const nodeData: MessageInitShape<typeof NodeSchema> = {
          kind: nodeKind,
          position,
          // Include the specific node data based on type
          ...(fullNodeData.condition && { condition: fullNodeData.condition }),
          ...(fullNodeData.for && { for: fullNodeData.for }),
          ...(fullNodeData.forEach && { forEach: fullNodeData.forEach }),
          ...(fullNodeData.js && { js: fullNodeData.js }),
        };

        // NO_OP nodes are special - they shouldn't be duplicated individually as they're part of control flow structures
        if (nodeKind === NodeKind.NO_OP) {
          // Skip NO_OP nodes
          return { edges: [], nodes: [] };
        }

        // Other nodes have names
        const baseName = fullNodeData.name || nodeKind.toString().toLowerCase();
        nodeData.name = makeCopyName(baseName);
        newNode = await makeNode(nodeData);
      }

      allNodes.push(newNode);
    }

    return { edges: allEdges, nodes: allNodes };
  };
};

export function useFlowCopyPaste() {
  const { dataClient } = useRouteContext({ from: '__root__' });
  const { isReadOnly = false } = use(FlowContext);
  const { addEdges, addNodes, getEdges, getNodes, setEdges, setNodes } = useReactFlow<Node, Edge>();

  const makeNode = useMakeNode();
  const makeEdge = useMakeEdge();
  const copyNode = useCopyNode();

  const copiedNodesRef = useRef({
    edges: Array.empty<Edge>(),
    nodes: Array.empty<Node>(),
  });

  useEffect(() => {
    const handleKeyDown = async (event: KeyboardEvent) => {
      if (isReadOnly) return;

      // Copy (Ctrl+C or Cmd+C)
      if ((event.ctrlKey || event.metaKey) && event.key === 'c') {
        const selectedNodes = getNodes().filter((node) => node.selected);
        if (selectedNodes.length > 0) {
          event.preventDefault();

          const selectedNodeIds = new Set(selectedNodes.map((n) => n.id));

          // Get edges that connect selected nodes (both source and target must be selected)
          const selectedEdges = getEdges().filter(
            (edge) => selectedNodeIds.has(edge.source) && selectedNodeIds.has(edge.target),
          );

          copiedNodesRef.current = {
            edges: selectedEdges,
            nodes: selectedNodes,
          };
        }
      }

      // Paste (Ctrl+V or Cmd+V)
      if ((event.ctrlKey || event.metaKey) && event.key === 'v') {
        if (copiedNodesRef.current.nodes.length > 0) {
          event.preventDefault();

          // Calculate bounding box of copied nodes to maintain relative positions
          const minX = Math.min(...copiedNodesRef.current.nodes.map((item) => item.position.x));
          const minY = Math.min(...copiedNodesRef.current.nodes.map((item) => item.position.y));

          // Create a map from old node IDs to new node IDs
          const nodeIdMap = new Map<string, string>();

          // Create new nodes with offset
          const allNewNodes: Node[] = [];
          const allNewEdges: Edge[] = [];

          for (const node of copiedNodesRef.current.nodes) {
            const position = {
              x: node.position.x - minX + minX + 50,
              y: node.position.y - minY + minY + 50,
            };

            const { edges: newEdges, nodes: newNodes } = await copyNode({ id: node.id, position });

            // Map the old node ID to the new main node ID
            if (newNodes.length > 0 && newNodes[0]) {
              const mainNode = Node.fromDTO(newNodes[0]);
              nodeIdMap.set(node.id, mainNode.id);

              // Add all nodes and edges
              allNewNodes.push(...newNodes.map((n) => Node.fromDTO(n)));
              allNewEdges.push(...newEdges.map((e) => Edge.fromDTO(e)));
            }
          }

          // Create new edges with mapped node IDs
          const newEdges = await Promise.all(
            copiedNodesRef.current.edges
              .filter((edge) => {
                // Only recreate edges where both nodes are in the pasted selection
                const sourceInMap = nodeIdMap.has(edge.source);
                const targetInMap = nodeIdMap.has(edge.target);
                return sourceInMap && targetInMap;
              })
              .map(async (edge) => {
                const newSourceId = nodeIdMap.get(edge.source)!;
                const newTargetId = nodeIdMap.get(edge.target)!;

                const newEdge = await makeEdge({
                  sourceHandle: edge.sourceHandle
                    ? enumFromJson(HandleKindSchema, edge.sourceHandle as HandleKindJson)
                    : HandleKind.UNSPECIFIED,
                  sourceId: Ulid.fromCanonical(newSourceId).bytes,
                  targetId: Ulid.fromCanonical(newTargetId).bytes,
                });
                return Edge.fromDTO(newEdge);
              }),
          );

          // 1) Deselect every existing node & edge
          setNodes((nodes) => nodes.map((n) => ({ ...n, selected: false })));
          setEdges((edges) => edges.map((e) => ({ ...e, selected: false })));

          // 2) Add _only_ our duplicates, all marked selected
          addNodes(allNewNodes.map((n) => ({ ...n, selected: true })));
          addEdges(
            [...allNewEdges, ...newEdges].map((e) => ({
              ...e,
              selected: true,
            })),
          );
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [
    addEdges,
    addNodes,
    dataClient,
    getEdges,
    getNodes,
    isReadOnly,
    makeEdge,
    makeNode,
    setNodes,
    setEdges,
    copyNode,
  ]);
}

export const useNodeDuplicate = (id: string) => {
  const { addEdges, addNodes, getNode, setNodes } = useReactFlow<Node, Edge>();

  const copyNode = useCopyNode();

  return async () => {
    const node = getNode(id);
    if (!node) return;

    const position = {
      x: node.position.x + 50,
      y: node.position.y + 50,
    };

    const { edges: newEdges, nodes: newNodes } = await copyNode({ id, position });

    // Deselect all nodes first
    setNodes((nodes) => nodes.map((n) => ({ ...n, selected: false })));
    // Add new nodes as selected
    addNodes(newNodes.map((n) => Node.fromDTO(n, { selected: true })));
    addEdges(newEdges.map((e) => Edge.fromDTO(e)));
  };
};
