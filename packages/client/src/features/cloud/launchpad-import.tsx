import { eq } from '@tanstack/react-db';
import { useQueryClient } from '@tanstack/react-query';
import { Ulid } from 'id128';
import { useEffect } from 'react';
import { ImportMissingDataKind, ImportService } from '@the-dev-tools/spec/buf/api/import/v1/import_pb';
import {
  EnvironmentCollectionSchema,
  EnvironmentVariableCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/environment';
import { useToastQueue } from '@the-dev-tools/ui/toast';
import { track } from '~/app/umami';
import { request, useApiCollection } from '~/shared/api';
import { getNextOrder, pick, queryCollection } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { getRun, type InitRun, type RepoTarget, setRun, useInitRun } from './launchpad-run';
import { cloudKeys, connectErrorText, pickFlow, upsertFlowFile, useNavigateToFlow } from './lib';

/** Imports generated flows into Files or a cloud repo, and opens the first one. */
export const useImportFlows = () => {
  const { transport } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const queryClient = useQueryClient();
  const navigateToFlow = useNavigateToFlow();
  const environmentCollection = useApiCollection(EnvironmentCollectionSchema);
  const variableCollection = useApiCollection(EnvironmentVariableCollectionSchema);

  // Recorded flows call {{ #env:APP_URL }} and the recorded secrets. Locally those resolve to
  // Global Environment variables of the same name: add the missing ones (never overwrite one).
  const ensureVariables = async (values: Record<string, string>) => {
    const globalEnv = (
      await queryCollection((_) =>
        _.from({ environment: environmentCollection })
          .where((_) => eq(_.environment.workspaceId, workspaceId))
          .where((_) => eq(_.environment.isGlobal, true))
          .select((_) => pick(_.environment, 'environmentId')),
      )
    )[0];
    if (!globalEnv) return;

    const existing = new Set(
      (
        await queryCollection((_) =>
          _.from({ variable: variableCollection })
            .where((_) => eq(_.variable.environmentId, globalEnv.environmentId))
            .select((_) => pick(_.variable, 'key')),
        )
      ).map((_) => _.key),
    );

    for (const [key, value] of Object.entries(values)) {
      if (existing.has(key)) continue;
      variableCollection.utils.insert({
        enabled: true,
        environmentId: globalEnv.environmentId,
        environmentVariableId: Ulid.generate().bytes,
        key,
        order: await getNextOrder(variableCollection),
        value,
      });
    }
  };

  const importLocal = async (files: StresseurGeneratedFile[]) => {
    let first: Uint8Array | undefined;
    for (const file of files) {
      const input = { name: file.path, textData: file.text, workspaceId };
      let { message } = await request({ input, method: ImportService.method.import, transport });
      if (message.missingData === ImportMissingDataKind.DOMAIN)
        ({ message } = await request({
          input: { ...input, domainData: message.domains.map((domain) => ({ domain, enabled: true })) },
          method: ImportService.method.import,
          transport,
        }));
      first ??= message.flowId;
    }
    return first;
  };

  const importRepo = async (destination: RepoTarget, files: StresseurGeneratedFile[]) => {
    let first: Uint8Array | undefined;
    for (const file of files) {
      const result = await upsertFlowFile(transport, workspaceId, {
        apiUrl: destination.apiUrl,
        cloudWorkspaceSlug: destination.workspaceSlug,
        path: `api-tests/${file.path}`,
        repoName: destination.repo.full_name,
        repoSlug: destination.repo.slug,
        sha: destination.repo.head_sha ?? '',
        yamlText: file.text,
      });
      if (result.kind === 'dirty') throw new Error(`api-tests/${file.path} has unsaved edits`);
      if (result.kind === 'error') throw new Error(result.message);
      first ??= pickFlow(result.link)?.flowId;
    }
    await queryClient.invalidateQueries({ queryKey: cloudKeys.all });
    return first;
  };

  /** Imports a finished run into `to`, then opens its first flow. */
  return async (run: InitRun, to: 'cloud' | 'files') => {
    const files = run.finished?.files ?? [];
    if (files.length === 0) return;
    const appUrl = run.detected?.url ?? run.url;
    try {
      const flowId =
        to === 'cloud' && run.destination.kind === 'cloud'
          ? await importRepo(run.destination.repo, files)
          : await importLocal(files);
      if (to === 'files') await ensureVariables({ ...(appUrl && { APP_URL: appUrl }), ...run.finished?.env });
      const where = to === 'cloud' && run.destination.kind === 'cloud' ? run.destination.repo.repo.full_name : 'Files';
      track('record_generated', { destination: run.destination.kind, files: files.length });
      setRun({ ...run, imported: { ...(flowId && { flowId }), where }, status: 'finished' });
      if (flowId) await navigateToFlow(flowId);
    } catch (error) {
      track('record_failed');
      setRun({ ...run, importError: connectErrorText(error), status: 'finished' });
    }
  };
};

/** Imports a finished run once: into Files (Files, sample shop) or its cloud repo. A repo folder keeps its flows. */
export const useFinishRun = () => {
  const run = useInitRun();
  const importFlows = useImportFlows();
  const toastQueue = useToastQueue();

  useEffect(() => {
    if (run?.status !== 'importing') return;
    if (run.destination.kind === 'folder' || (run.finished?.files.length ?? 0) === 0) {
      setRun({ ...run, status: 'finished' });
      return;
    }
    // Mark it first, so a re-render does not import twice.
    setRun({ ...run, status: 'finished' });
    void importFlows(run, run.destination.kind === 'cloud' ? 'cloud' : 'files').then(() => {
      const imported = getRun()?.imported;
      if (imported) toastQueue.add({ title: `Flows saved · ${imported.where}` }, { timeout: 5000 });
    });
  }, [importFlows, run, toastQueue]);
};
