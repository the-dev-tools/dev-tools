import { Cause, Effect } from 'effect';

export const makeUrl = (url: string | URL, base?: string | URL) =>
  Effect.try({
    // https://developer.mozilla.org/en-US/docs/Web/API/URL/URL#exceptions
    catch: (error) => new Cause.IllegalArgumentException((error as TypeError).message),
    try: () => new URL(url, base),
  });
