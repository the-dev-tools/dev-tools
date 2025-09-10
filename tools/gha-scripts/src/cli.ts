import { Args, Command as CliCommand } from '@effect/cli';
import { FileSystem, Path, Command as PlatformCommand } from '@effect/platform';
import { NodeContext, NodeRuntime } from '@effect/platform-node';
import {
  Array,
  Boolean,
  Cause,
  Config,
  Effect,
  flow,
  Match,
  Option,
  pipe,
  Record,
  Schema,
  String,
  Struct,
} from 'effect';
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
    PlatformCommand.make('pnpm', 'nx', 'show', 'project', name, '--json'),
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
      { name, root, version },
      Record.map((value, key) => String.camelToSnake(key) + '=' + value),
      Record.values,
      Array.join('\n'),
    );

    yield* fs.writeFileString(output, info);
  }, Effect.provide(Repository.Default)),
);

type ReleaseWorkflow =
  | 'release-chrome-extension.yaml'
  | 'release-cloudflare-pages.yaml'
  | 'release-electron-builder.yaml'
  | 'release-go.yaml';

const ReleaseWorkflows: Record<string, ReleaseWorkflow> = {
  'api-recorder-extension': 'release-chrome-extension.yaml',
  desktop: 'release-electron-builder.yaml',
  web: 'release-cloudflare-pages.yaml',
  cli: 'release-go.yaml',
  server: 'release-go.yaml',
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
        gitCommitMessage: 'Version projects',
        versionData: projectsVersionData,
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
              ref: gitTag,
              workflow: _,
            }),
          ),
        ),
      ),
      Effect.all,
    );
  }, Effect.provide(Repository.Default)),
);

const uploadElectronReleaseAssets = CliCommand.make(
  'upload-electron-release-assets',
  {},
  Effect.fn(function* () {
    const path = yield* Path.Path;
    const fs = yield* FileSystem.FileSystem;
    const repo = yield* Repository;

    const tag = yield* repo.tag;
    const { id: releaseId } = yield* repo.getReleaseByTag(tag);
    const { name, version } = yield* repo.project;
    const { root: projectRoot } = yield* getProjectInfo(name);

    const dist = path.join(projectRoot, 'dist');

    const pickPlatformArchForLatest = (file: string) => {
      // Infer platform + arch from filename to support multi-arch builds per runner
      // electron-builder emits:
      //   - Windows: latest.yml, latest-arm64.yml, latest-ia32.yml
      //   - macOS:  latest-mac.yml (runner-specific arch)
      //   - Linux:  latest-linux.yml (runner-specific arch)
      const lower = file.toLowerCase();
      let platform: NodeJS.Platform = process.platform as NodeJS.Platform;
      if (lower.startsWith('latest-mac')) platform = 'darwin';
      else if (lower.startsWith('latest-linux')) platform = 'linux';
      else if (lower.startsWith('latest')) platform = 'win32';

      let arch: string = process.arch;
      if (/-arm64\.yml$/i.test(file)) arch = 'arm64';
      else if (/-ia32\.yml$/i.test(file)) arch = 'ia32';

      return { platform, arch } as const;
    };

    yield* pipe(
      yield* fs.readDirectory(dist),
      Array.filterMap(
        flow(
          Match.value,

          // Auto update meta (rename to our custom per-platform/arch scheme)
          Match.when(String.startsWith('latest'), (file) =>
            pipe(
              pickPlatformArchForLatest(file),
              ({ platform, arch }) =>
                Option.some(
                  repo.uploadReleaseAsset({
                    name: `latest-${platform}-${arch}.yml`,
                    path: path.join(dist, file),
                    releaseId,
                  }),
                ),
            ),
          ),

          // Build artifacts
          Match.when(String.includes(version), (file) =>
            Option.some(
              repo.uploadReleaseAsset({
                path: path.join(dist, file),
                releaseId,
              }),
            ),
          ),

          Match.orElse(() => Option.none()),
        ),
      ),
      Effect.all,
    );
  }, Effect.provide(Repository.Default)),
);

const uploadGoReleaseAssets = CliCommand.make(
  'upload-go-release-assets',
  {},
  Effect.fn(function* () {
    const path = yield* Path.Path;
    const fs = yield* FileSystem.FileSystem;
    const repo = yield* Repository;

    const tag = yield* repo.tag;
    const { id: releaseId } = yield* repo.getReleaseByTag(tag);
    const { name } = yield* repo.project;
    const { root: projectRoot } = yield* getProjectInfo(name);

    const dist = path.join(projectRoot, 'dist');

    yield* pipe(
      yield* fs.readDirectory(dist),
      Array.map((file) =>
        repo.uploadReleaseAsset({
          path: path.join(dist, file),
          releaseId,
        }),
      ),
      Effect.all,
    );
  }, Effect.provide(Repository.Default)),
);

pipe(
  CliCommand.make('gha-scripts'),
  CliCommand.withSubcommands([exportProjectInfo, release, uploadElectronReleaseAssets, uploadGoReleaseAssets]),
  CliCommand.run({ name: 'Scripts for GitHub Actions', version: 'internal' }),
  (_) => _(process.argv),
  Effect.provide(NodeContext.layer),
  NodeRuntime.runMain,
);
