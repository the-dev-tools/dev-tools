import { Config, Effect, pipe } from 'effect';
import { Ulid } from 'id128';

interface Umami {
  identify(uniqueId: string, data?: object): void;
  identify(data: object): void;

  track(payload?: object): void;
  track(event: string, data?: object): void;
}

export const initUmami = Effect.gen(function* () {
  const configNamespace = Config.nested('PUBLIC_UMAMI');

  const enable = yield* pipe(
    Config.boolean('ENABLE'),
    configNamespace,
    Config.orElse(() => Config.succeed(false)),
  );
  if (!enable) return;

  const host = yield* pipe(Config.string('HOST'), configNamespace);
  const websiteId = yield* pipe(Config.string('ID'), configNamespace);

  const umami = yield* Effect.async<Umami>((resume) => {
    const script = document.createElement('script');
    script.src = `${host}/script.js`;
    script.setAttribute('data-website-id', websiteId);
    script.setAttribute('data-auto-track', 'false');
    document.head.appendChild(script);

    script.addEventListener('load', () => {
      const { umami } = window as unknown as { umami: Umami };
      resume(Effect.succeed(umami));
    });
  });

  const sessionIdKey = 'UMAMI_SESSION_ID';
  let sessionId = localStorage.getItem(sessionIdKey);
  if (!sessionId) {
    sessionId = Ulid.generate().toCanonical();
    localStorage.setItem(sessionIdKey, sessionId);
  }

  umami.identify(sessionId);
  umami.track('init');
});
