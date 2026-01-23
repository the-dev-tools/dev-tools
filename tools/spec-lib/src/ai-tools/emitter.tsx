import { For, Indent, refkey, Show, SourceDirectory } from '@alloy-js/core';
import { SourceFile, VarDeclaration } from '@alloy-js/typescript';
import { EmitContext, getDoc, Model, ModelProperty, Program } from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { Output, useTsp, writeOutput } from '@typespec/emitter-framework';
import { Array, pipe, Record, String } from 'effect';
import { join } from 'node:path/posix';
import { primaryKeys } from '../core/index.jsx';
import { aiTools, AIToolOptions, MutationToolOptions, mutationTools, ToolCategory } from './lib.js';

export const $onEmit = async (context: EmitContext) => {
  const { emitterOutputDir, program } = context;

  if (program.compilerOptions.noEmit) return;

  // Check if there are any AI tools to emit
  const tools = aiTools(program);
  const mutations = mutationTools(program);
  if (tools.size === 0 && mutations.size === 0) {
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
  if (!visibilityDec) return true; // No restriction = visible everywhere

  // Check if any of the visibility args matches our phase
  return visibilityDec.args.some((arg) => {
    const val = arg.value as { value?: { name?: string } } | undefined;
    return val?.value?.name === phase;
  });
}

function resolveToolProperties(program: Program, collectionModel: Model, toolDef: MutationToolOptions): ResolvedProperty[] {
  const { exclude = [], operation, parent: parentName } = toolDef;

  // Resolve parent model by name from the collection model's namespace
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
        title: toolDef.title,
      });
    }
  }

  return tools;
}

const CategoryFiles = () => {
  const { program } = useTsp();

  // Group aiTool-decorated tools by category (for Exploration/Execution)
  const toolsByCategory = pipe(
    aiTools(program).entries(),
    Array.fromIterable,
    Array.groupBy(([, options]) => options.category),
    Record.map(Array.map(([model, options]) => ({ model, options }))),
  );

  // Resolve mutation tools from @mutationTool decorator
  const resolvedMutationTools = resolveMutationTools(program);

  const allCategories: ToolCategory[] = ['Mutation', 'Exploration', 'Execution'];

  return (
    <For each={allCategories}>
      {(category) => {
        if (category === 'Mutation') {
          // Mutation tools from @mutationTool decorator
          if (resolvedMutationTools.length === 0) return null;

          return (
            <SourceFile path="mutation.ts">
              <ResolvedSchemaImports tools={resolvedMutationTools} />
              {'\n'}
              <For doubleHardline each={resolvedMutationTools} ender>
                {(tool) => <ResolvedToolSchema tool={tool} />}
              </For>

              <VarDeclaration const export name="MutationSchemas" refkey={refkey('schemas', 'Mutation')}>
                {'{'}
                {'\n'}
                <Indent>
                  <For comma each={resolvedMutationTools} hardline>
                    {(tool) => <>{tool.name}</>}
                  </For>
                  ,
                </Indent>
                {'\n'}
                {'}'} as const
              </VarDeclaration>
              {'\n\n'}
              <For each={resolvedMutationTools}>
                {(tool) => (
                  <>
                    export type {tool.name} = typeof {tool.name}.Type;{'\n'}
                  </>
                )}
              </For>
            </SourceFile>
          );
        }

        // Exploration/Execution tools from @aiTool decorator
        const tools = toolsByCategory[category] ?? [];
        if (tools.length === 0) return null;

        const fileName = category.toLowerCase() + '.ts';
        const exportName = category + 'Schemas';

        return (
          <SourceFile path={fileName}>
            <SchemaImports tools={tools} />
            {'\n'}
            <For doubleHardline each={tools} ender>
              {({ model, options }) => <ToolSchema model={model} options={options} />}
            </For>

            <VarDeclaration const export name={exportName} refkey={refkey('schemas', category)}>
              {'{'}
              {'\n'}
              <Indent>
                <For comma each={tools} hardline>
                  {({ model }) => <>{model.name}</>}
                </For>
                ,
              </Indent>
              {'\n'}
              {'}'} as const
            </VarDeclaration>
            {'\n\n'}
            <For each={tools}>
              {({ model }) => (
                <>
                  export type {model.name} = typeof {model.name}.Type;{'\n'}
                </>
              )}
            </For>
          </SourceFile>
        );
      }}
    </For>
  );
};

interface SchemaImportsProps {
  tools: { model: Model; options: AIToolOptions }[];
}

const SchemaImports = ({ tools }: SchemaImportsProps) => {
  const { program } = useTsp();

  // Collect all imported field schemas from common.ts
  const commonImports = new Set<string>();

  tools.forEach(({ model }) => {
    model.properties.forEach((prop) => {
      const fieldSchema = getFieldSchema(prop, program);
      if (fieldSchema.importFrom === 'common') {
        commonImports.add(fieldSchema.schemaName);
      }
    });
  });

  const commonImportList = Array.sort(Array.fromIterable(commonImports), String.Order);

  return (
    <>
      import {'{'} Schema {'}'} from 'effect';
      {'\n\n'}
      <Show when={commonImportList.length > 0}>
        import {'{'}
        {'\n'}
        <Indent>
          <For comma each={commonImportList} hardline>
            {(name) => <>{name}</>}
          </For>
        </Indent>
        {'\n'}
        {'}'} from '../../../src/tools/common.ts';
        {'\n'}
      </Show>
    </>
  );
};

interface ResolvedSchemaImportsProps {
  tools: ResolvedTool[];
}

const ResolvedSchemaImports = ({ tools }: ResolvedSchemaImportsProps) => {
  const { program } = useTsp();

  const commonImports = new Set<string>();

  tools.forEach(({ properties }) => {
    properties.forEach(({ property }) => {
      const fieldSchema = getFieldSchema(property, program);
      if (fieldSchema.importFrom === 'common') {
        commonImports.add(fieldSchema.schemaName);
      }
    });
  });

  const commonImportList = Array.sort(Array.fromIterable(commonImports), String.Order);

  return (
    <>
      import {'{'} Schema {'}'} from 'effect';
      {'\n\n'}
      <Show when={commonImportList.length > 0}>
        import {'{'}
        {'\n'}
        <Indent>
          <For comma each={commonImportList} hardline>
            {(name) => <>{name}</>}
          </For>
        </Indent>
        {'\n'}
        {'}'} from '../../../src/tools/common.ts';
        {'\n'}
      </Show>
    </>
  );
};

const ResolvedToolSchema = ({ tool }: { tool: ResolvedTool }) => {
  const identifier = String.uncapitalize(tool.name);

  return (
    <VarDeclaration const export name={tool.name} refkey={refkey('tool', tool.name)}>
      Schema.Struct({'{'}
      {'\n'}
      <Indent>
        <For comma each={tool.properties} hardline>
          {({ optional, property }) => <ResolvedPropertySchema isOptional={optional} property={property} />}
        </For>
      </Indent>
      {'\n'}
      {'}'}).pipe(
      {'\n'}
      <Indent>
        Schema.annotations({'{'}
        {'\n'}
        <Indent>
          identifier: '{identifier}',{'\n'}
          <Show when={!!tool.title}>title: '{tool.title}',{'\n'}</Show>
          <Show when={!!tool.description}>description: {formatStringLiteral(tool.description ?? '')},{'\n'}</Show>
        </Indent>
        {'}'}),
      </Indent>
      {'\n'})
    </VarDeclaration>
  );
};

interface ResolvedPropertySchemaProps {
  isOptional: boolean;
  property: ModelProperty;
}

const ResolvedPropertySchema = ({ isOptional, property }: ResolvedPropertySchemaProps) => {
  const { program } = useTsp();
  const doc = getDoc(program, property);
  const fieldSchema = getFieldSchema(property, program);

  const schemaExpr = isOptional && !fieldSchema.includesOptional
    ? `Schema.optional(${fieldSchema.expression})`
    : fieldSchema.expression;

  if (doc || fieldSchema.needsDescription) {
    const description = doc ?? '';
    return (
      <>
        {property.name}: {schemaExpr}.pipe(
        {'\n'}
        <Indent>
          Schema.annotations({'{'}
          {'\n'}
          <Indent>description: {formatStringLiteral(description)},{'\n'}</Indent>
          {'}'}),
        </Indent>
        {'\n'})
      </>
    );
  }

  return (
    <>
      {property.name}: {schemaExpr}
    </>
  );
};

interface ToolSchemaProps {
  model: Model;
  options: AIToolOptions;
}

const ToolSchema = ({ model, options }: ToolSchemaProps) => {
  const { program } = useTsp();
  const doc = getDoc(program, model);
  const identifier = String.uncapitalize(model.name);

  const properties = model.properties.values().toArray();

  return (
    <VarDeclaration const export name={model.name} refkey={refkey('tool', model)}>
      Schema.Struct({'{'}
      {'\n'}
      <Indent>
        <For comma each={properties} hardline>
          {(prop) => <PropertySchema property={prop} />}
        </For>
      </Indent>
      {'\n'}
      {'}'}).pipe(
      {'\n'}
      <Indent>
        Schema.annotations({'{'}
        {'\n'}
        <Indent>
          identifier: '{identifier}',{'\n'}
          <Show when={!!options.title}>title: '{options.title}',{'\n'}</Show>
          <Show when={!!doc}>description: {formatStringLiteral(doc ?? '')},{'\n'}</Show>
        </Indent>
        {'}'}),
      </Indent>
      {'\n'})
    </VarDeclaration>
  );
};

interface PropertySchemaProps {
  property: ModelProperty;
}

const PropertySchema = ({ property }: PropertySchemaProps) => {
  const { program } = useTsp();
  const doc = getDoc(program, property);
  const fieldSchema = getFieldSchema(property, program);

  // Handle optional wrapping - but not for schemas that already include optional
  const schemaExpr = property.optional && !fieldSchema.includesOptional
    ? `Schema.optional(${fieldSchema.expression})`
    : fieldSchema.expression;

  // Add description annotation if doc exists
  if (doc || fieldSchema.needsDescription) {
    const description = doc ?? '';
    return (
      <>
        {property.name}: {schemaExpr}.pipe(
        {'\n'}
        <Indent>
          Schema.annotations({'{'}
          {'\n'}
          <Indent>description: {formatStringLiteral(description)},{'\n'}</Indent>
          {'}'}),
        </Indent>
        {'\n'})
      </>
    );
  }

  return (
    <>
      {property.name}: {schemaExpr}
    </>
  );
};

interface FieldSchemaResult {
  expression: string;
  importFrom: 'common' | 'effect' | 'none';
  includesOptional: boolean;
  needsDescription: boolean;
  schemaName: string;
}

function getFieldSchema(property: ModelProperty, program: ReturnType<typeof useTsp>['program']): FieldSchemaResult {
  const { name, type } = property;

  // Check for known field names that map to common.ts schemas
  const knownFieldSchemas: Record<string, string> = {
    code: 'JsCode',
    condition: 'ConditionExpression',
    edgeId: 'EdgeId',
    errorHandling: 'ErrorHandling',
    flowId: 'FlowId',
    flowVariableId: 'UlidId',
    httpId: 'UlidId',
    nodeId: 'NodeId',
    position: 'OptionalPosition',
    sourceHandle: 'SourceHandle',
    sourceId: 'NodeId',
    targetId: 'NodeId',
  };

  // Position field is special - it uses OptionalPosition from common when optional
  if (name === 'position') {
    if (property.optional) {
      return {
        expression: 'OptionalPosition',
        importFrom: 'common',
        includesOptional: true,
        needsDescription: false,
        schemaName: 'OptionalPosition',
      };
    }
    return {
      expression: 'Position',
      importFrom: 'common',
      includesOptional: false,
      needsDescription: false,
      schemaName: 'Position',
    };
  }

  // Name field uses NodeName
  if (name === 'name') {
    return {
      expression: 'NodeName',
      importFrom: 'common',
      includesOptional: false,
      needsDescription: false,
      schemaName: 'NodeName',
    };
  }

  // Check if it's a known field
  const knownSchema = knownFieldSchemas[name];
  if (knownSchema) {
    return {
      expression: knownSchema,
      importFrom: 'common',
      includesOptional: false,
      needsDescription: false,
      schemaName: knownSchema,
    };
  }

  // Check the actual type
  if ($(program).scalar.is(type)) {
    const scalarName = type.name;

    // bytes type → UlidId
    if (scalarName === 'bytes') {
      return {
        expression: 'UlidId',
        importFrom: 'common',
        includesOptional: false,
        needsDescription: true,
        schemaName: 'UlidId',
      };
    }

    // string type
    if (scalarName === 'string') {
      return {
        expression: 'Schema.String',
        importFrom: 'effect',
        includesOptional: false,
        needsDescription: true,
        schemaName: 'Schema.String',
      };
    }

    // int32 type
    if (scalarName === 'int32') {
      return {
        expression: 'Schema.Number.pipe(Schema.int())',
        importFrom: 'effect',
        includesOptional: false,
        needsDescription: true,
        schemaName: 'Schema.Number',
      };
    }

    // float32 type
    if (scalarName === 'float32') {
      return {
        expression: 'Schema.Number',
        importFrom: 'effect',
        includesOptional: false,
        needsDescription: true,
        schemaName: 'Schema.Number',
      };
    }

    // boolean type
    if (scalarName === 'boolean') {
      return {
        expression: 'Schema.Boolean',
        importFrom: 'effect',
        includesOptional: false,
        needsDescription: true,
        schemaName: 'Schema.Boolean',
      };
    }
  }

  // Check for enum types
  if ($(program).enum.is(type)) {
    const enumName = type.name;
    // Map known enum names to common.ts schemas
    if (enumName === 'ErrorHandling') {
      return {
        expression: 'ErrorHandling',
        importFrom: 'common',
        includesOptional: false,
        needsDescription: false,
        schemaName: 'ErrorHandling',
      };
    }
    if (enumName === 'HandleKind') {
      return {
        expression: 'SourceHandle',
        importFrom: 'common',
        includesOptional: false,
        needsDescription: false,
        schemaName: 'SourceHandle',
      };
    }
  }

  // Default to Schema.String for unknown types
  return {
    expression: 'Schema.String',
    importFrom: 'effect',
    includesOptional: false,
    needsDescription: true,
    schemaName: 'Schema.String',
  };
}

function formatStringLiteral(str: string): string {
  // Check if we need multi-line formatting
  if (str.length > 80 || str.includes('\n')) {
    return '`' + str.replace(/`/g, '\\`').replace(/\$/g, '\\$') + '`';
  }
  // Use single quotes for short strings
  return "'" + str.replace(/'/g, "\\'") + "'";
}
