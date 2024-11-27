import { Config, pipe } from 'effect';

export const LocalMode = pipe(Config.boolean('LOCAL_MODE'), Config.withDefault(false));
