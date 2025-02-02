import { Args, Command } from '@effect/cli';
import { NodeContext, NodeRuntime } from '@effect/platform-node';
import { DefaultServices, Effect, pipe } from 'effect';

import { Repository } from './repository.ts';

const uploadReleaseAssets = Command.make(
  'upload-release-assets',
  { files: Args.atLeast(Args.file(), 1) },
  Effect.fn(
    function* ({ files }) {
      const repo = yield* Repository;

      const tag = yield* repo.tag;
      const { id } = yield* repo.getReleaseByTag(tag);

      yield* pipe(
        files.map((_) => repo.uploadReleaseAsset({ id, path: _ })),
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
