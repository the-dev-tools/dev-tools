import { create } from '@bufbuild/protobuf';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { Position, useReactFlow } from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiExternalLink, FiX } from 'react-icons/fi';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import { endpointGet } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import {
  exampleCreate,
  exampleGet,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { collectionGet } from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { NodeRequest, NodeRequestSchema } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { SendRequestIcon } from '@the-dev-tools/ui/icons';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { CollectionListTree } from '../../collection';
import { EndpointRequestView, ResponsePanel, useEndpointUrl } from '../../endpoint';
import { ReferenceContext } from '../../reference';
import { FlowContext, flowRoute, Handle, useSetSelectedNodes, workspaceRoute } from '../internal';
import { NodeBase, NodePanelProps, NodeProps } from '../node';

export const RequestNode = (props: NodeProps) => {
  const { id, data } = props;
  const { updateNodeData } = useReactFlow();

  const exampleCreateMutation = useConnectMutation(exampleCreate);

  return (
    <>
      <NodeBase {...props} Icon={SendRequestIcon} title='Send Request'>
        <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
          {data.request?.exampleId.length !== 0 ? (
            <RequestNodeSelected request={data.request!} />
          ) : (
            <CollectionListTree
              onAction={async ({ collectionId, endpointId, exampleId }) => {
                if (collectionId === undefined || endpointId === undefined || exampleId === undefined) return;
                const { exampleId: deltaExampleId } = await exampleCreateMutation.mutateAsync({ endpointId });
                const request = create(NodeRequestSchema, {
                  ...data.request!,
                  collectionId,
                  endpointId,
                  exampleId,
                  deltaExampleId,
                });
                updateNodeData(id, { ...data, request });
              }}
            />
          )}
        </div>
      </NodeBase>

      <Handle type='target' position={Position.Top} />
      <Handle type='source' position={Position.Bottom} />
    </>
  );
};

interface RequestNodeSelectedProps {
  request: NodeRequest;
}

const RequestNodeSelected = ({ request: { collectionId, endpointId, exampleId } }: RequestNodeSelectedProps) => {
  const { transport } = flowRoute.useRouteContext();

  const [
    {
      data: { name: collectionName },
    },
    {
      data: { method },
    },
    {
      data: { name },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(collectionGet, { collectionId }, { transport }),
      createQueryOptions(endpointGet, { endpointId }, { transport }),
      createQueryOptions(exampleGet, { exampleId }, { transport }),
    ],
  });

  return (
    <div className={tw`space-y-1.5 p-2`}>
      <div className={tw`truncate text-xs leading-4 tracking-tight text-slate-400`}>{collectionName}</div>
      <div className={tw`flex items-center gap-1.5`}>
        <MethodBadge method={method} />
        <div className={tw`flex-1 truncate text-xs font-medium leading-5 tracking-tight text-slate-800`}>{name}</div>
        <ButtonAsLink
          variant='ghost'
          className={tw`p-0.5`}
          href={{
            to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
            params: {
              endpointIdCan: Ulid.construct(endpointId).toCanonical(),
              exampleIdCan: Ulid.construct(exampleId).toCanonical(),
            },
          }}
        >
          <FiExternalLink className={tw`size-4 text-slate-500`} />
        </ButtonAsLink>
      </div>
    </div>
  );
};

export const RequestPanel = ({ node: { nodeId, request } }: NodePanelProps) => {
  const { collectionId, endpointId, exampleId, deltaExampleId } = request!;
  const { isReadOnly = false } = use(FlowContext);

  const setSelectedNodes = useSetSelectedNodes();

  const { transport } = flowRoute.useRouteContext();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const [{ data: collection }, { data: endpoint }, { data: example }] = useSuspenseQueries({
    queries: [
      createQueryOptions(collectionGet, { collectionId }, { transport }),
      createQueryOptions(endpointGet, { endpointId }, { transport }),
      createQueryOptions(exampleGet, { exampleId }, { transport }),
    ],
  });

  const url = useEndpointUrl({ endpointId, exampleId });

  const { lastResponseId } = example;

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div className='min-w-0'>
          <div className={tw`text-md leading-5 text-slate-400`}>{collection.name}</div>
          <div className={tw`truncate text-sm font-medium leading-5 text-slate-800`}>{example.name}</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink
          variant='ghost'
          className={tw`shrink-0 px-2`}
          href={{
            to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
            params: {
              endpointIdCan: Ulid.construct(endpointId).toCanonical(),
              exampleIdCan: Ulid.construct(exampleId).toCanonical(),
            },
          }}
        >
          <FiExternalLink className={tw`size-4 text-slate-500`} />
          Open API
        </ButtonAsLink>

        <div className={tw`ml-2 mr-3 h-5 w-px shrink-0 bg-slate-300`} />

        <Button variant='ghost' className={tw`p-1`} onPress={() => void setSelectedNodes()}>
          <FiX className={tw`size-5 text-slate-500`} />
        </Button>
      </div>

      <div className='m-5 mb-4 flex flex-1 items-center gap-3 rounded-lg border border-slate-300 px-3 py-2 shadow-sm'>
        <MethodBadge method={endpoint.method} size='lg' />
        <div className={tw`h-7 w-px bg-slate-200`} />
        <div className={tw`truncate font-medium leading-5 tracking-tight text-slate-800`}>{url}</div>
      </div>

      <div className={tw`mx-5 overflow-auto rounded-lg border border-slate-200`}>
        <div
          className={tw`border-b border-slate-200 bg-slate-50 px-3 py-2 text-md font-medium leading-5 tracking-tight text-slate-800`}
        >
          Request
        </div>

        <ReferenceContext value={{ nodeId, exampleId, workspaceId }}>
          <EndpointRequestView
            className={tw`p-5 pt-3`}
            exampleId={exampleId}
            deltaExampleId={deltaExampleId}
            isReadOnly={isReadOnly}
          />
        </ReferenceContext>
      </div>

      {lastResponseId && (
        <div className={tw`mx-5 my-4 overflow-auto rounded-lg border border-slate-200`}>
          <div
            className={tw`border-b border-slate-200 bg-slate-50 px-3 py-2 text-md font-medium leading-5 tracking-tight text-slate-800`}
          >
            Response
          </div>

          <ResponsePanel className={tw`p-5 pt-3`} responseId={lastResponseId} />
        </div>
      )}
    </>
  );
};
