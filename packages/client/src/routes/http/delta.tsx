import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { HttpDeltaPage, HttpTab, httpTabId } from '~/features/http';
import { openTab } from '~/tabs';
import { ErrorComponent } from '../error';

export const Route = createFileRoute('/workspace/$workspaceIdCan/http/$httpIdCan/delta/$deltaHttpIdCan')({
  component: HttpDeltaPage,
  context: ({ params: { deltaHttpIdCan } }) => {
    const deltaHttpId = Ulid.fromCanonical(deltaHttpIdCan).bytes;
    return { deltaHttpId };
  },
  errorComponent: (props) => <ErrorComponent {...props} />,
  onEnter: async (match) => {
    const { deltaHttpId, httpId } = match.context;

    await openTab({
      id: httpTabId({ deltaHttpId, httpId }),
      match,
      node: <HttpTab deltaHttpId={deltaHttpId} httpId={httpId} />,
    });
  },
});
