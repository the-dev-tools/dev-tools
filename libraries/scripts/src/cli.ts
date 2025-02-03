import { Args, Command } from '@effect/cli';
import { FileSystem, Path } from '@effect/platform';
import { NodeContext, NodeRuntime } from '@effect/platform-node';
import { Array, Boolean, Cause, Effect, Option, pipe, Record } from 'effect';
import { releaseChangelog, releaseVersion } from 'nx/release/index.js';
import { type NxReleaseArgs } from 'nx/src/command-line/release/command-object.js';

import { Repository } from './repository.ts';

const resolveMonorepoRoot = Effect.gen(function* () {
  const path = yield* Path.Path;
  const fs = yield* FileSystem.FileSystem;

  let dir = process.cwd();

  while (yield* pipe(path.resolve(dir, 'nx.json'), fs.exists, Effect.map(Boolean.not))) {
    const nextDir = path.resolve(dir, '..');
    if (nextDir === dir) yield* new Cause.NoSuchElementException('Unable to resolve monorepo root');
    dir = nextDir;
  }

  return dir;
});

type ReleaseWorkflow =
  | 'release-chrome-extension.yaml'
  | 'release-cloudflare-pages.yaml'
  | 'release-electron-builder.yaml';

const ReleaseWorkflows: Record<string, ReleaseWorkflow> = {
  'api-recorder-extension': 'release-chrome-extension.yaml',
  desktop: 'release-electron-builder.yaml',
  web: 'release-cloudflare-pages.yaml',
};

const release = Command.make(
  'release',
  {
    projects: pipe(
      Args.choice(
        pipe(
          ReleaseWorkflows,
          Record.keys,
          Array.map((_) => [_, _]),
        ),
        { name: 'projects' },
      ),
      Args.atLeast(1),
    ),
  },
  Effect.fn(function* ({ projects }) {
    const repo = yield* Repository;

    process.chdir(yield* resolveMonorepoRoot);

    const options: NxReleaseArgs = { projects, verbose: true };

    const { projectsVersionData } = yield* Effect.tryPromise(() => releaseVersion(options));

    const { projectChangelogs = {} } = yield* Effect.tryPromise(() =>
      releaseChangelog({
        versionData: projectsVersionData,
        gitCommitMessage: 'Version projects',
        deleteVersionPlans: true,
        ...options,
      }),
    );

    yield* pipe(
      Record.filterMap(projectChangelogs, ({ releaseVersion: { gitTag } }, project) =>
        pipe(
          ReleaseWorkflows[project],
          Option.fromNullable,
          Option.map((_) =>
            repo.dispatchWorkflow({
              workflow: _,
              ref: gitTag,
            }),
          ),
        ),
      ),
      Effect.all,
    );
  }),
);

const uploadReleaseAssets = Command.make(
  'upload-release-assets',
  { files: pipe(Args.file({ name: 'files' }), Args.atLeast(1)) },
  Effect.fn(function* ({ files }) {
    const repo = yield* Repository;
    const path = yield* Path.Path;
    const root = yield* resolveMonorepoRoot;

    const tag = yield* repo.tag;
    const { id } = yield* repo.getReleaseByTag(tag);

    yield* pipe(
      files.map((_) => repo.uploadReleaseAsset({ id, path: path.resolve(root, _) })),
      Effect.all,
    );
  }),
);

pipe(
  Command.make('scripts'),
  Command.withSubcommands([release, uploadReleaseAssets]),
  Command.run({ name: 'Internal scripts', version: '' }),
  (_) => _(process.argv),
  Effect.provide(NodeContext.layer),
  Effect.provide(Repository.Default),
  NodeRuntime.runMain,
);
