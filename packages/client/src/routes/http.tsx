import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { addTab } from '@the-dev-tools/ui/router';
import { HttpPage, HttpTab, httpTabId } from '~/features/http';
import { ErrorComponent } from './error';

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute('/workspace/$workspaceIdCan/http/$httpIdCan')({
  loader: ({ params: { httpIdCan } }) => {
    const httpId = Ulid.fromCanonical(httpIdCan).bytes;
    return { httpId };
  },
  component: RouteComponent,
  errorComponent: (props) => <ErrorComponent {...props} />,
  onEnter: (match) => {
    if (!match.loaderData) return;

    const { httpId } = match.loaderData;

    addTab({
      id: httpTabId({ httpId }),
      match,
      node: <HttpTab httpId={httpId} />,
    });
  },
  shouldReload: false,
});
/* eslint-enable perfectionist/sort-objects */

function RouteComponent() {
  return (
    <QueryErrorResetBoundary>
      <HttpPage />
    </QueryErrorResetBoundary>
  );
}
