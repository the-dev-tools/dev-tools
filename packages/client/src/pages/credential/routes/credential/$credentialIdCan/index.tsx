import { create } from '@bufbuild/protobuf';
import { useLiveQuery } from '@tanstack/react-db';
import { createFileRoute } from '@tanstack/react-router';
import { Match, pipe } from 'effect';
import { Ulid } from 'id128';
import {
  CredentialAnthropicSchema,
  CredentialAnthropicUpdate_BaseUrlUnion_Kind,
  CredentialGeminiSchema,
  CredentialGeminiUpdate_BaseUrlUnion_Kind,
  CredentialKind,
  CredentialOpenAiSchema,
  CredentialOpenAiUpdate_BaseUrlUnion_Kind,
  CredentialSchema,
} from '@the-dev-tools/spec/buf/api/credential/v1/credential_pb';
import { Unset } from '@the-dev-tools/spec/buf/global/v1/global_pb';
import {
  CredentialAnthropicCollectionSchema,
  CredentialCollectionSchema,
  CredentialGeminiCollectionSchema,
  CredentialOpenAiCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/credential';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { CredentialTab, credentialTabId } from '~/pages/credential/tab';
import { useApiCollection } from '~/shared/api';
import { eqStruct, pick } from '~/shared/lib';
import { openTab } from '~/widgets/tabs';

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute(
  '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(credential)/credential/$credentialIdCan/',
)({
  loader: ({ params: { credentialIdCan } }) => {
    const credentialId = Ulid.fromCanonical(credentialIdCan).bytes;
    return { credentialId };
  },
  component: RouteComponent,
  onEnter: async (match) => {
    if (!match.loaderData) return;

    const { credentialId } = match.loaderData;

    await openTab({
      id: credentialTabId(credentialId),
      match,
      node: <CredentialTab credentialId={credentialId} />,
    });
  },
});
/* eslint-enable perfectionist/sort-objects */

function RouteComponent() {
  const { credentialId } = Route.useLoaderData();

  const collection = useApiCollection(CredentialCollectionSchema);

  const { kind } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ credentialId }))
          .select((_) => pick(_.item, 'kind'))
          .findOne(),
      [collection, credentialId],
    ).data ?? create(CredentialSchema);

  const content = pipe(
    Match.value(kind),
    Match.when(CredentialKind.OPEN_AI, () => <OpenAiCredentials />),
    Match.when(CredentialKind.GEMINI, () => <GeminiCredentials />),
    Match.when(CredentialKind.ANTHROPIC, () => <AnthropicCredentials />),
    Match.orElse(() => null),
  );

  return <div className={tw`flex flex-col gap-4 px-6 py-4`}>{content}</div>;
}

const OpenAiCredentials = () => {
  const { credentialId } = Route.useLoaderData();

  const collection = useApiCollection(CredentialOpenAiCollectionSchema);

  const data =
    useLiveQuery(
      (_) => _.from({ item: collection }).where(eqStruct({ credentialId })).findOne(),
      [collection, credentialId],
    ).data ?? create(CredentialOpenAiSchema);

  return (
    <>
      <TextInputField
        label='Token'
        onChange={(_) => collection.utils.updatePaced({ credentialId, token: _ })}
        type='password'
        value={data.token}
      />

      <TextInputField
        label='Base URL'
        onChange={(_) =>
          collection.utils.updatePaced({
            baseUrl: _
              ? { kind: CredentialOpenAiUpdate_BaseUrlUnion_Kind.VALUE, value: _ }
              : { kind: CredentialOpenAiUpdate_BaseUrlUnion_Kind.UNSET, unset: Unset.UNSET },
            credentialId,
          })
        }
        value={data.baseUrl ?? ''}
      />
    </>
  );
};

const GeminiCredentials = () => {
  const { credentialId } = Route.useLoaderData();

  const collection = useApiCollection(CredentialGeminiCollectionSchema);

  const data =
    useLiveQuery(
      (_) => _.from({ item: collection }).where(eqStruct({ credentialId })).findOne(),
      [collection, credentialId],
    ).data ?? create(CredentialGeminiSchema);

  return (
    <>
      <TextInputField
        label='API Key'
        onChange={(_) => collection.utils.updatePaced({ apiKey: _, credentialId })}
        type='password'
        value={data.apiKey}
      />

      <TextInputField
        label='Base URL'
        onChange={(_) =>
          collection.utils.updatePaced({
            baseUrl: _
              ? { kind: CredentialGeminiUpdate_BaseUrlUnion_Kind.VALUE, value: _ }
              : { kind: CredentialGeminiUpdate_BaseUrlUnion_Kind.UNSET, unset: Unset.UNSET },
            credentialId,
          })
        }
        value={data.baseUrl ?? ''}
      />
    </>
  );
};

const AnthropicCredentials = () => {
  const { credentialId } = Route.useLoaderData();

  const collection = useApiCollection(CredentialAnthropicCollectionSchema);

  const data =
    useLiveQuery(
      (_) => _.from({ item: collection }).where(eqStruct({ credentialId })).findOne(),
      [collection, credentialId],
    ).data ?? create(CredentialAnthropicSchema);

  return (
    <>
      <TextInputField
        label='API Key'
        onChange={(_) => collection.utils.updatePaced({ apiKey: _, credentialId })}
        type='password'
        value={data.apiKey}
      />

      <TextInputField
        label='Base URL'
        onChange={(_) =>
          collection.utils.updatePaced({
            baseUrl: _
              ? { kind: CredentialAnthropicUpdate_BaseUrlUnion_Kind.VALUE, value: _ }
              : { kind: CredentialAnthropicUpdate_BaseUrlUnion_Kind.UNSET, unset: Unset.UNSET },
            credentialId,
          })
        }
        value={data.baseUrl ?? ''}
      />
    </>
  );
};
