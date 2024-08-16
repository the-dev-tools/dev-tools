import { createQueryOptions, useMutation as useConnectMutation, useTransport } from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, useRouter } from '@tanstack/react-router';
import { Effect, pipe, Struct } from 'effect';
import { useState } from 'react';
import { Button, Form, Input, TextField } from 'react-aria-components';

import { setOrganizationId } from '@the-dev-tools/api/auth';
import { Organization } from '@the-dev-tools/protobuf/organization/v1/organization_pb';
import {
  createOrganization,
  deleteOrganization,
  getOrganizations,
  updateOrganization,
} from '@the-dev-tools/protobuf/organization/v1/organization-OrganizationService_connectquery';

import { Runtime } from './runtime';

const route = getRouteApi('/user/organizations');

class OrganizationRenameForm extends Schema.Class<OrganizationRenameForm>('OrganizationEditForm')({
  name: Schema.String,
}) {}

export const OrganizationsPage = () => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  const queryOptions = createQueryOptions(getOrganizations, undefined, { transport });
  const query = useQuery({ ...queryOptions, enabled: true });
  const createMutation = useConnectMutation(createOrganization);

  if (!query.isSuccess) return null;
  const { organizations } = query.data;

  return (
    <div className='flex size-full flex-col items-center justify-center gap-4'>
      <div>
        <Button
          onPress={async () => {
            await createMutation.mutateAsync({ name: 'New organization' });
            await queryClient.invalidateQueries(queryOptions);
          }}
        >
          Create organization
        </Button>
      </div>

      {organizations.map((_) => (
        <OrganizationRow key={_.organizationId} organization={_} />
      ))}
    </div>
  );
};

interface OrganizationRowProps {
  organization: Organization;
}

const OrganizationRow = ({ organization }: OrganizationRowProps) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  const router = useRouter();

  const { redirect } = route.useSearch();

  const [renaming, setRenaming] = useState(false);

  const queryOptions = createQueryOptions(getOrganizations, undefined, { transport });

  const updateMutation = useConnectMutation(updateOrganization);
  const deleteMutation = useConnectMutation(deleteOrganization);

  return (
    <div className='flex gap-4'>
      {renaming ? (
        <Form
          className='contents'
          onSubmit={(event) =>
            Effect.gen(function* () {
              event.preventDefault();

              const { name } = yield* pipe(
                new FormData(event.currentTarget),
                Object.fromEntries,
                Schema.decode(OrganizationRenameForm),
              );

              const data = Struct.evolve(organization, { name: () => name });
              yield* Effect.tryPromise(() => updateMutation.mutateAsync(data));
              yield* Effect.tryPromise(() => queryClient.invalidateQueries(queryOptions));

              setRenaming(false);
            }).pipe(Runtime.runPromise)
          }
        >
          {/* eslint-disable-next-line jsx-a11y/no-autofocus */}
          <TextField aria-label='Organization name' name='name' defaultValue={organization.name} autoFocus>
            <Input />
          </TextField>
          <Button type='submit'>Save</Button>
        </Form>
      ) : (
        <>
          <Button
            onPress={() =>
              Effect.gen(function* () {
                yield* setOrganizationId(organization.organizationId);
                if (redirect) yield* Effect.try(() => void router.history.push(redirect));
                else yield* Effect.tryPromise(() => router.navigate({ to: '/' }));
              }).pipe(Runtime.runPromise)
            }
          >
            {organization.name}
          </Button>
          <Button onPress={() => void setRenaming(true)}>Rename</Button>
        </>
      )}
      <Button
        onPress={async () => {
          await deleteMutation.mutateAsync({ organizationId: organization.organizationId });
          await queryClient.invalidateQueries(queryOptions);
        }}
      >
        Delete
      </Button>
    </div>
  );
};
