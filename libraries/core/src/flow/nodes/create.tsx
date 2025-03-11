import { Position, useReactFlow } from '@xyflow/react';
import { Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { ComponentProps, useCallback, useMemo } from 'react';
import { Header, ListBoxSection } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiTerminal } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';

import { NodeKind, NodeNoOpKind } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { CollectIcon, DataSourceIcon, DelayIcon, ForIcon, IfIcon, SendRequestIcon } from '@the-dev-tools/ui/icons';
import { ListBox, ListBoxItem, ListBoxItemProps } from '@the-dev-tools/ui/list-box';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { Edge, EdgeDTO, useMakeEdge } from '../edge';
import { Handle, HandleKind } from '../internal';
import { Node, NodeDTO, NodeProps, useMakeNode } from '../node';

const CreateNodeHeader = (props: Omit<ComponentProps<'div'>, 'className'>) => (
  <Header {...props} className={tw`px-3 pt-2 text-xs font-semibold leading-5 tracking-tight text-slate-500`} />
);

interface CreateNodeItemProps extends Omit<ListBoxItemProps, 'children' | 'className' | 'textValue'> {
  Icon: IconType;
  title: string;
  description: string;
}

const CreateNodeItem = ({ Icon, title, description, ...props }: CreateNodeItemProps) => (
  <ListBoxItem {...props} className={tw`grid grid-cols-[auto_1fr] gap-x-2 gap-y-0 px-3 py-2`} textValue={title}>
    <div className={tw`row-span-2 rounded-md border border-slate-200 bg-white p-1.5`}>
      <Icon className={tw`size-5 text-slate-500`} />
    </div>
    <span className={tw`text-md font-semibold leading-5 tracking-tight`}>{title}</span>
    <span className={tw`text-xs leading-4 tracking-tight text-slate-500`}>{description}</span>
  </ListBoxItem>
);

export const CreateNode = ({ id, selected }: NodeProps) => {
  const { getNode, getEdges, addNodes, addEdges, deleteElements } = useReactFlow();

  const edge = useMemo(
    () =>
      pipe(
        getEdges().find((_) => _.target === id),
        Option.fromNullable,
      ),
    [getEdges, id],
  );

  const sourceId = useMemo(() => Option.map(edge, (_) => Ulid.fromCanonical(_.source).bytes), [edge]);

  const add = useCallback(
    async (nodes: NodeDTO[], edges: EdgeDTO[]) => {
      await deleteElements({ nodes: [{ id }], edges: Option.toArray(edge) });

      pipe(nodes.map(Node.fromDTO), addNodes);
      pipe(edges.map(Edge.fromDTO), addEdges);
    },
    [addEdges, addNodes, deleteElements, edge, id],
  );

  const makeNode = useMakeNode();
  const makeEdge = useMakeEdge();

  const getPosition = useCallback(() => getNode(id)!.position, [getNode, id]);

  const offset = 200;

  return (
    <>
      <ListBox
        aria-label='Create node type'
        onAction={() => void {}}
        className={twJoin(tw`w-80 divide-y divide-slate-200 pt-0 transition-colors`, selected && tw`bg-slate-100`)}
      >
        <ListBoxSection>
          <CreateNodeHeader>Task</CreateNodeHeader>

          <CreateNodeItem
            id='request'
            Icon={SendRequestIcon}
            title='Send Request'
            description='Send request from your collection'
            onAction={async () => {
              const node = await makeNode({ kind: NodeKind.REQUEST, request: {}, position: getPosition() });
              const edges = Option.isNone(sourceId)
                ? []
                : [await makeEdge({ sourceId: sourceId.value, targetId: node.nodeId })];
              await add([node], edges);
            }}
          />

          <CreateNodeItem
            id='data'
            Icon={DataSourceIcon}
            title='Data Source'
            description='Import data from .xlsx file'
          />

          <CreateNodeItem id='delay' Icon={DelayIcon} title='Delay' description='Wait specific time' />

          <CreateNodeItem
            id='javascript'
            Icon={FiTerminal}
            title='JavaScript'
            description='Custom Javascript block'
            onAction={async () => {
              const node = await makeNode({ kind: NodeKind.JS, js: {}, position: getPosition() });
              const edges = Option.isNone(sourceId)
                ? []
                : [await makeEdge({ sourceId: sourceId.value, targetId: node.nodeId })];
              await add([node], edges);
            }}
          />
        </ListBoxSection>

        <ListBoxSection>
          <CreateNodeHeader>Logic</CreateNodeHeader>

          <CreateNodeItem
            id='condition'
            Icon={IfIcon}
            title='If'
            description='Add true/false'
            onAction={async () => {
              const position = getPosition();
              const { x, y } = position;
              const [node, nodeThen, nodeElse] = await Promise.all([
                makeNode({ kind: NodeKind.CONDITION, condition: {}, position }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.THEN,
                  position: { x: x - offset, y: y + offset },
                }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.ELSE,
                  position: { x: x + offset, y: y + offset },
                }),
              ]);

              const edges = await Promise.all([
                ...(Option.isNone(sourceId)
                  ? []
                  : [await makeEdge({ sourceId: sourceId.value, targetId: node.nodeId })]),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.THEN,
                  targetId: nodeThen.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.ELSE,
                  targetId: nodeElse.nodeId,
                }),
              ]);

              await add([node, nodeThen, nodeElse], edges);
            }}
          />
        </ListBoxSection>

        <ListBoxSection>
          <CreateNodeHeader>Looping</CreateNodeHeader>

          <CreateNodeItem id='collect' Icon={CollectIcon} title='Collect' description='Collect all result' />

          <CreateNodeItem
            id='for'
            Icon={ForIcon}
            title='For Loop'
            description='Loop'
            onAction={async () => {
              const position = getPosition();
              const { x, y } = position;
              const [node, nodeLoop, nodeThen] = await Promise.all([
                makeNode({ kind: NodeKind.FOR, for: {}, position }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.LOOP,
                  position: { x: x - offset, y: y + offset },
                }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.THEN,
                  position: { x: x + offset, y: y + offset },
                }),
              ]);

              const edges = await Promise.all([
                ...(Option.isNone(sourceId)
                  ? []
                  : [await makeEdge({ sourceId: sourceId.value, targetId: node.nodeId })]),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.LOOP,
                  targetId: nodeLoop.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.THEN,
                  targetId: nodeThen.nodeId,
                }),
              ]);

              await add([node, nodeLoop, nodeThen], edges);
            }}
          />

          <CreateNodeItem
            id='foreach'
            Icon={ForIcon}
            title='For Each Loop'
            description='Loop'
            onAction={async () => {
              const position = getPosition();
              const { x, y } = position;
              const [node, nodeLoop, nodeThen] = await Promise.all([
                makeNode({ kind: NodeKind.FOR_EACH, forEach: {}, position }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.LOOP,
                  position: { x: x - offset, y: y + offset },
                }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.THEN,
                  position: { x: x + offset, y: y + offset },
                }),
              ]);

              const edges = await Promise.all([
                ...(Option.isNone(sourceId)
                  ? []
                  : [await makeEdge({ sourceId: sourceId.value, targetId: node.nodeId })]),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.LOOP,
                  targetId: nodeLoop.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.THEN,
                  targetId: nodeThen.nodeId,
                }),
              ]);

              await add([node, nodeLoop, nodeThen], edges);
            }}
          />
        </ListBoxSection>
      </ListBox>

      <Handle type='target' position={Position.Top} isConnectableStart={false} />
    </>
  );
};
