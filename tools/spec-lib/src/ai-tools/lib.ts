import { createTypeSpecLibrary, DecoratorContext, EnumValue, Model } from '@typespec/compiler';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/ai-tools',
});

export const $decorators = {
  'DevTools.AITools': {
    aiTool,
    mutationTool,
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

export type CrudOperation = 'Delete' | 'Insert' | 'Update';

export interface MutationToolOptions {
  description?: string | undefined;
  exclude?: string[] | undefined;
  name?: string | undefined;
  operation: CrudOperation;
  parent?: string | undefined;
  title: string;
}

export const mutationTools = makeStateMap<Model, MutationToolOptions[]>('mutationTools');

interface RawMutationToolOptions {
  description?: string;
  exclude?: string[];
  name?: string;
  operation: EnumValue;
  parent?: string;
  title: string;
}

function mutationTool({ program }: DecoratorContext, target: Model, ...tools: RawMutationToolOptions[]) {
  const resolved: MutationToolOptions[] = tools.map((tool) => ({
    description: tool.description,
    exclude: tool.exclude,
    name: tool.name,
    operation: tool.operation.value.name as CrudOperation,
    parent: tool.parent,
    title: tool.title,
  }));
  mutationTools(program).set(target, resolved);
}
