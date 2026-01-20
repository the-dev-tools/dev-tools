import { useLiveQuery } from '@tanstack/react-db';
import { useEffect } from 'react';
import { TbGauge } from 'react-icons/tb';
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
        .select((_) => pick(_.item, 'name'))
        .findOne(),
    [collection, credentialId],
  ).data;

  const credentialExists = credential !== undefined;

  useEffect(() => {
    if (!credentialExists) void closeTab(credentialTabId(credentialId));
  }, [credentialExists, credentialId, closeTab]);

  return (
    <>
      <TbGauge className={tw`size-5 shrink-0 text-slate-500`} />
      <span className={tw`min-w-0 flex-1 truncate`}>{credential?.name}</span>
    </>
  );
};
