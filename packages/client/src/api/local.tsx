import { Config, pipe } from 'effect';

export const LocalMode = pipe(Config.boolean('PUBLIC_LOCAL_MODE'), Config.withDefault(false));
