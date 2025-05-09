import { createTypeSpecLibrary } from '@typespec/compiler';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: 'meta',
  state: {
    autoChanges: {},
    base: {},
    endpoint: {},
    entity: {},
    move: {},
    normalKeys: {},
  },
});
