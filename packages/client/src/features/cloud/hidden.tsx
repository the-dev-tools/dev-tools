import { Ulid } from 'id128';
import { useMemo } from 'react';
import { CloudLinkKind } from '@the-dev-tools/spec/buf/api/cloud_link/v1/cloud_link_pb';
import { useCloudLinks } from './lib';

/** Canonical ids of repo folders mirrored from Stresseur Cloud. They stay out of the Files tree. */
export const useHiddenCloudFileIds = () => {
  const links = useCloudLinks();
  return useMemo(
    () =>
      new Set(
        (links.data ?? [])
          .filter((_) => _.kind === CloudLinkKind.REPO)
          .map((_) => Ulid.construct(_.fileId).toCanonical()),
      ),
    [links.data],
  );
};
