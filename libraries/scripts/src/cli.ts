import { Args, Command } from '@effect/cli';
import { FileSystem, Path } from '@effect/platform';
import { NodeContext, NodeRuntime } from '@effect/platform-node';
import { Boolean, Cause, DefaultServices, Effect, pipe } from 'effect';

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

const uploadReleaseAssets = Command.make(
  'upload-release-assets',
  { files: Args.atLeast(Args.file(), 1) },
  Effect.fn(
    function* ({ files }) {
      const repo = yield* Repository;
      const path = yield* Path.Path;
      const root = yield* resolveMonorepoRoot;

      const tag = yield* repo.tag;
      const { id } = yield* repo.getReleaseByTag(tag);

      yield* pipe(
        files.map((_) => repo.uploadReleaseAsset({ id, path: path.resolve(root, _) })),
        Effect.all,
      );
    },
    Effect.provide(Repository.Default),
    Effect.provide(DefaultServices.liveServices),
  ),
);

pipe(
  Command.make('scripts'),
  Command.withSubcommands([uploadReleaseAssets]),
  Command.run({ name: 'Internal scripts', version: '' }),
  (_) => _(process.argv),
  Effect.provide(NodeContext.layer),
  NodeRuntime.runMain,
);
