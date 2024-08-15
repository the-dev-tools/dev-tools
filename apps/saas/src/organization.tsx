import { useMutation as useConnectMutation, useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { getRouteApi, useRouter } from '@tanstack/react-router';
import { Effect } from 'effect';
import { Button } from 'react-aria-components';

import { setOrganizationId } from '@the-dev-tools/api/auth';
import {
  createOrganization,
  getOrganizations,
} from '@the-dev-tools/protobuf/organization/v1/organization-OrganizationService_connectquery';

import { Runtime } from './runtime';

const route = getRouteApi('/user/organizations');

export const OrganizationsPage = () => {
  const router = useRouter();

  const { redirect } = route.useSearch();

  const organizationsQuery = useConnectQuery(getOrganizations);
  const createOrganizationMutation = useConnectMutation(createOrganization);

  if (!organizationsQuery.isSuccess) return null;
  const { organizations } = organizationsQuery.data;

  return (
    <div className='flex size-full flex-col items-center justify-center gap-4'>
      <div>
        <Button
          onPress={() => {
            createOrganizationMutation.mutate({ name: 'New organization' });
          }}
        >
          Create organization
        </Button>
      </div>

      {organizations.map((_) => (
        <div key={_.organizationId}>
          <Button
            onPress={() =>
              Effect.gen(function* () {
                yield* setOrganizationId(_.organizationId);
                if (redirect) yield* Effect.try(() => void router.history.push(redirect));
                else yield* Effect.tryPromise(() => router.navigate({ to: '/' }));
              }).pipe(Runtime.runPromise)
            }
          >
            {_.name}
          </Button>
        </div>
      ))}
    </div>
  );
};
