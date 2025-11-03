import OpenReplayTracker from '@openreplay/tracker';
import { Config, Data, Effect, Option, pipe, Redacted } from 'effect';

export class StartOpenReplayError extends Data.TaggedError('StartOpenReplayError')<{ reason: string }> {}

export const startOpenReplay = Effect.gen(function* () {
  const configNamespace = Config.nested('PUBLIC_OPEN_REPLAY');

  const track = yield* pipe(
    Config.boolean('TRACK'),
    configNamespace,
    Config.orElse(() => Config.succeed(false)),
  );
  if (!track) return;

  const projectKey = yield* pipe(Config.redacted('PROJECT_KEY'), configNamespace);
  const tracker = new OpenReplayTracker({
    projectKey: Redacted.value(projectKey),

    __DISABLE_SECURE_MODE: true,

    network: {
      captureInIframes: true,
      capturePayload: true,
      failuresOnly: false,
      ignoreHeaders: false,
      sessionTokenHeader: false,
    },
  });

  const sessionName = yield* pipe(Config.string('SESSION_NAME'), configNamespace, Config.option);
  Option.map(sessionName, (_) => void tracker.setMetadata('session-name', _));

  const userId = yield* pipe(Config.string('USER_ID'), configNamespace, Config.option);
  Option.map(userId, (_) => void tracker.setUserID(_));

  const result = yield* Effect.promise(() => tracker.start());
  if (!result.success) return yield* Effect.fail(new StartOpenReplayError({ reason: result.reason }));
  yield* Effect.logInfo('Tracking started', { ...result, sessionName, userId });

  yield* Effect.addFinalizer(() => Effect.sync(() => tracker.stop()));
});
