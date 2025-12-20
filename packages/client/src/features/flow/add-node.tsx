import { MessageInitShape } from '@bufbuild/protobuf';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { ReactNode, use } from 'react';
import * as RAC from 'react-aria-components';
import { FiArrowLeft, FiBriefcase, FiChevronRight, FiTerminal, FiX } from 'react-icons/fi';
import { FileKind } from '@the-dev-tools/spec/buf/api/file_system/v1/file_system_pb';
import { HandleKind, NodeHttpInsertSchema, NodeKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { HttpMethod } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import { FileCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import {
  EdgeCollectionSchema,
  NodeCollectionSchema,
  NodeConditionCollectionSchema,
  NodeForCollectionSchema,
  NodeForEachCollectionSchema,
  NodeHttpCollectionSchema,
  NodeJsCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { HttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { FlowsIcon, ForIcon, IfIcon, SendRequestIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { FileTree } from '~/file-system';
import { workspaceRouteApi } from '~/routes';
import { getNextOrder } from '~/utils/order';
import { FlowContext } from './context';

interface AddNodeSidebarProps {
  handleKind?: HandleKind | undefined;
  position?: undefined | XF.XYPosition;
  previous?: ReactNode;
  sourceId?: Uint8Array | undefined;
  targetId?: Uint8Array | undefined;
}

export const AddNodeSidebar = (props: AddNodeSidebarProps) => {
  const { setSidebar } = use(FlowContext);

  return (
    <>
      <SidebarHeader title='What happens next?' />

      <RAC.ListBox aria-label='Node categories' className={tw`mt-3`}>
        <Item
          description='Branch, merge or loop the flow, etc.'
          icon={<FlowsIcon />}
          onAction={() => void setSidebar?.((_) => <AddFlowNodeSidebar {...props} previous={_} />)}
          title='Flow'
        />

        <Item
          description='Run code, make HTTP request, set webhooks, etc.'
          icon={<FiBriefcase />}
          onAction={() => void setSidebar?.((_) => <AddCoreNodeSidebar {...props} previous={_} />)}
          title='Core'
        />
      </RAC.ListBox>
    </>
  );
};

interface SidebarHeaderProps {
  previous?: ReactNode;
  title: string;
}

const SidebarHeader = ({ previous, title }: SidebarHeaderProps) => {
  const { setSidebar } = use(FlowContext);

  return (
    <div className={tw`flex items-center gap-2 border-b border-slate-200 px-3 py-2`}>
      {previous && (
        <Button className={tw`p-1`} onPress={() => void setSidebar?.(previous)} variant='ghost'>
          <FiArrowLeft className={tw`size-5 text-slate-500`} />
        </Button>
      )}

      <div className={tw`flex-1 leading-6 font-semibold tracking-tight text-slate-800`}>{title}</div>

      <Button className={tw`p-1`} onPress={() => void setSidebar?.(null)} variant='ghost'>
        <FiX className={tw`size-5 text-slate-500`} />
      </Button>
    </div>
  );
};

interface ItemProps {
  description?: string;
  icon: ReactNode;
  onAction: () => void;
  title: string;
}

const Item = ({ description, icon, onAction, title }: ItemProps) => (
  <ListBoxItem className={tw`gap-2 px-3 py-2`} onAction={onAction} textValue={title}>
    <div className={tw`rounded-md border border-slate-200 bg-white p-1.5 text-xl text-slate-500`}>{icon}</div>

    <div className={tw`flex-1`}>
      <div className={tw`text-md leading-5 font-semibold tracking-tight text-slate-800`}>{title}</div>
      {description && <div className={tw`text-xs leading-4 tracking-tight text-slate-500`}>{description}</div>}
    </div>

    <FiChevronRight className={tw`m-1 size-4 text-slate-500`} />
  </ListBoxItem>
);

interface InsertNodeProps {
  handleKind?: HandleKind | undefined;
  kind: NodeKind;
  name: string;
  nodeId: Uint8Array;
  position?: undefined | XF.XYPosition;
  sourceId?: Uint8Array | undefined;
  targetId?: Uint8Array | undefined;
}

const useInsertNode = () => {
  const { flowId, setSidebar } = use(FlowContext);
  const { getNodes, screenToFlowPosition } = XF.useReactFlow();
  const storeApi = XF.useStoreApi();

  const nodeCollection = useApiCollection(NodeCollectionSchema);
  const edgeCollection = useApiCollection(EdgeCollectionSchema);

  return ({ handleKind, kind, name, nodeId, position, sourceId, targetId }: InsertNodeProps) => {
    const { domNode } = storeApi.getState();
    const box = domNode?.getBoundingClientRect();
    const defaultPosition: XF.XYPosition = box
      ? screenToFlowPosition({ x: box.x + box.width * 0.5, y: box.y + box.height * 0.4 })
      : { x: 0, y: 0 };

    nodeCollection.utils.insert({
      flowId,
      kind,
      name: `${name}_${getNodes().length}`,
      nodeId,
      position: position ?? defaultPosition,
    });

    if (sourceId)
      edgeCollection.utils.insert({
        edgeId: Ulid.generate().bytes,
        flowId,
        sourceId,
        targetId: nodeId,
        ...(handleKind && { sourceHandle: handleKind }),
      });

    if (targetId)
      edgeCollection.utils.insert({
        edgeId: Ulid.generate().bytes,
        flowId,
        sourceId: nodeId,
        targetId,
        ...(handleKind && { sourceHandle: handleKind }),
      });

    setSidebar?.(null);
  };
};

const AddFlowNodeSidebar = ({ handleKind, position, previous, sourceId, targetId }: AddNodeSidebarProps) => {
  const insertNode = useInsertNode();

  const conditionCollection = useApiCollection(NodeConditionCollectionSchema);
  const forCollection = useApiCollection(NodeForCollectionSchema);
  const forEachCollection = useApiCollection(NodeForEachCollectionSchema);

  return (
    <>
      <SidebarHeader previous={previous} title='Flow' />

      <RAC.ListBox aria-label='Node categories' className={tw`mt-3`}>
        <Item
          description='Route items to different branches'
          icon={<IfIcon />}
          onAction={() => {
            const nodeId = Ulid.generate().bytes;
            conditionCollection.utils.insert({ nodeId });
            insertNode({ handleKind, kind: NodeKind.CONDITION, name: 'if', nodeId, position, sourceId, targetId });
          }}
          title='If'
        />

        <Item
          description='Iterate for a set amount of times'
          icon={<ForIcon />}
          onAction={() => {
            const nodeId = Ulid.generate().bytes;
            forCollection.utils.insert({ nodeId });
            insertNode({ handleKind, kind: NodeKind.FOR, name: 'for', nodeId, position, sourceId, targetId });
          }}
          title='For loop'
        />

        <Item
          description='Iterate over data'
          icon={<ForIcon />}
          onAction={() => {
            const nodeId = Ulid.generate().bytes;
            forEachCollection.utils.insert({ nodeId });
            insertNode({ handleKind, kind: NodeKind.FOR_EACH, name: 'for_each', nodeId, position, sourceId, targetId });
          }}
          title='For each loop'
        />
      </RAC.ListBox>
    </>
  );
};

const AddCoreNodeSidebar = (props: AddNodeSidebarProps) => {
  const { handleKind, position, previous, sourceId, targetId } = props;
  const { setSidebar } = use(FlowContext);

  const insertNode = useInsertNode();

  const jsCollection = useApiCollection(NodeJsCollectionSchema);

  return (
    <>
      <SidebarHeader previous={previous} title='Flow' />

      <RAC.ListBox aria-label='Node categories' className={tw`mt-3`}>
        <Item
          description='Run custom JavaScript code'
          icon={<FiTerminal />}
          onAction={() => {
            const nodeId = Ulid.generate().bytes;
            jsCollection.utils.insert({ nodeId });
            insertNode({ handleKind, kind: NodeKind.JS, name: 'js', nodeId, position, sourceId, targetId });
          }}
          title='JavaScript'
        />

        <Item
          description='Makes an HTTP request and returns the respons data'
          icon={<SendRequestIcon />}
          onAction={() => void setSidebar?.((_) => <AddHttpRequestNodeSidebar {...props} previous={_} />)}
          title='HTTP Request'
        />
      </RAC.ListBox>
    </>
  );
};

const AddHttpRequestNodeSidebar = ({ handleKind, position, previous, sourceId, targetId }: AddNodeSidebarProps) => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const insertNode = useInsertNode();

  const fileCollection = useApiCollection(FileCollectionSchema);
  const httpCollection = useApiCollection(HttpCollectionSchema);
  const nodeHttpCollection = useApiCollection(NodeHttpCollectionSchema);

  return (
    <>
      <SidebarHeader previous={previous} title='HTTP request' />

      <div className={tw`mx-4 my-3`}>
        <Button
          className={tw`w-full`}
          onPress={async () => {
            const httpId = Ulid.generate().bytes;

            httpCollection.utils.insert({
              httpId: httpId,
              method: HttpMethod.GET,
              name: 'New HTTP request',
            });

            fileCollection.utils.insert({
              fileId: httpId,
              kind: FileKind.HTTP,
              order: await getNextOrder(fileCollection),
              workspaceId,
            });

            const nodeId = Ulid.generate().bytes;
            nodeHttpCollection.utils.insert({ httpId, nodeId });
            insertNode({ handleKind, kind: NodeKind.HTTP, name: 'http', nodeId, position, sourceId, targetId });
          }}
        >
          New HTTP request
        </Button>
      </div>

      <FileTree
        onAction={(key) => {
          const nodeId = Ulid.generate().bytes;
          const data: MessageInitShape<typeof NodeHttpInsertSchema> = { nodeId };

          const file = fileCollection.get(key.toString())!;

          if (file.kind === FileKind.HTTP) {
            data.httpId = file.fileId;
          } else if (file.kind === FileKind.HTTP_DELTA) {
            data.httpId = file.parentId!;
            data.deltaHttpId = file.fileId;
          } else {
            return;
          }

          nodeHttpCollection.utils.insert(data);
          insertNode({ handleKind, kind: NodeKind.HTTP, name: 'http', nodeId, position, sourceId, targetId });
        }}
        showControls
      />
    </>
  );
};
