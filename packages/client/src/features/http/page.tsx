import { eq, useLiveQuery } from '@tanstack/react-db';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { HttpResponseCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { useApiCollection } from '~/api-new';
import { ReferenceContext } from '~/reference';
import { httpRouteApi, workspaceRouteApi } from '~/routes';
import { pick } from '~/utils/tanstack-db';
import { HttpRequest, HttpTopBar } from './request';
import { ResponsePanel } from './response';

export const HttpPage = () => {
  const { httpId } = httpRouteApi.useLoaderData();
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const responseCollection = useApiCollection(HttpResponseCollectionSchema);

  const { data: { httpResponseId } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: responseCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => pick(_.item, 'httpResponseId'))
        .orderBy((_) => _.item.httpResponseId, 'desc')
        .limit(1)
        .findOne(),
    [responseCollection, httpId],
  );

  return (
    <PanelGroup autoSaveId='endpoint' direction='vertical'>
      <Panel className='flex h-full flex-col' id='request' order={1}>
        <ReferenceContext value={{ httpId, workspaceId }}>
          <HttpTopBar httpId={httpId} />

          <HttpRequest httpId={httpId} />
        </ReferenceContext>
      </Panel>

      {httpResponseId && (
        <>
          <PanelResizeHandle direction='vertical' />

          <Panel defaultSize={40} id='response' order={2}>
            <ResponsePanel fullWidth httpResponseId={httpResponseId} />
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};
