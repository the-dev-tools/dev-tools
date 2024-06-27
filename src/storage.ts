import { Schema } from '@effect/schema';
import { pipe } from 'effect';

import * as PlasmoStorage from '@plasmohq/storage';

export const Local = new PlasmoStorage.Storage({ area: 'local' });

export const RECORDING_TAB_ID = 'RECORDING_TAB_ID';

export const RECORDED_CALLS = 'RECORDED_CALLS';

export interface NetworkCall {
  method: string;
  url: string;
  time: number;
}

export const Change = <S extends Schema.Schema.All>(schema: S) => {
  const value = pipe(schema, Schema.optional({ as: 'Option' }));
  return Schema.Struct({ newValue: value, oldValue: value });
};
