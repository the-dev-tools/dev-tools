import { FileSystem, Path } from '@effect/platform';
import { createActionAuth } from '@octokit/auth-action';
import { Octokit as OctokitBase } from '@octokit/rest';
import { Config, Console, DefaultServices, Effect, flow, Layer, Option, pipe, String, Tuple } from 'effect';

interface DispatchWorkflowProps {
  inputs?: unknown;
  ref: string;
  workflow: string;
}

interface UploadReleaseAssetProps {
  name?: string;
  path: string;
  releaseId: number;
}

class Octokit extends Effect.Service<Octokit>()('Octokit', {
  dependencies: [Layer.succeedContext(DefaultServices.liveServices)],
  effect: Effect.gen(function* () {
    const console = yield* Console.Console;
    return yield* Effect.try(
      () =>
        new OctokitBase({
          authStrategy: createActionAuth,
          log: {
            debug: () => undefined,
            error: (_) => void console.unsafe.error(_),
            info: (_) => void console.unsafe.info(_),
            warn: (_) => void console.unsafe.warn(_),
          },
        }),
    );
  }),
}) {}

export class Repository extends Effect.Service<Repository>()('Repository', {
  effect: Effect.gen(function* () {
    const [owner, repo] = yield* pipe(
      yield* Config.string('GITHUB_REPOSITORY'),
      String.split('/'),
      Option.liftPredicate(Tuple.isTupleOf(2)),
    );

    const dispatchWorkflow = Effect.fn(
      function* (_: DispatchWorkflowProps) {
        const octokit = yield* Octokit;
        return yield* Effect.tryPromise(() =>
          octokit.rest.actions.createWorkflowDispatch({ owner, ref: _.ref, repo, workflow_id: _.workflow }),
        );
      },
      Effect.provide(Octokit.Default),
      Effect.asVoid,
    );

    const getReleaseByTag = Effect.fn(
      function* (tag: string) {
        const octokit = yield* Octokit;
        return yield* Effect.tryPromise(() => octokit.rest.repos.getReleaseByTag({ owner, repo, tag }));
      },
      Effect.provide(Octokit.Default),
      Effect.map((_) => _.data),
    );

    const uploadReleaseAsset = Effect.fn(
      function* (_: UploadReleaseAssetProps) {
        const path = yield* Path.Path;
        const fs = yield* FileSystem.FileSystem;
        const octokit = yield* Octokit;

        const name = _.name ?? path.basename(_.path);
        const data: unknown = yield* fs.readFile(_.path);

        return yield* Effect.tryPromise(() =>
          octokit.rest.repos.uploadReleaseAsset({ data: data as string, name, owner, release_id: _.releaseId, repo }),
        );
      },
      Effect.provide(Octokit.Default),
      Effect.asVoid,
    );

    const tag = pipe(
      Config.literal('tag')('GITHUB_REF_TYPE'),
      Effect.flatMap(() => Config.string('GITHUB_REF_NAME')),
    );

    const project = Effect.flatMap(
      tag,
      flow(
        String.split('@'),
        Option.liftPredicate(Tuple.isTupleOf(2)),
        Option.map(([name, version]) => ({ name, version })),
      ),
    );

    return { dispatchWorkflow, getReleaseByTag, project, tag, uploadReleaseAsset };
  }),
}) {}
