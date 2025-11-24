import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { addTab } from '@the-dev-tools/ui/router';
import { HttpDeltaPage, HttpTab, httpTabId } from '~/features/http';
import { ErrorComponent } from '../error';

export const Route = createFileRoute('/workspace/$workspaceIdCan/http/$httpIdCan/delta/$deltaHttpIdCan')({
  component: HttpDeltaPage,
  context: ({ params: { deltaHttpIdCan } }) => {
    const deltaHttpId = Ulid.fromCanonical(deltaHttpIdCan).bytes;
    return { deltaHttpId };
  },
  errorComponent: (props) => <ErrorComponent {...props} />,
  onEnter: (match) => {
    const { deltaHttpId, httpId } = match.context;

    addTab({
      id: httpTabId({ deltaHttpId, httpId }),
      match,
      node: <HttpTab deltaHttpId={deltaHttpId} httpId={httpId} />,
    });
  },
});
