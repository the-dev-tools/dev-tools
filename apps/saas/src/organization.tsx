import { createQueryOptions, useMutation as useConnectMutation, useTransport } from '@connectrpc/connect-query';
import { useQuery, useQueryClient } from '@tanstack/react-query';
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
  const queryClient = useQueryClient();
  const transport = useTransport();

  const { redirect } = route.useSearch();

  const organizationsQueryOptions = createQueryOptions(getOrganizations, undefined, { transport });
  const organizationsQuery = useQuery({ ...organizationsQueryOptions, enabled: true });
  const createOrganizationMutation = useConnectMutation(createOrganization);

  if (!organizationsQuery.isSuccess) return null;
  const { organizations } = organizationsQuery.data;

  return (
    <div className='flex size-full flex-col items-center justify-center gap-4'>
      <div>
        <Button
          onPress={async () => {
            await createOrganizationMutation.mutateAsync({ name: 'New organization' });
            await queryClient.invalidateQueries(organizationsQueryOptions);
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
