import { createTypeSpecLibrary, JSONSchemaType } from '@typespec/compiler';
import { JSONSchema, pipe, Record, Schema } from 'effect';

export class EmitterOptions extends Schema.Class<EmitterOptions>('EmitterOptions')({
  goPackage: pipe(Schema.String, Schema.optionalWith({ as: 'Option' })),
  rootNamespace: pipe(Schema.String, Schema.optionalWith({ default: () => 'API' })),
  version: pipe(Schema.Positive, Schema.int(), Schema.optionalWith({ default: () => 1 })),
}) {}

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  emitter: { options: pipe(EmitterOptions.fields, Schema.Struct, JSONSchema.make) as JSONSchemaType<EmitterOptions> },
  name: 'spec-lib',
});

const stateKeys = ['externals', 'maps', 'streams'] as const;
export const state: Record<(typeof stateKeys)[number], symbol> = Record.fromIterableWith(stateKeys, (_) => [
  _,
  $lib.createStateSymbol(_),
]);
