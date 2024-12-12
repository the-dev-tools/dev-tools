import { Config, Effect, Layer, pipe } from 'effect';

import { ApiLive } from './live';
import { ApiMock } from './mock';

export const ApiLayer = Effect.gen(function* () {
  const mock = yield* pipe(Config.boolean('PUBLIC_MOCK'), Config.withDefault(false));
  if (mock) return ApiMock;
  return ApiLive;
}).pipe(Layer.unwrapEffect);
