import { FileSystem, Path } from '@effect/platform';
import { createActionAuth } from '@octokit/auth-action';
import { Octokit } from '@octokit/rest';
import { Config, Console, Effect, Option, pipe, String, Tuple } from 'effect';

interface UploadReleaseAssetProps {
  id: number;
  path: string;
}

export class Repository extends Effect.Service<Repository>()('Repository', {
  effect: Effect.gen(function* () {
    const console = yield* Console.Console;

    const [owner, repo] = yield* pipe(
      yield* Config.string('GITHUB_REPOSITORY'),
      String.split('/'),
      Option.liftPredicate(Tuple.isTupleOf(2)),
    );

    const octokit = yield* Effect.try(
      () =>
        new Octokit({
          authStrategy: createActionAuth,
          log: {
            debug: () => undefined,
            info: (_) => void console.unsafe.info(_),
            warn: (_) => void console.unsafe.warn(_),
            error: (_) => void console.unsafe.error(_),
          },
        }),
    );

    const getReleaseByTag = Effect.fn(
      (tag: string) => Effect.tryPromise(() => octokit.rest.repos.getReleaseByTag({ owner, repo, tag })),
      Effect.map((_) => _.data),
    );

    const uploadReleaseAsset = Effect.fn(function* (_: UploadReleaseAssetProps) {
      const path = yield* Path.Path;
      const fs = yield* FileSystem.FileSystem;

      const name = path.basename(_.path);
      const data = yield* fs.readFileString(_.path);

      return yield* Effect.tryPromise(() =>
        octokit.rest.repos.uploadReleaseAsset({ owner, repo, release_id: _.id, name, data }),
      );
    }, Effect.asVoid);

    const tag = pipe(
      Config.literal('tag')('GITHUB_REF_TYPE'),
      Effect.flatMap(() => Config.string('GITHUB_REF_NAME')),
    );

    return { getReleaseByTag, uploadReleaseAsset, tag };
  }),
}) {}
