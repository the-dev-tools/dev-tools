import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { HashSet } from 'effect';
import { Ulid } from 'id128';
import { ComponentProps, use, useEffect } from 'react';
import { Header, ListBoxSection } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiTerminal } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { EdgeKind, HandleKind, NodeKind, NodeNoOpKind } from '@the-dev-tools/spec/api/flow/v1/flow_pb';
import {
  EdgeCollectionSchema,
  NodeCollectionSchema,
  NodeConditionCollectionSchema,
  NodeForCollectionSchema,
  NodeForEachCollectionSchema,
  NodeHttpCollectionSchema,
  NodeJsCollectionSchema,
  NodeNoOpCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { ForIcon, IfIcon, SendRequestIcon } from '@the-dev-tools/ui/icons';
import { ListBox, ListBoxItem, ListBoxItemProps } from '@the-dev-tools/ui/list-box';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeStateContext } from '../node';

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

export const CreateNode = ({ id, selected }: XF.NodeProps) => {
  const { deleteElements, getNode, getNodes } = XF.useReactFlow();

  const { flowId } = use(FlowContext);
  const { setNodeSelection } = use(NodeStateContext);

  const createNodeId = Ulid.fromCanonical(id).bytes;

  const edgeCollection = useApiCollection(EdgeCollectionSchema);
  const nodeCollection = useApiCollection(NodeCollectionSchema);
  const noOpCollection = useApiCollection(NodeNoOpCollectionSchema);
  const jsCollection = useApiCollection(NodeJsCollectionSchema);
  const httpCollection = useApiCollection(NodeHttpCollectionSchema);
  const conditionCollection = useApiCollection(NodeConditionCollectionSchema);
  const forCollection = useApiCollection(NodeForCollectionSchema);
  const forEachCollection = useApiCollection(NodeForEachCollectionSchema);

  const edgeId = useLiveQuery(
    (_) =>
      _.from({ item: edgeCollection })
        .where((_) => eq(_.item.targetId, createNodeId))
        .select((_) => pick(_.item, 'edgeId'))
        .findOne(),
    [edgeCollection, createNodeId],
  ).data?.edgeId;

  useEffect(() => {
    if (!selected) void deleteElements({ nodes: [{ id }] });
  }, [deleteElements, id, selected]);

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
            description='Send an HTTP request'
            Icon={SendRequestIcon}
            id='http'
            onAction={() => {
              const nodeUlid = Ulid.generate();

              setNodeSelection(HashSet.add(nodeUlid.toCanonical()));

              httpCollection.utils.insert({ nodeId: nodeUlid.bytes });

              nodeCollection.utils.insert({
                flowId,
                kind: NodeKind.HTTP,
                name: `http_${getNodes().length}`,
                nodeId: nodeUlid.bytes,
                position: getNode(id)!.position,
              });

              if (edgeId) edgeCollection.utils.update({ edgeId, kind: EdgeKind.UNSPECIFIED, targetId: nodeUlid.bytes });

              nodeCollection.utils.delete({ nodeId: createNodeId });
            }}
            title='HTTP Request'
          />

          <CreateNodeItem
            description='Custom Javascript block'
            Icon={FiTerminal}
            id='javascript'
            onAction={() => {
              const nodeId = Ulid.generate().bytes;

              jsCollection.utils.insert({ nodeId });

              nodeCollection.utils.insert({
                flowId,
                kind: NodeKind.JS,
                name: `js_${getNodes().length}`,
                nodeId,
                position: getNode(id)!.position,
              });

              if (edgeId) edgeCollection.utils.update({ edgeId, kind: EdgeKind.UNSPECIFIED, targetId: nodeId });

              nodeCollection.utils.delete({ nodeId: createNodeId });
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
            onAction={() => {
              const nodeId = Ulid.generate().bytes;
              const nodeThenId = Ulid.generate().bytes;
              const nodeElseId = Ulid.generate().bytes;

              const position = getNode(id)!.position;
              const { x, y } = position;

              conditionCollection.utils.insert({ nodeId });

              noOpCollection.utils.insert([
                { kind: NodeNoOpKind.THEN, nodeId: nodeThenId },
                { kind: NodeNoOpKind.ELSE, nodeId: nodeElseId },
              ]);

              nodeCollection.utils.insert([
                { flowId, kind: NodeKind.CONDITION, name: `condition_${getNodes().length}`, nodeId, position },
                { flowId, kind: NodeKind.NO_OP, nodeId: nodeThenId, position: { x: x - offset, y: y + offset } },
                { flowId, kind: NodeKind.NO_OP, nodeId: nodeElseId, position: { x: x + offset, y: y + offset } },
              ]);

              edgeCollection.utils.insert([
                {
                  edgeId: Ulid.generate().bytes,
                  flowId,
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.THEN,
                  sourceId: nodeId,
                  targetId: nodeThenId,
                },
                {
                  edgeId: Ulid.generate().bytes,
                  flowId,
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.ELSE,
                  sourceId: nodeId,
                  targetId: nodeElseId,
                },
              ]);

              if (edgeId) edgeCollection.utils.update({ edgeId, kind: EdgeKind.UNSPECIFIED, targetId: nodeId });

              nodeCollection.utils.delete({ nodeId: createNodeId });
            }}
            title='If'
          />
        </ListBoxSection>

        <ListBoxSection>
          <CreateNodeHeader>Looping</CreateNodeHeader>

          <CreateNodeItem
            description='Loop'
            Icon={ForIcon}
            id='for'
            onAction={() => {
              const nodeId = Ulid.generate().bytes;
              const nodeThenId = Ulid.generate().bytes;
              const nodeLoopId = Ulid.generate().bytes;

              const position = getNode(id)!.position;
              const { x, y } = position;

              forCollection.utils.insert({ nodeId });

              noOpCollection.utils.insert([
                { kind: NodeNoOpKind.THEN, nodeId: nodeThenId },
                { kind: NodeNoOpKind.LOOP, nodeId: nodeLoopId },
              ]);

              nodeCollection.utils.insert([
                { flowId, kind: NodeKind.FOR, name: `for_${getNodes().length}`, nodeId, position },
                { flowId, kind: NodeKind.NO_OP, nodeId: nodeThenId, position: { x: x + offset, y: y + offset } },
                { flowId, kind: NodeKind.NO_OP, nodeId: nodeLoopId, position: { x: x - offset, y: y + offset } },
              ]);

              edgeCollection.utils.insert([
                {
                  edgeId: Ulid.generate().bytes,
                  flowId,
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.THEN,
                  sourceId: nodeId,
                  targetId: nodeThenId,
                },
                {
                  edgeId: Ulid.generate().bytes,
                  flowId,
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.LOOP,
                  sourceId: nodeId,
                  targetId: nodeLoopId,
                },
              ]);

              if (edgeId) edgeCollection.utils.update({ edgeId, kind: EdgeKind.UNSPECIFIED, targetId: nodeId });

              nodeCollection.utils.delete({ nodeId: createNodeId });
            }}
            title='For Loop'
          />

          <CreateNodeItem
            description='Loop'
            Icon={ForIcon}
            id='for_each'
            onAction={() => {
              const nodeId = Ulid.generate().bytes;
              const nodeThenId = Ulid.generate().bytes;
              const nodeLoopId = Ulid.generate().bytes;

              const position = getNode(id)!.position;
              const { x, y } = position;

              forEachCollection.utils.insert({ nodeId });

              noOpCollection.utils.insert([
                { kind: NodeNoOpKind.THEN, nodeId: nodeThenId },
                { kind: NodeNoOpKind.LOOP, nodeId: nodeLoopId },
              ]);

              nodeCollection.utils.insert([
                { flowId, kind: NodeKind.FOR_EACH, name: `for_each_${getNodes().length}`, nodeId, position },
                { flowId, kind: NodeKind.NO_OP, nodeId: nodeThenId, position: { x: x + offset, y: y + offset } },
                { flowId, kind: NodeKind.NO_OP, nodeId: nodeLoopId, position: { x: x - offset, y: y + offset } },
              ]);

              edgeCollection.utils.insert([
                {
                  edgeId: Ulid.generate().bytes,
                  flowId,
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.THEN,
                  sourceId: nodeId,
                  targetId: nodeThenId,
                },
                {
                  edgeId: Ulid.generate().bytes,
                  flowId,
                  kind: EdgeKind.NO_OP,
                  sourceHandle: HandleKind.LOOP,
                  sourceId: nodeId,
                  targetId: nodeLoopId,
                },
              ]);

              if (edgeId) edgeCollection.utils.update({ edgeId, kind: EdgeKind.UNSPECIFIED, targetId: nodeId });

              nodeCollection.utils.delete({ nodeId: createNodeId });
            }}
            title='For Each Loop'
          />
        </ListBoxSection>
      </ListBox>

      <Handle isConnectableStart={false} position={XF.Position.Top} type='target' />
    </>
  );
};
