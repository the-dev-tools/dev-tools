import { createTypeSpecLibrary } from '@typespec/compiler';

export const $lib = createTypeSpecLibrary({
  name: 'meta',
  diagnostics: {},
  state: {
    normalKey: {},
  },
});
