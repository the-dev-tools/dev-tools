import { Position, useReactFlow } from '@xyflow/react';
import { Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { ComponentProps, useCallback, useEffect, useMemo } from 'react';
import { Header, ListBoxSection } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiTerminal } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';

import { EdgeKind, EdgeListItem } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { NodeKind, NodeListItem, NodeNoOpKind } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { ForIcon, IfIcon, SendRequestIcon } from '@the-dev-tools/ui/icons';
import { ListBox, ListBoxItem, ListBoxItemProps } from '@the-dev-tools/ui/list-box';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { Edge, useMakeEdge } from '../edge';
import { Handle, HandleKind } from '../internal';
import { Node, NodeProps, useMakeNode } from '../node';

const CreateNodeHeader = (props: Omit<ComponentProps<'div'>, 'className'>) => (
  <Header {...props} className={tw`px-3 pt-2 text-xs leading-5 font-semibold tracking-tight text-slate-500`} />
);

interface CreateNodeItemProps extends Omit<ListBoxItemProps, 'children' | 'className' | 'textValue'> {
  description: string;
  Icon: IconType;
  title: string;
}

const CreateNodeItem = ({ description, Icon, title, ...props }: CreateNodeItemProps) => (
  <ListBoxItem {...props} className={tw`grid grid-cols-[auto_1fr] gap-x-2 gap-y-0 px-3 py-2`} textValue={title}>
    <div className={tw`row-span-2 rounded-md border border-slate-200 bg-white p-1.5`}>
      <Icon className={tw`size-5 text-slate-500`} />
    </div>
    <span className={tw`text-md leading-5 font-semibold tracking-tight`}>{title}</span>
    <span className={tw`text-xs leading-4 tracking-tight text-slate-500`}>{description}</span>
  </ListBoxItem>
);

export const CreateNode = ({ id, selected }: NodeProps) => {
  const { addEdges, addNodes, deleteElements, getEdges, getNode, getNodes, setNodes } = useReactFlow();

  const edge = useMemo(
    () =>
      pipe(
        getEdges().find((_) => _.target === id),
        Option.fromNullable,
      ),
    [getEdges, id],
  );

  useEffect(() => {
    if (!selected) void deleteElements({ nodes: [{ id }] });
  }, [deleteElements, id, selected]);

  const sourceId = useMemo(() => Option.map(edge, (_) => Ulid.fromCanonical(_.source).bytes), [edge]);

  const add = useCallback(
    async (nodes: NodeListItem[], edges: EdgeListItem[]) => {
      await deleteElements({ nodes: [{ id }] });

      setNodes((_) => _.map((_) => ({ ..._, selected: false })));

      pipe(
        nodes.map((_) => Node.fromDTO(_, { selected: true })),
        addNodes,
      );
      pipe(edges.map(Edge.fromDTO), addEdges);
    },
    [addEdges, addNodes, deleteElements, id, setNodes],
  );

  const makeNode = useMakeNode();
  const makeEdge = useMakeEdge();

  const getPosition = useCallback(() => getNode(id)!.position, [getNode, id]);

  const offset = 200;

  return (
    <>
      <ListBox
        aria-label='Create node type'
        className={twJoin(tw`w-80 divide-y divide-slate-200 pt-0 transition-colors`)}
        onAction={() => void {}}
      >
        <ListBoxSection>
          <CreateNodeHeader>Task</CreateNodeHeader>

          <CreateNodeItem
            description='Send request from your collection'
            Icon={SendRequestIcon}
            id='request'
            onAction={async () => {
              const node = await makeNode({
                kind: NodeKind.REQUEST,
                name: `request_${getNodes().length}`,
                position: getPosition(),
                request: {},
              });

              const edges = Option.isNone(sourceId)
                ? []
                : [await makeEdge({ sourceId: sourceId.value, targetId: node.nodeId })];
              await add([node], edges);
            }}
            title='Send Request'
          />

          {/* <CreateNodeItem
            id='data'
            Icon={DataSourceIcon}
            title='Data Source'
            description='Import data from .xlsx file'
          /> */}

          {/* <CreateNodeItem id='delay' Icon={DelayIcon} title='Delay' description='Wait specific time' /> */}

          <CreateNodeItem
            description='Custom Javascript block'
            Icon={FiTerminal}
            id='javascript'
            onAction={async () => {
              const node = await makeNode({
                js: {},
                kind: NodeKind.JS,
                name: `js_${getNodes().length}`,
                position: getPosition(),
              });
              const edges = Option.isNone(sourceId)
                ? []
                : [await makeEdge({ sourceId: sourceId.value, targetId: node.nodeId })];
              await add([node], edges);
            }}
            title='JavaScript'
          />
        </ListBoxSection>

        <ListBoxSection>
          <CreateNodeHeader>Logic</CreateNodeHeader>

          <CreateNodeItem
            description='Add true/false'
            Icon={IfIcon}
            id='condition'
            onAction={async () => {
              const position = getPosition();
              const { x, y } = position;
              const [node, nodeThen, nodeElse] = await Promise.all([
                makeNode({
                  condition: {},
                  kind: NodeKind.CONDITION,
                  name: `condition_${getNodes().length}`,
                  position,
                }),
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
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.THEN,
                  sourceId: node.nodeId,
                  targetId: nodeThen.nodeId,
                }),
                makeEdge({
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.ELSE,
                  sourceId: node.nodeId,
                  targetId: nodeElse.nodeId,
                }),
              ]);

              await add([node, nodeThen, nodeElse], edges);
            }}
            title='If'
          />
        </ListBoxSection>

        <ListBoxSection>
          <CreateNodeHeader>Looping</CreateNodeHeader>

          {/* <CreateNodeItem id='collect' Icon={CollectIcon} title='Collect' description='Collect all result' /> */}

          <CreateNodeItem
            description='Loop'
            Icon={ForIcon}
            id='for'
            onAction={async () => {
              const position = getPosition();
              const { x, y } = position;
              const [node, nodeLoop, nodeThen] = await Promise.all([
                makeNode({
                  for: {},
                  kind: NodeKind.FOR,
                  name: `for_${getNodes().length}`,
                  position,
                }),
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
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.LOOP,
                  sourceId: node.nodeId,
                  targetId: nodeLoop.nodeId,
                }),
                makeEdge({
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.THEN,
                  sourceId: node.nodeId,
                  targetId: nodeThen.nodeId,
                }),
              ]);

              await add([node, nodeLoop, nodeThen], edges);
            }}
            title='For Loop'
          />

          <CreateNodeItem
            description='Loop'
            Icon={ForIcon}
            id='foreach'
            onAction={async () => {
              const position = getPosition();
              const { x, y } = position;
              const [node, nodeLoop, nodeThen] = await Promise.all([
                makeNode({
                  forEach: {},
                  kind: NodeKind.FOR_EACH,
                  name: `foreach_${getNodes().length}`,
                  position,
                }),
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
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.LOOP,
                  sourceId: node.nodeId,
                  targetId: nodeLoop.nodeId,
                }),
                makeEdge({
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.THEN,
                  sourceId: node.nodeId,
                  targetId: nodeThen.nodeId,
                }),
              ]);

              await add([node, nodeLoop, nodeThen], edges);
            }}
            title='For Each Loop'
          />
        </ListBoxSection>
      </ListBox>

      <Handle isConnectableStart={false} position={Position.Top} type='target' />
    </>
  );
};
