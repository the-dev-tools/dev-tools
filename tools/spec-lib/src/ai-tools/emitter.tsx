import { code, For, Indent, refkey, Show, SourceDirectory } from '@alloy-js/core';
import { SourceFile, VarDeclaration } from '@alloy-js/typescript';
import { EmitContext, getDoc, Model, ModelProperty, Program } from '@typespec/compiler';
import { Output, useTsp, writeOutput } from '@typespec/emitter-framework';
import { Array, String } from 'effect';
import { join } from 'node:path/posix';
import { primaryKeys } from '../core/index.jsx';
import { formatStringLiteral, getFieldSchema } from './field-schema.js';
import { aiTools, explorationTools, MutationToolOptions, mutationTools, ToolCategory } from './lib.js';

export const $onEmit = async (context: EmitContext) => {
  const { emitterOutputDir, program } = context;

  if (program.compilerOptions.noEmit) return;

  const tools = aiTools(program);
  const mutations = mutationTools(program);
  const explorations = explorationTools(program);
  if (tools.size === 0 && mutations.size === 0 && explorations.size === 0) {
    return;
  }

  await writeOutput(
    program,
    <Output printWidth={120} program={program}>
      <SourceDirectory path="v1">
        <CategoryFiles />
      </SourceDirectory>
    </Output>,
    join(emitterOutputDir, 'ai-tools'),
  );
};

interface ResolvedProperty {
  optional: boolean;
  property: ModelProperty;
}

interface ResolvedTool {
  description?: string | undefined;
  name: string;
  properties: ResolvedProperty[];
  title: string;
}

function isVisibleFor(property: ModelProperty, phase: 'Create' | 'Update'): boolean {
  const visibilityDec = property.decorators.find(
    (d) => d.decorator.name === '$visibility',
  );
  if (!visibilityDec) return true;

  return visibilityDec.args.some((arg) => {
    const val = arg.value as { value?: { name?: string } } | undefined;
    return val?.value?.name === phase;
  });
}

function resolveToolProperties(program: Program, collectionModel: Model, toolDef: MutationToolOptions): ResolvedProperty[] {
  const { exclude = [], operation, parent: parentName } = toolDef;
  const parent = parentName ? collectionModel.namespace?.models.get(parentName) : undefined;

  switch (operation) {
    case 'Insert': {
      const props: ResolvedProperty[] = [];
      if (parent) {
        for (const prop of parent.properties.values()) {
          if (primaryKeys(program).has(prop)) continue;
          if (!isVisibleFor(prop, 'Create')) continue;
          if (exclude.includes(prop.name)) continue;
          props.push({ optional: prop.optional, property: prop });
        }
      }
      for (const prop of collectionModel.properties.values()) {
        if (primaryKeys(program).has(prop)) continue;
        if (!isVisibleFor(prop, 'Create')) continue;
        if (exclude.includes(prop.name)) continue;
        props.push({ optional: prop.optional, property: prop });
      }
      return props;
    }
    case 'Update': {
      const props: ResolvedProperty[] = [];
      for (const prop of collectionModel.properties.values()) {
        if (!isVisibleFor(prop, 'Update')) continue;
        if (primaryKeys(program).has(prop)) {
          props.push({ optional: false, property: prop });
        } else {
          if (exclude.includes(prop.name)) continue;
          props.push({ optional: true, property: prop });
        }
      }
      return props;
    }
    case 'Delete': {
      const props: ResolvedProperty[] = [];
      for (const prop of collectionModel.properties.values()) {
        if (primaryKeys(program).has(prop)) {
          props.push({ optional: false, property: prop });
        }
      }
      return props;
    }
  }
}

function resolveExplorationTools(program: Program): ResolvedTool[] {
  const tools: ResolvedTool[] = [];

  for (const [model, toolDefs] of explorationTools(program).entries()) {
    for (const toolDef of toolDefs) {
      const properties: ResolvedProperty[] = [];
      for (const prop of model.properties.values()) {
        if (primaryKeys(program).has(prop)) {
          properties.push({ optional: false, property: prop });
        }
      }
      if (properties.length === 0) continue;

      tools.push({
        description: toolDef.description,
        name: toolDef.name!,
        properties,
        title: toolDef.title!,
      });
    }
  }

  return tools;
}

function resolveMutationTools(program: Program): ResolvedTool[] {
  const tools: ResolvedTool[] = [];

  for (const [model, toolDefs] of mutationTools(program).entries()) {
    for (const toolDef of toolDefs) {
      const name = toolDef.name ?? `${toolDef.operation}${model.name}`;
      const properties = resolveToolProperties(program, model, toolDef);
      tools.push({
        description: toolDef.description,
        name,
        properties,
        title: toolDef.title!,
      });
    }
  }

  return tools;
}

function resolveAiTools(program: Program): Partial<Record<ToolCategory, ResolvedTool[]>> {
  const result: Partial<Record<ToolCategory, ResolvedTool[]>> = {};

  for (const [model, options] of aiTools(program).entries()) {
    const properties: ResolvedProperty[] = [];
    for (const prop of model.properties.values()) {
      properties.push({ optional: prop.optional, property: prop });
    }
    const category = options.category;
    if (!result[category]) result[category] = [];
    result[category]!.push({
      description: getDoc(program, model),
      name: model.name,
      properties,
      title: options.title ?? model.name,
    });
  }

  return result;
}

const CategoryFiles = () => {
  const { program } = useTsp();

  const resolvedMutationTools = resolveMutationTools(program);
  const resolvedExplorationTools = resolveExplorationTools(program);
  const aiToolsByCategory = resolveAiTools(program);

  const categories: { category: ToolCategory; tools: ResolvedTool[] }[] = [];

  if (resolvedMutationTools.length > 0) {
    categories.push({ category: 'Mutation', tools: resolvedMutationTools });
  }

  const allExploration = [...resolvedExplorationTools, ...(aiToolsByCategory['Exploration'] ?? [])];
  if (allExploration.length > 0) {
    categories.push({ category: 'Exploration', tools: allExploration });
  }

  const executionTools = aiToolsByCategory['Execution'] ?? [];
  if (executionTools.length > 0) {
    categories.push({ category: 'Execution', tools: executionTools });
  }

  return (
    <For each={categories}>
      {({ category, tools }) => (
        <SourceFile path={category.toLowerCase() + '.ts'}>
          <SchemaImports tools={tools} />
          <hbr />
          <For doubleHardline each={tools} ender>
            {(tool) => <ToolSchema tool={tool} />}
          </For>

          <VarDeclaration const export name={category + 'Schemas'} refkey={refkey('schemas', category)}>
            {'{'}
            <hbr />
            <Indent>
              <For comma each={tools} hardline>
                {(tool) => <>{tool.name}</>}
              </For>
              ,
            </Indent>
            <hbr />
            {'}'} as const
          </VarDeclaration>
          <hbr /><hbr />
          <For each={tools}>
            {(tool) => (
              <>
                export type {tool.name} = typeof {tool.name}.Type;<hbr />
              </>
            )}
          </For>
        </SourceFile>
      )}
    </For>
  );
};

const SchemaImports = ({ tools }: { tools: ResolvedTool[] }) => {
  const { program } = useTsp();
  const commonImports = new Set<string>();

  for (const { properties } of tools) {
    for (const { property } of properties) {
      const fieldSchema = getFieldSchema(property, program);
      if (fieldSchema.importFrom === 'common') {
        commonImports.add(fieldSchema.schemaName);
      }
    }
  }

  const commonImportList = Array.sort(Array.fromIterable(commonImports), String.Order);

  return (
    <>
      {code`import { Schema } from 'effect';`}
      <hbr /><hbr />
      <Show when={commonImportList.length > 0}>
        import {'{'}
        <hbr />
        <Indent>
          <For comma each={commonImportList} hardline>
            {(name) => <>{name}</>}
          </For>
        </Indent>
        <hbr />
        {'}'} from '@the-dev-tools/spec-lib/common';
        <hbr />
      </Show>
    </>
  );
};

const ToolSchema = ({ tool }: { tool: ResolvedTool }) => {
  const identifier = String.uncapitalize(tool.name);

  return (
    <VarDeclaration const export name={tool.name} refkey={refkey('tool', tool.name)}>
      Schema.Struct({'{'}
      <hbr />
      <Indent>
        <For comma each={tool.properties} hardline>
          {({ optional, property }) => <PropertySchema isOptional={optional} property={property} />}
        </For>
      </Indent>
      <hbr />
      {'}'}).pipe(
      <hbr />
      <Indent>
        Schema.annotations({'{'}
        <hbr />
        <Indent>
          identifier: '{identifier}',<hbr />
          <Show when={!!tool.title}>title: '{tool.title}',<hbr /></Show>
          <Show when={!!tool.description}>description: {formatStringLiteral(tool.description ?? '')},<hbr /></Show>
        </Indent>
        {'}'}),
      </Indent>
      <hbr />)
    </VarDeclaration>
  );
};

interface PropertySchemaProps {
  isOptional: boolean;
  property: ModelProperty;
}

const PropertySchema = ({ isOptional, property }: PropertySchemaProps) => {
  const { program } = useTsp();
  const doc = getDoc(program, property);
  const fieldSchema = getFieldSchema(property, program);

  const needsOptionalWrapper = isOptional && !fieldSchema.includesOptional;

  if (doc || fieldSchema.needsDescription) {
    const description = doc ?? '';
    // When optional, wrap the annotated inner schema with Schema.optional()
    // Schema.optional() returns a PropertySignature that can't be piped
    const annotatedInner = (
      <>
        {fieldSchema.expression}.pipe(
        <hbr />
        <Indent>
          Schema.annotations({'{'}
          <hbr />
          <Indent>description: {formatStringLiteral(description)},<hbr /></Indent>
          {'}'}),
        </Indent>
        <hbr />)
      </>
    );

    if (needsOptionalWrapper) {
      return (
        <>
          {property.name}: Schema.optional({annotatedInner})
        </>
      );
    }

    return (
      <>
        {property.name}: {annotatedInner}
      </>
    );
  }

  const schemaExpr = needsOptionalWrapper
    ? `Schema.optional(${fieldSchema.expression})`
    : fieldSchema.expression;

  return (
    <>
      {property.name}: {schemaExpr}
    </>
  );
};
