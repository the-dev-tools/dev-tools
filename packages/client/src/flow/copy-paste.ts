import type { MessageInitShape } from '@bufbuild/protobuf';
import { enumFromJson } from '@bufbuild/protobuf';
import { Ulid } from 'id128';
import type { EdgeListItem, EdgeSchema } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import type { NodeListItem, NodeSchema } from '@the-dev-tools/spec/flow/node/v1/node_pb';
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
  HeaderDeltaListEndpoint,
  HeaderDeltaUpdateEndpoint,
  QueryDeltaListEndpoint,
  QueryDeltaUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/request/v1/request.endpoints.js';
import { NodeGetEndpoint } from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.js';
import { DataClient } from '~data-client';
import { HandleKind, HandleKindJson, HandleKindSchema } from './internal';

// Import actual types and conversion helpers
import { Edge } from './edge';
import { Node } from './node';

// Type for makeNode function
type MakeNodeFunction = (data: MessageInitShape<typeof NodeSchema>) => Promise<NodeListItem>;

// Type for makeEdge function  
type MakeEdgeFunction = (data: MessageInitShape<typeof EdgeSchema>) => Promise<EdgeListItem>;

interface CopyNodeOptions {
  dataClient: DataClient;
  makeEdge: MakeEdgeFunction;
  makeNode: MakeNodeFunction;
  nameTransform?: (name: string) => string;
  nodeId: Uint8Array;
  position: { x: number; y: number };
}

interface CopyNodeResult {
  edges: EdgeListItem[];
  nodes: NodeListItem[];
}

// Default name transform for duplicating nodes
export const duplicateName = (originalName: string) => {
  return `${originalName}_copy`;
};

// Copy delta data (headers, query params, body) from source to target
async function copyDeltaData(
  dataClient: DataClient,
  originalDeltaExampleId: Uint8Array,
  deltaExampleId: Uint8Array,
  exampleId: Uint8Array
) {
  // 1. Copy all header deltas
  try {
    // Get headers from the source delta that have been modified
    const { items: sourceDeltas }: { items: HeaderDeltaListItem[] } = await dataClient.fetch(HeaderDeltaListEndpoint, {
      exampleId: originalDeltaExampleId,
      originId: exampleId,
    });
    
    // Filter to only get overridden headers (not ORIGIN)
    const overriddenHeaders = sourceDeltas.filter((h) => h.source !== SourceKind.ORIGIN);
    console.log(`Found ${overriddenHeaders.length} overridden headers to copy`);
    
    if (overriddenHeaders.length > 0) {
      // Get all headers from the new delta example (they will all be ORIGIN)
      const { items: newDeltas }: { items: HeaderDeltaListItem[] } = await dataClient.fetch(HeaderDeltaListEndpoint, {
        exampleId: deltaExampleId,
        originId: exampleId,
      });
      
      // For each overridden header in source, update the matching one in target
      for (const src of overriddenHeaders) {
        const match = newDeltas.find((d) => d.key === src.key);
        if (!match) {
          console.warn(`No matching header found for key: ${src.key}`);
          continue;
        }
        
        console.log(`Updating header '${src.key}' with value: ${src.value}`);
        
        // Update the header with the overridden values
        await dataClient.fetch(HeaderDeltaUpdateEndpoint, {
          description: src.description,
          enabled: src.enabled,
          headerId: match.headerId,
          key: src.key,
          value: src.value,
        });
      }
    }
  } catch (e) {
    console.error('Error copying headers:', e);
  }
  
  // 2. Copy all query parameter deltas
  try {
    // Get query params from the source delta that have been modified
    const { items: sourceQueryDeltas }: { items: QueryDeltaListItem[] } = await dataClient.fetch(QueryDeltaListEndpoint, {
      exampleId: originalDeltaExampleId,
      originId: exampleId,
    });
    
    // Filter to only get overridden query params (not ORIGIN)
    const overriddenQueries = sourceQueryDeltas.filter((q) => q.source !== SourceKind.ORIGIN);
    console.log(`Found ${overriddenQueries.length} overridden query params to copy`);
    
    if (overriddenQueries.length > 0) {
      // Get all query params from the new delta example (they will all be ORIGIN)
      const { items: newQueryDeltas }: { items: QueryDeltaListItem[] } = await dataClient.fetch(QueryDeltaListEndpoint, {
        exampleId: deltaExampleId,
        originId: exampleId,
      });
      
      // For each overridden query param in source, update the matching one in target
      for (const src of overriddenQueries) {
        const match = newQueryDeltas.find((d) => d.key === src.key);
        if (!match) {
          console.warn(`No matching query param found for key: ${src.key}`);
          continue;
        }
        
        console.log(`Updating query param '${src.key}' with value: ${src.value}`);
        
        // Update the query param with the overridden values
        await dataClient.fetch(QueryDeltaUpdateEndpoint, {
          description: src.description,
          enabled: src.enabled,
          key: src.key,
          queryId: match.queryId,
          value: src.value,
        });
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

// Copy a single node with all its data and potentially its children
export async function copyNode(options: CopyNodeOptions): Promise<CopyNodeResult> {
  const { dataClient, makeEdge, makeNode, nameTransform = duplicateName, nodeId, position } = options;
  
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
        name: nameTransform(fullNodeData.name || 'condition'),
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
        ...(nodeKind === NodeKind.FOR
          ? { for: fullNodeData.for ?? {} }
          : { forEach: fullNodeData.forEach ?? {} }),
        kind: nodeKind,
        name: nameTransform(fullNodeData.name || (nodeKind === NodeKind.FOR ? 'for' : 'foreach')),
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
      const { collectionId, deltaEndpointId: originalDeltaEndpointId, deltaExampleId: originalDeltaExampleId, endpointId, exampleId } = fullNodeData.request;
      
      // Create new delta endpoint (hidden)
      const { endpoint: { endpointId: deltaEndpointId } } = await dataClient.fetch(
        EndpointCreateEndpoint,
        {
          collectionId,
          hidden: true,
        }
      );
      
      // Copy method and other endpoint properties from original delta endpoint if it exists
      if (originalDeltaEndpointId.length > 0) {
        const originalDeltaEndpoint = await dataClient.fetch(EndpointGetEndpoint, { 
          endpointId: originalDeltaEndpointId 
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
      const { exampleId: deltaExampleId } = await dataClient.fetch(
        ExampleCreateEndpoint,
        {
          endpointId: deltaEndpointId,
          hidden: true,
        }
      );
      
      // Copy delta data if original has deltas
      if (originalDeltaExampleId.length > 0) {
        await copyDeltaData(dataClient, originalDeltaExampleId, deltaExampleId, exampleId);
      }
      
      // Create node with new deltas - just like when user selects from collection
      newNode = await makeNode({
        kind: NodeKind.REQUEST,
        name: nameTransform(fullNodeData.name || 'request'),
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
      nodeData.name = nameTransform(baseName);
      newNode = await makeNode(nodeData);
    }

    allNodes.push(newNode);
  }
  
  return { edges: allEdges, nodes: allNodes };
}

// Helper function to handle copy/paste keyboard events
export function setupCopyPasteHandlers(
  getNodes: () => Node[],
  getEdges: () => Edge[],
  dataClient: DataClient,
  makeNode: MakeNodeFunction,
  makeEdge: MakeEdgeFunction,
  addNodes: (nodes: Node[]) => void,
  addEdges: (edges: Edge[]) => void,
  setNodes: (updater: (nodes: Node[]) => Node[]) => void,
  setEdges: (updater: (edges: Edge[]) => Edge[]) => void,
  isReadOnly: boolean
) {
  const copiedNodesRef = { current: { edges: [] as Edge[], nodes: [] as { fullData: unknown; node: Node }[] } };

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

        // Fetch full data for each selected node
        const nodesWithData = await Promise.all(
          selectedNodes.map(async (node) => {
            const nodeId = Ulid.fromCanonical(node.id).bytes;
            const fullData = await dataClient.fetch(NodeGetEndpoint, { nodeId });
            return { fullData, node };
          }),
        );

        copiedNodesRef.current = {
          edges: selectedEdges,
          nodes: nodesWithData,
        };
      }
    }

    // Paste (Ctrl+V or Cmd+V)
    if ((event.ctrlKey || event.metaKey) && event.key === 'v') {
      if (copiedNodesRef.current.nodes.length > 0) {
        event.preventDefault();

        // Calculate bounding box of copied nodes to maintain relative positions
        const minX = Math.min(...copiedNodesRef.current.nodes.map((item) => item.node.position.x));
        const minY = Math.min(...copiedNodesRef.current.nodes.map((item) => item.node.position.y));

        // Create a map from old node IDs to new node IDs
        const nodeIdMap = new Map<string, string>();

        // Create new nodes with offset
        const allNewNodes: Node[] = [];
        const allNewEdges: Edge[] = [];
        
        for (const { node } of copiedNodesRef.current.nodes) {
          const position = {
            x: node.position.x - minX + minX + 50,
            y: node.position.y - minY + minY + 50,
          };
          
          const nodeId = Ulid.fromCanonical(node.id).bytes;
          
          // Use the copyNode function
          const { edges: newEdges, nodes: newNodes } = await copyNode({
            dataClient,
            makeEdge,
            makeNode,
            nameTransform: duplicateName,
            nodeId,
            position,
          });
          
          // Map the old node ID to the new main node ID
          if (newNodes.length > 0 && newNodes[0]) {
            const mainNode = Node.fromDTO(newNodes[0]);
            nodeIdMap.set(node.id, mainNode.id);
            
            // Add all nodes and edges
            allNewNodes.push(...newNodes.map(n => Node.fromDTO(n)));
            allNewEdges.push(...newEdges.map(e => Edge.fromDTO(e)));
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
          }))
        );
      }
    }
  };

  return handleKeyDown;
}

// Helper function to duplicate a single node from context menu
// We use a callback pattern to avoid type issues between modules
export async function duplicateNodeFromMenu(
  nodeId: string,
  nodePosition: { x: number; y: number },
  dataClient: DataClient,
  makeNode: MakeNodeFunction,
  makeEdge: MakeEdgeFunction,
  callback: (nodes: NodeListItem[], edges: EdgeListItem[]) => void
) {
  const position = {
    x: nodePosition.x + 50,
    y: nodePosition.y + 50,
  };

  const nodeIdBytes = Ulid.fromCanonical(nodeId).bytes;

  // Use the copyNode function
  const { edges: newEdges, nodes: newNodes } = await copyNode({
    dataClient,
    makeEdge,
    makeNode,
    nodeId: nodeIdBytes,
    position,
  });

  // Call the callback with the new nodes and edges
  callback(newNodes, newEdges);
}