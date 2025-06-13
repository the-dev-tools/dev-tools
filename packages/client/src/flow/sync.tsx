import { create, equals } from '@bufbuild/protobuf';
import { useRouteContext } from '@tanstack/react-router';
import { useEdgesState, useNodesState } from '@xyflow/react';
import { Array, HashMap, Option, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { use } from 'react';
import { useDebouncedCallback } from 'use-debounce';
import { EdgeListItemSchema } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { NodeListItemSchema } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  EdgeCreateEndpoint,
  EdgeDeleteEndpoint,
  EdgeListEndpoint,
  EdgeUpdateEndpoint,
} from '@the-dev-tools/spec/meta/flow/edge/v1/edge.endpoints.js';
import {
  NodeCreateEndpoint,
  NodeDeleteEndpoint,
  NodeListEndpoint,
  NodeUpdateEndpoint,
} from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.js';
import { useQuery } from '~data-client';
import { Edge } from './edge';
import { FlowContext } from './internal';
import { Node } from './node';

export const useFlowStateSynced = () => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { flowId, isReadOnly = false } = use(FlowContext);

  const { items: edgesServer } = useQuery(EdgeListEndpoint, { flowId });
  const { items: nodesServer } = useQuery(NodeListEndpoint, { flowId });

  const [edgesClient, _setEdgesClient, onEdgesChangeClient] = useEdgesState(edgesServer.map((_) => Edge.fromDTO(_)));
  const [nodesClient, _setNodesClient, onNodesChangeClient] = useNodesState(nodesServer.map((_) => Node.fromDTO(_)));

  const sync = useDebouncedCallback(async () => {
    const edgeServerMap = pipe(
      edgesServer.map((_) => {
        const id = Ulid.construct(_.edgeId).toCanonical();
        const value = create(EdgeListItemSchema, Struct.omit(_, '$typeName'));
        return [id, value] as const;
      }),
      HashMap.fromIterable,
    );

    const edgeClientMap = pipe(
      edgesClient.map((_) => {
        const value = create(EdgeListItemSchema, Edge.toDTO(_));
        return [_.id, value] as const;
      }),
      HashMap.fromIterable,
    );

    const edgeChanges: Record<string, [string, ReturnType<typeof Edge.toDTO>][]> = pipe(
      HashMap.union(edgeServerMap, edgeClientMap),
      HashMap.entries,
      Array.groupBy(([id]) => {
        const edgeServer = HashMap.get(edgeServerMap, id);
        const edgeClient = HashMap.get(edgeClientMap, id);

        if (Option.isNone(edgeServer)) return 'create';
        if (Option.isNone(edgeClient)) return 'delete';

        return equals(EdgeListItemSchema, edgeServer.value, edgeClient.value) ? 'ignore' : 'update';
      }),
    );

    const nodeServerMap = pipe(
      nodesServer.map((_) => {
        const id = Ulid.construct(_.nodeId).toCanonical();
        const value = pipe(Struct.pick(_, 'kind', 'nodeId', 'position'), (_) => create(NodeListItemSchema, _));
        return [id, value] as const;
      }),
      HashMap.fromIterable,
    );

    const nodeClientMap = pipe(
      nodesClient.map((_) => {
        const value = create(NodeListItemSchema, Node.toDTO(_));
        return [_.id, value] as const;
      }),
      HashMap.fromIterable,
    );

    const nodeChanges: Record<string, [string, ReturnType<typeof Node.toDTO>][]> = pipe(
      HashMap.union(nodeServerMap, nodeClientMap),
      HashMap.entries,
      Array.groupBy(([id]) => {
        const nodeServer = HashMap.get(nodeServerMap, id);
        const nodeClient = HashMap.get(nodeClientMap, id);

        if (Option.isNone(nodeServer)) return 'create';
        if (Option.isNone(nodeClient)) return 'delete';

        return equals(NodeListItemSchema, nodeServer.value, nodeClient.value) ? 'ignore' : 'update';
      }),
    );

    // Change processing order matters to avoid race conditions,
    // hence different kinds of changes are awaited separately

    await pipe(
      nodeChanges['create'] ?? [],
      Array.filterMap(([_id, node]) =>
        pipe(
          Option.liftPredicate(node, (_) => !_.nodeId.length),
          Option.map((node) => dataClient.fetch(NodeCreateEndpoint, node)),
        ),
      ),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      edgeChanges['create'] ?? [],
      Array.filterMap(([_id, edge]) =>
        pipe(
          Option.liftPredicate(edge, (_) => !_.edgeId.length),
          Option.map((edge) => dataClient.fetch(EdgeCreateEndpoint, edge)),
        ),
      ),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      nodeChanges['update'] ?? [],
      Array.map(([_id, node]) => dataClient.fetch(NodeUpdateEndpoint, node)),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      edgeChanges['update'] ?? [],
      Array.map(([_id, edge]) => dataClient.fetch(EdgeUpdateEndpoint, edge)),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      edgeChanges['delete'] ?? [],
      Array.map(([_id, edge]) => dataClient.fetch(EdgeDeleteEndpoint, edge)),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      nodeChanges['delete'] ?? [],
      Array.map(([_id, node]) => dataClient.fetch(NodeDeleteEndpoint, node)),
      (_) => Promise.allSettled(_),
    );
  }, 500);

  const onEdgesChangeSync: typeof onEdgesChangeClient = (changes) => {
    onEdgesChangeClient(changes);
    if (!isReadOnly) void sync();
  };

  const onNodesChangeSync: typeof onNodesChangeClient = (changes) => {
    onNodesChangeClient(changes);
    if (!isReadOnly) void sync();
  };

  return {
    edges: edgesClient,
    onEdgesChange: onEdgesChangeSync,

    nodes: nodesClient,
    onNodesChange: onNodesChangeSync,
  };
};
