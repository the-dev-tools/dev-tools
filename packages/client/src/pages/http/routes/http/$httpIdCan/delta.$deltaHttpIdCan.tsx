import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { openTab } from '~/widgets/tabs';
import { HttpDeltaPage } from '../../../page';
import { HttpTab, httpTabId } from '../../../tab';

export const Route = createFileRoute(
  '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(http)/http/$httpIdCan/delta/$deltaHttpIdCan',
)({
  component: HttpDeltaPage,
  context: ({ params: { deltaHttpIdCan } }) => {
    const deltaHttpId = Ulid.fromCanonical(deltaHttpIdCan).bytes;
    return { deltaHttpId };
  },
  onEnter: async (match) => {
    const { deltaHttpId, httpId } = match.context;

    await openTab({
      id: httpTabId({ deltaHttpId, httpId }),
      match,
      node: <HttpTab deltaHttpId={deltaHttpId} httpId={httpId} />,
    });
  },
});
