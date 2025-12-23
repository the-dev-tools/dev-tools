import { eq, useLiveQuery } from '@tanstack/react-db';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { HttpResponseCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { useApiCollection } from '~/api';
import { ReferenceContext } from '~/reference';
import { httpDeltaRouteApi, httpRouteApi, workspaceRouteApi } from '~/routes';
import { pick } from '~/utils/tanstack-db';
import { HttpRequestPanel, HttpTopBar } from './request';
import { ResponseInfo, ResponsePanel } from './response';

export const HttpPage = () => {
  const { httpId } = httpRouteApi.useRouteContext();
  return <Page httpId={httpId} />;
};

export const HttpDeltaPage = () => {
  const { deltaHttpId, httpId } = httpDeltaRouteApi.useRouteContext();
  return <Page deltaHttpId={deltaHttpId} httpId={httpId} />;
};

interface PageProps {
  deltaHttpId?: Uint8Array;
  httpId: Uint8Array;
}

const Page = ({ deltaHttpId, httpId }: PageProps) => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const responseCollection = useApiCollection(HttpResponseCollectionSchema);

  const { httpResponseId } =
    useLiveQuery(
      (_) =>
        _.from({ item: responseCollection })
          .where((_) => eq(_.item.httpId, deltaHttpId ?? httpId))
          .select((_) => pick(_.item, 'httpResponseId'))
          .orderBy((_) => _.item.httpResponseId, 'desc')
          .limit(1)
          .findOne(),
      [responseCollection, deltaHttpId, httpId],
    ).data ?? {};

  return (
    <PanelGroup autoSaveId='endpoint' direction='vertical'>
      <Panel className='flex h-full flex-col' id='request' order={1}>
        <ReferenceContext value={{ httpId, workspaceId, ...(deltaHttpId && { deltaHttpId }) }}>
          <HttpTopBar deltaHttpId={deltaHttpId} httpId={httpId} />

          <HttpRequestPanel deltaHttpId={deltaHttpId} httpId={httpId} />
        </ReferenceContext>
      </Panel>

      {httpResponseId && (
        <>
          <PanelResizeHandle direction='vertical' />

          <Panel defaultSize={40} id='response' order={2}>
            <ResponsePanel fullWidth httpResponseId={httpResponseId}>
              <ResponseInfo httpResponseId={httpResponseId} />
            </ResponsePanel>
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};
