import { createFileRoute } from '@tanstack/react-router';
import { pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { WorkspaceLayout } from '~workspace/layout';

export class WorkspaceRouteSearch extends Schema.Class<WorkspaceRouteSearch>('WorkspaceRouteSearch')({
  showLogs: pipe(Schema.Boolean, Schema.optional),
}) {}

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute('/workspace/$workspaceIdCan')({
  validateSearch: (_) => Schema.decodeSync(WorkspaceRouteSearch)(_),
  loader: ({ params: { workspaceIdCan } }) => {
    const workspaceId = Ulid.fromCanonical(workspaceIdCan).bytes;
    return { workspaceId };
  },
  component: WorkspaceLayout,
});
/* eslint-enable perfectionist/sort-objects */
