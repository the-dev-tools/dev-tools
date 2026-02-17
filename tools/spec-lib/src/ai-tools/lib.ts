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

export type ToolCategory = 'Execution' | 'Mutation';

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

function pascalToWords(name: string): string[] {
  return name.replace(/([a-z])([A-Z])/g, '$1 $2').split(' ');
}

export type CrudOperation = 'Delete' | 'Insert' | 'Update';

export interface IncludeFromModel {
  fields: string[];
  fromModel: string;
}

export interface MutationToolOptions {
  description?: string | undefined;
  exclude?: string[] | undefined;
  include?: IncludeFromModel[] | undefined;
  name?: string | undefined;
  operation: CrudOperation;
  parent?: string | undefined;
  title?: string | undefined;
}

export const mutationTools = makeStateMap<Model, MutationToolOptions[]>('mutationTools');

interface RawIncludeFromModel {
  fields: string[];
  fromModel: string;
}

interface RawMutationToolOptions {
  description?: string;
  exclude?: string[];
  include?: RawIncludeFromModel[];
  name?: string;
  operation: EnumValue;
  parent?: string;
  title?: string;
}

function mutationTool({ program }: DecoratorContext, target: Model, ...tools: RawMutationToolOptions[]) {
  const words = pascalToWords(target.name);
  const spacedName = words.join(' ');

  const resolved: MutationToolOptions[] = tools.map((tool) => {
    const operation = tool.operation.value.name as CrudOperation;
    return {
      description: tool.description,
      exclude: tool.exclude,
      include: tool.include,
      name: tool.name ?? `${operation}${target.name}`,
      operation,
      parent: tool.parent,
      title: tool.title ?? `${operation} ${spacedName}`,
    };
  });
  mutationTools(program).set(target, resolved);
}
