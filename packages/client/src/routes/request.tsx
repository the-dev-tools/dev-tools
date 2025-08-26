import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { addTab } from '@the-dev-tools/ui/router';
import { EndpointPage, EndpointTab, endpointTabId } from '~endpoint';
import { ErrorComponent } from './error';

export class EndpointRouteSearch extends Schema.Class<EndpointRouteSearch>('EndpointRouteSearch')({
  responseIdCan: pipe(Schema.String, Schema.optional),
}) {}

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute('/workspace/$workspaceIdCan/request/$endpointIdCan/$exampleIdCan')({
  validateSearch: (_) => Schema.decodeSync(EndpointRouteSearch)(_),
  loaderDeps: (_) => Struct.pick(_.search, 'responseIdCan'),
  loader: ({ deps: { responseIdCan }, params: { endpointIdCan, exampleIdCan } }) => {
    const endpointId = Ulid.fromCanonical(endpointIdCan).bytes;
    const exampleId = Ulid.fromCanonical(exampleIdCan).bytes;
    const responseId = pipe(
      Option.fromNullable(responseIdCan),
      Option.map((_) => Ulid.fromCanonical(_).bytes),
    );

    return { endpointId, exampleId, responseId };
  },
  component: RouteComponent,
  errorComponent: (props) => <ErrorComponent {...props} />,
  onEnter: (match) => {
    if (!match.loaderData) return;

    const { endpointId, exampleId } = match.loaderData;

    addTab({
      id: endpointTabId({ endpointId, exampleId }),
      match,
      node: <EndpointTab endpointId={endpointId} />,
    });
  },
  shouldReload: false,
});
/* eslint-enable perfectionist/sort-objects */

function RouteComponent() {
  return (
    <QueryErrorResetBoundary>
      <EndpointPage />
    </QueryErrorResetBoundary>
  );
}
