import { useLiveQuery } from '@tanstack/react-db';
import { Match, pipe } from 'effect';
import { useEffect } from 'react';
import { RiAnthropicFill, RiGeminiFill, RiOpenaiFill } from 'react-icons/ri';
import { TbGauge } from 'react-icons/tb';
import { CredentialKind } from '@the-dev-tools/spec/buf/api/credential/v1/credential_pb';
import { CredentialCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/credential';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { eqStruct, pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { useCloseTab } from '~/widgets/tabs';

interface CredentialTabProps {
  credentialId: Uint8Array;
}

export const credentialTabId = (credentialId: Uint8Array) =>
  JSON.stringify({ credentialId, route: routes.dashboard.workspace.flow.route.id });

export const CredentialTab = ({ credentialId }: CredentialTabProps) => {
  const closeTab = useCloseTab();

  const collection = useApiCollection(CredentialCollectionSchema);

  const credential = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where(eqStruct({ credentialId }))
        .select((_) => pick(_.item, 'name', 'kind'))
        .findOne(),
    [collection, credentialId],
  ).data;

  const credentialExists = credential !== undefined;

  useEffect(() => {
    if (!credentialExists) void closeTab(credentialTabId(credentialId));
  }, [credentialExists, credentialId, closeTab]);

  return (
    <>
      {pipe(
        Match.value(credential?.kind),
        Match.when(CredentialKind.OPEN_AI, () => <RiOpenaiFill className={tw`size-4 shrink-0 text-fg-muted`} />),
        Match.when(CredentialKind.ANTHROPIC, () => <RiAnthropicFill className={tw`size-4 shrink-0 text-fg-muted`} />),
        Match.when(CredentialKind.GEMINI, () => <RiGeminiFill className={tw`size-4 shrink-0 text-fg-muted`} />),
        Match.orElse(() => <TbGauge className={tw`size-4 shrink-0 text-fg-muted`} />),
      )}

      <span className={tw`min-w-0 flex-1 truncate`}>{credential?.name}</span>
    </>
  );
};
