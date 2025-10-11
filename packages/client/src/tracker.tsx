import ORTracker from '@openreplay/tracker';
import { Config, Data, Effect, identity, Option, pipe, Redacted } from 'effect';

export class TrackerStartError extends Data.TaggedError('TrackerStartError')<{ reason: string }> {}

interface TrackerReturn {
  tracker?: ORTracker;
}

export class Tracker extends Effect.Service<Tracker>()('Tracker', {
  effect: Effect.gen(function* () {
    const configNamespace = Config.nested('PUBLIC_OPEN_REPLAY');

    const track = yield* pipe(
      Config.boolean('TRACK'),
      configNamespace,
      Config.orElse(() => Config.succeed(false)),
    );
    if (!track) return identity<TrackerReturn>({});

    const projectKey = yield* pipe(Config.redacted('PROJECT_KEY'), configNamespace);
    const tracker = new ORTracker({
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
    if (!result.success) return yield* Effect.fail(new TrackerStartError({ reason: result.reason }));
    yield* Effect.logInfo('Tracking started', { ...result, sessionName, userId });

    yield* Effect.addFinalizer(() => Effect.sync(() => tracker.stop()));

    return identity<TrackerReturn>({ tracker });
  }),
}) {}
