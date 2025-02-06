import { Args, Command as CliCommand } from '@effect/cli';
import { FileSystem, Path, Command as PlatformCommand } from '@effect/platform';
import { NodeContext, NodeRuntime } from '@effect/platform-node';
import { Array, Boolean, Cause, Config, Effect, Option, pipe, Record, Schema, String, Struct } from 'effect';
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

class ProjectInfo extends Schema.Class<ProjectInfo>('ProjectInfo')({
  root: Schema.String,
}) {}

const getProjectInfo = Effect.fn(function* (name: string) {
  const path = yield* Path.Path;
  const root = yield* resolveMonorepoRoot;
  return yield* pipe(
    PlatformCommand.make('pnpm', 'nx show project', name, '--json'),
    PlatformCommand.string,
    Effect.flatMap(Schema.decode(Schema.parseJson(ProjectInfo))),
    Effect.map(Struct.evolve({ root: (_) => path.resolve(root, _) })),
  );
});

const exportProjectInfo = CliCommand.make(
  'export-project-info',
  {},
  Effect.fn(function* () {
    const fs = yield* FileSystem.FileSystem;
    const repo = yield* Repository;

    const { name, version } = yield* repo.project;
    const { root } = yield* getProjectInfo(name);

    const output = yield* Config.string('GITHUB_OUTPUT');

    const info = pipe(
      { name, version, root },
      Record.map((value, key) => String.camelToSnake(key) + '=' + value),
      Record.values,
      Array.join('\n'),
    );

    yield* fs.writeFileString(output, info);
  }, Effect.provide(Repository.Default)),
);

const goInstallTools = CliCommand.make(
  'go-install-tools',
  {},
  Effect.fn(function* () {
    const path = yield* Path.Path;
    const fs = yield* FileSystem.FileSystem;
    const root = yield* resolveMonorepoRoot;

    const tools = yield* pipe(
      path.resolve(root, 'libraries', 'tools', 'tools.go'),
      fs.readFileString,
      Effect.flatMap(String.match(/(?<=_ ").*(?=")/g)),
    );

    for (const tool of tools) {
      yield* pipe(
        PlatformCommand.make('go', 'install', tool),
        PlatformCommand.stdout('inherit'),
        PlatformCommand.stderr('inherit'),
        PlatformCommand.exitCode,
      );
    }
  }),
);

type ReleaseWorkflow =
  | 'release-chrome-extension.yaml'
  | 'release-cloudflare-pages.yaml'
  | 'release-electron-builder.yaml';

const ReleaseWorkflows: Record<string, ReleaseWorkflow> = {
  'api-recorder-extension': 'release-chrome-extension.yaml',
  desktop: 'release-electron-builder.yaml',
  web: 'release-cloudflare-pages.yaml',
};

const release = CliCommand.make(
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

    const { projectsVersionData } = yield* Effect.tryPromise(() => releaseVersion({ gitCommit: false, ...options }));

    const { projectChangelogs = {} } = yield* Effect.tryPromise(() =>
      releaseChangelog({
        versionData: projectsVersionData,
        gitCommitMessage: 'Version projects',
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
  }, Effect.provide(Repository.Default)),
);

const uploadReleaseAssets = CliCommand.make(
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
  }, Effect.provide(Repository.Default)),
);

pipe(
  CliCommand.make('scripts'),
  CliCommand.withSubcommands([exportProjectInfo, goInstallTools, release, uploadReleaseAssets]),
  CliCommand.run({ name: 'Internal scripts', version: '' }),
  (_) => _(process.argv),
  Effect.provide(NodeContext.layer),
  NodeRuntime.runMain,
);
