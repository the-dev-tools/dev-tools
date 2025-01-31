import { createActionAuth } from '@octokit/auth-action';
import { Octokit } from '@octokit/rest';
import { releaseChangelog, releaseVersion } from 'nx/release/index.js';
import { type NxReleaseArgs } from 'nx/src/command-line/release/command-object.js';

import { goToRoot } from './go-to-root.ts';

goToRoot();

const [owner, repo] = process.env['GITHUB_REPOSITORY']?.split('/') ?? [];
if (!owner || !repo) throw new Error('Could not determine owner/repo');

const octokit = new Octokit({ authStrategy: createActionAuth });

type ReleaseWorkflow =
  | 'release-chrome-extension.yaml'
  | 'release-cloudflare-pages.yaml'
  | 'release-electron-builder.yaml';

const ReleaseWorkflows: Record<string, ReleaseWorkflow> = {
  'api-recorder-extension': 'release-chrome-extension.yaml',
  desktop: 'release-electron-builder.yaml',
  web: 'release-cloudflare-pages.yaml',
};

const options: NxReleaseArgs = { verbose: true };

const { projectsVersionData } = await releaseVersion(options);

const { projectChangelogs = {} } = await releaseChangelog({
  versionData: projectsVersionData,
  gitCommitMessage: 'Version projects',
  ...options,
});

for (const [project, { releaseVersion }] of Object.entries(projectChangelogs)) {
  const releaseWorkflow = ReleaseWorkflows[project];
  if (!releaseWorkflow) continue;

  console.log(`Dispatching workflow '${releaseWorkflow}' at '${releaseVersion.gitTag}'`);

  await octokit.rest.actions.createWorkflowDispatch({
    owner,
    repo,
    ref: releaseVersion.gitTag,
    workflow_id: releaseWorkflow,
  });
}

process.exit();
