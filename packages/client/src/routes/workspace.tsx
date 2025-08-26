import { createFileRoute } from '@tanstack/react-router';
import { pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { makeTabsAtom, TabsRouteContext } from '@the-dev-tools/ui/router';
import { workspaceRouteApi } from '~routes';
import { WorkspaceLayout } from '~workspace/layout';

export class WorkspaceRouteSearch extends Schema.Class<WorkspaceRouteSearch>('WorkspaceRouteSearch')({
  showLogs: pipe(Schema.Boolean, Schema.optional),
}) {}

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute('/workspace/$workspaceIdCan')({
  validateSearch: (_) => Schema.decodeSync(WorkspaceRouteSearch)(_),
  context: ({ params: { workspaceIdCan } }): Omit<TabsRouteContext, 'runtime'> => ({
    baseRoute: { params: { workspaceIdCan }, to: workspaceRouteApi.id },
    tabsAtom: makeTabsAtom(),
  }),
  loader: ({ params: { workspaceIdCan } }) => {
    const workspaceId = Ulid.fromCanonical(workspaceIdCan).bytes;
    return { workspaceId };
  },
  component: WorkspaceLayout,
});
/* eslint-enable perfectionist/sort-objects */
