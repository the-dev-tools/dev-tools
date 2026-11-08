import { useQueryClient } from '@tanstack/react-query';
import { Ulid } from 'id128';
import { CloudLinkKind } from '@the-dev-tools/spec/buf/api/cloud_link/v1/cloud_link_pb';
import { useToastQueue } from '@the-dev-tools/ui/toast';
import { track } from '~/app/umami';
import { routes } from '~/shared/routes';
import { KeepOrTake, KeepOrTakeDialog, useDialogHost } from './dialogs';
import {
  cloudErrorReason,
  cloudKeys,
  fetchSource,
  pickFlow,
  upsertFlowFile,
  useCloudLinks,
  useNavigateToFlow,
} from './lib';

export interface CloudTarget {
  apiUrl: string;
  repo: StresseurCloudRepo;
  workspaceSlug: string;
}

/**
 * Fetches a flow file (clone first, else Cloud), imports it with CloudLinkUpsert and opens the flow.
 * With unsaved edits it asks Keep mine / Take main. Also used by "Update" in the flow editor.
 */
export const useSyncCloudFlow = (dialogs: ReturnType<typeof useDialogHost>) => {
  const { transport } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const queryClient = useQueryClient();
  const toastQueue = useToastQueue();
  const navigateToFlow = useNavigateToFlow();
  const links = useCloudLinks();

  return async ({ apiUrl, repo, workspaceSlug }: CloudTarget, path: string, flowName?: string, update = false) => {
    const source = await fetchSource(repo, path);
    if (!source.ok) {
      if (source.error) track('cloud_refused', { action: 'open', reason: cloudErrorReason(source.error) });
      toastQueue.add({ title: source.message }, { timeout: 5000 });
      return;
    }

    const existing = links.data?.find(
      (_) =>
        _.kind === CloudLinkKind.FLOW_FILE &&
        _.apiUrl === apiUrl &&
        _.cloudWorkspaceSlug === workspaceSlug &&
        _.repoSlug === repo.slug &&
        _.path === path,
    );

    // Already up to date: open the local copy, edits included.
    if (!update && existing && existing.baseSha === source.sha) {
      const flow = pickFlow(existing, flowName);
      if (flow) {
        track('cloud_flow_opened', { source: source.source, synced: false });
        await navigateToFlow(flow.flowId);
        return;
      }
    }

    const input = {
      apiUrl,
      cloudWorkspaceSlug: workspaceSlug,
      path,
      repoName: repo.full_name,
      repoSlug: repo.slug,
      sha: source.sha,
      yamlText: source.text,
    };

    let result = await upsertFlowFile(transport, workspaceId, input);

    if (result.kind === 'dirty') {
      const choice = await dialogs.ask<KeepOrTake>((done) => <KeepOrTakeDialog done={done} />);
      if (choice === null) return;
      if (choice === 'mine') {
        const flow = existing && pickFlow(existing, flowName);
        if (flow) await navigateToFlow(flow.flowId);
        return;
      }
      result = await upsertFlowFile(transport, workspaceId, { ...input, force: true });
    }

    if (result.kind === 'error') {
      toastQueue.add({ title: result.message }, { timeout: 5000 });
      return;
    }

    if (result.kind !== 'ok') return;

    await queryClient.invalidateQueries({
      queryKey: cloudKeys.links(Ulid.construct(workspaceId).toCanonical()),
    });

    track(update ? 'cloud_flow_updated' : 'cloud_flow_opened', { source: source.source, synced: true });

    const flow = pickFlow(result.link, flowName);
    if (flow) await navigateToFlow(flow.flowId);
  };
};
