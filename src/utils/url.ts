import { Cause, Effect } from 'effect';

export const make = (url: string | URL, base?: string | URL) =>
  Effect.try({
    try: () => new URL(url, base),
    // https://developer.mozilla.org/en-US/docs/Web/API/URL/URL#exceptions
    catch: (error) => new Cause.IllegalArgumentException((error as TypeError).message),
  });
