import { createTypeSpecLibrary, DecoratorContext, EnumValue, Model } from '@typespec/compiler';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/ai-tools',
});

export const $decorators = {
  'DevTools.AITools': {
    aiTool,
  },
};

const { makeStateMap } = makeStateFactory((_) => $lib.createStateSymbol(_));

export type ToolCategory = 'Execution' | 'Exploration' | 'Mutation';

export interface AIToolOptions {
  category: ToolCategory;
  title?: string | undefined;
}

export const aiTools = makeStateMap<Model, AIToolOptions>('aiTools');

interface RawAIToolOptions {
  category: EnumValue;
  title?: string;
}

function aiTool({ program }: DecoratorContext, target: Model, options: RawAIToolOptions) {
  // Extract category name from EnumValue
  const category = options.category.value.name as ToolCategory;
  aiTools(program).set(target, {
    category,
    title: options.title,
  });
}
