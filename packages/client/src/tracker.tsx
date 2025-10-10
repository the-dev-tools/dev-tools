import Tracker from '@openreplay/tracker';
import { Config, Effect, Option, pipe, Redacted } from 'effect';

export const Track = Effect.gen(function* () {
  const configNamespace = Config.nested('PUBLIC_OPEN_RELAY');

  const track = yield* pipe(Config.boolean('TRACK'), configNamespace);
  if (!track) return;

  const projectKey = yield* pipe(Config.redacted('PROJECT_KEY'), configNamespace);
  const tracker = new Tracker({
    projectKey: Redacted.value(projectKey),

    __DISABLE_SECURE_MODE: true,
  });

  const sessionName = yield* pipe(Config.string('SESSION_NAME'), configNamespace, Config.option);
  Option.map(sessionName, (_) => void tracker.setMetadata('session-name', _));

  const userId = yield* pipe(Config.string('USER_ID'), configNamespace, Config.option);
  Option.map(userId, (_) => void tracker.setUserID(_));

  yield* Effect.logInfo('Tracking started', { sessionName, userId });

  yield* Effect.tryPromise(() => tracker.start());
}).pipe(Effect.ignore);
