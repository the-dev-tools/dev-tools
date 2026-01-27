import { For, Indent, refkey, Show, SourceDirectory } from '@alloy-js/core';
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
        <CommonSchemaFile />
        <CategoryFiles />
        <IndexFile />
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
        name: toolDef.name,
        properties,
        title: toolDef.title,
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
        title: toolDef.title,
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
          {'\n'}
          <For doubleHardline each={tools} ender>
            {(tool) => <ToolSchema tool={tool} />}
          </For>

          <VarDeclaration const export name={category + 'Schemas'} refkey={refkey('schemas', category)}>
            {'{'}
            {'\n'}
            <Indent>
              <For comma each={tools} hardline>
                {(tool) => <>{tool.name}</>}
              </For>
              ,
            </Indent>
            {'\n'}
            {'}'} as const
          </VarDeclaration>
          {'\n\n'}
          <For each={tools}>
            {(tool) => (
              <>
                export type {tool.name} = typeof {tool.name}.Type;{'\n'}
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
        {'}'} from './common.ts';
        {'\n'}
      </Show>
    </>
  );
};

const ToolSchema = ({ tool }: { tool: ResolvedTool }) => {
  const identifier = String.uncapitalize(tool.name);

  return (
    <VarDeclaration const export name={tool.name} refkey={refkey('tool', tool.name)}>
      Schema.Struct({'{'}
      {'\n'}
      <Indent>
        <For comma each={tool.properties} hardline>
          {({ optional, property }) => <PropertySchema isOptional={optional} property={property} />}
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

// =============================================================================
// Generated common.ts — inline schema building blocks
// =============================================================================

const CommonSchemaFile = () => {
  return (
    <SourceFile path="common.ts">
      {`/**
 * AUTO-GENERATED FILE - DO NOT EDIT
 * Common schemas and utilities for tool definitions.
 * Generated by the TypeSpec emitter.
 */

import { Schema } from 'effect';

import {
  ErrorHandling as PbErrorHandling,
  HandleKind as PbHandleKind,
} from '../../buf/typescript/api/flow/v1/flow_pb.ts';

// =============================================================================
// Common Field Schemas
// =============================================================================

/**
 * ULID identifier schema - used for all entity IDs
 */
export const UlidId = Schema.String.pipe(
  Schema.pattern(/^[0-9A-HJKMNP-TV-Z]{26}$/),
  Schema.annotations({
    title: 'ULID',
    description: 'A ULID (Universally Unique Lexicographically Sortable Identifier)',
    examples: ['01ARZ3NDEKTSV4RRFFQ69G5FAV'],
  }),
);

/**
 * Flow ID - references a workflow
 */
export const FlowId = UlidId.pipe(
  Schema.annotations({
    identifier: 'flowId',
    description: 'The ULID of the workflow',
  }),
);

/**
 * Node ID - references a node within a workflow
 */
export const NodeId = UlidId.pipe(
  Schema.annotations({
    identifier: 'nodeId',
    description: 'The ULID of the node',
  }),
);

/**
 * Edge ID - references an edge connection
 */
export const EdgeId = UlidId.pipe(
  Schema.annotations({
    identifier: 'edgeId',
    description: 'The ULID of the edge',
  }),
);

// =============================================================================
// Position Schema
// =============================================================================

export const Position = Schema.Struct({
  x: Schema.Number.pipe(
    Schema.annotations({
      description: 'X coordinate on the canvas',
    }),
  ),
  y: Schema.Number.pipe(
    Schema.annotations({
      description: 'Y coordinate on the canvas',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'Position',
    description: 'Position on the canvas',
  }),
);

export const OptionalPosition = Schema.optional(
  Position.pipe(
    Schema.annotations({
      description: 'Position on the canvas (optional)',
    }),
  ),
);

// =============================================================================
// Enums DERIVED from TypeSpec/Protobuf definitions
// =============================================================================

type ValidHandleKind = Exclude<PbHandleKind, PbHandleKind.UNSPECIFIED>;
type ValidErrorHandling = Exclude<PbErrorHandling, PbErrorHandling.UNSPECIFIED>;

function literalFromValues<T extends Record<number, string>>(mapping: T) {
  const values = Object.values(mapping) as [string, ...string[]];
  return Schema.Literal(...values);
}

const errorHandlingValues: Record<ValidErrorHandling, string> = {
  [PbErrorHandling.IGNORE]: 'ignore',
  [PbErrorHandling.BREAK]: 'break',
};

export const ErrorHandling = literalFromValues(errorHandlingValues).pipe(
  Schema.annotations({
    identifier: 'ErrorHandling',
    description: 'How to handle errors: "ignore" continues, "break" stops the loop',
  }),
);

const handleKindValues: Record<ValidHandleKind, string> = {
  [PbHandleKind.THEN]: 'then',
  [PbHandleKind.ELSE]: 'else',
  [PbHandleKind.LOOP]: 'loop',
};

export const SourceHandle = literalFromValues(handleKindValues).pipe(
  Schema.annotations({
    identifier: 'SourceHandle',
    description:
      'Output handle for branching nodes. Use "then"/"else" for Condition nodes, "loop"/"then" for For/ForEach nodes.',
  }),
);

export const ApiCategory = Schema.Literal(
  'messaging',
  'payments',
  'project-management',
  'storage',
  'database',
  'email',
  'calendar',
  'crm',
  'social',
  'analytics',
  'developer',
).pipe(
  Schema.annotations({
    identifier: 'ApiCategory',
    description: 'Category of the API',
  }),
);

// =============================================================================
// Display Name & Code Schemas
// =============================================================================

export const NodeName = Schema.String.pipe(
  Schema.minLength(1),
  Schema.maxLength(100),
  Schema.annotations({
    description: 'Display name for the node',
    examples: ['Transform Data', 'Fetch User', 'Check Status'],
  }),
);

export const JsCode = Schema.String.pipe(
  Schema.annotations({
    description:
      'The function body only. Write code directly - do NOT define inner functions. Use ctx for input. MUST have a return statement. The tool auto-wraps with "export default function(ctx) { ... }". Example: "const result = ctx.value * 2; return { result };"',
    examples: [
      'const result = ctx.value * 2; return { result };',
      'const items = ctx.data.filter(x => x.active); return { items, count: items.length };',
    ],
  }),
);

export const ConditionExpression = Schema.String.pipe(
  Schema.annotations({
    description:
      'Boolean expression using expr-lang syntax. Use == for equality (NOT ===). Use Input to reference previous node output (e.g., "Input.status == 200", "Input.success == true")',
    examples: ['Input.status == 200', 'Input.success == true', 'Input.count > 0'],
  }),
);

// =============================================================================
// Type Exports
// =============================================================================

export type Position = typeof Position.Type;
export type ErrorHandling = typeof ErrorHandling.Type;
export type SourceHandle = typeof SourceHandle.Type;
export type ApiCategory = typeof ApiCategory.Type;
`}
    </SourceFile>
  );
};

// =============================================================================
// Generated index.ts — runtime JSON Schema conversion
// =============================================================================

const IndexFile = () => {
  return (
    <SourceFile path="index.ts">
      {`/**
 * AUTO-GENERATED FILE - DO NOT EDIT
 * Runtime tool schema index — converts Effect Schemas to JSON Schema tool definitions.
 * Generated by the TypeSpec emitter.
 */

import { JSONSchema, Schema } from 'effect';

export * from './common.ts';
export * from './execution.ts';
export * from './exploration.ts';
export * from './mutation.ts';

import { ExecutionSchemas } from './execution.ts';
import { ExplorationSchemas } from './exploration.ts';
import { MutationSchemas } from './mutation.ts';

// =============================================================================
// Tool Definition Type
// =============================================================================

export interface ToolDefinition {
  name: string;
  description: string;
  parameters: object;
}

// =============================================================================
// JSON Schema Generation
// =============================================================================

/** Recursively resolve $ref references in a JSON Schema */
function resolveRefs(obj: unknown, defs: Record<string, unknown>): unknown {
  if (obj === null || typeof obj !== 'object') return obj;
  if (Array.isArray(obj)) return obj.map((item) => resolveRefs(item, defs));

  const record = obj as Record<string, unknown>;

  if ('$ref' in record && typeof record['$ref'] === 'string') {
    const defName = record['$ref'].replace('#/$defs/', '');
    const resolved = defs[defName];
    if (resolved) {
      const { $ref: _, ...rest } = record;
      return { ...(resolveRefs(resolved, defs) as Record<string, unknown>), ...rest };
    }
  }

  if ('allOf' in record && Array.isArray(record['allOf']) && record['allOf'].length === 1) {
    const first = record['allOf'][0] as Record<string, unknown>;
    if ('$ref' in first) {
      const { allOf: _, ...rest } = record;
      return { ...(resolveRefs(first, defs) as Record<string, unknown>), ...rest };
    }
  }

  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(record)) {
    if (key === '$defs' || key === '$schema') continue;
    result[key] = resolveRefs(value, defs);
  }
  return result;
}

/** Convert an Effect Schema to a tool definition with JSON Schema parameters */
function schemaToToolDefinition<A, I, R>(schema: Schema.Schema<A, I, R>): ToolDefinition {
  const jsonSchema = JSONSchema.make(schema) as {
    $schema: string;
    $defs: Record<string, unknown>;
    $ref: string;
  };

  const defs = jsonSchema.$defs ?? {};
  const defName = (jsonSchema.$ref ?? '').replace('#/$defs/', '');
  const def = defs[defName] as {
    description?: string;
    type: string;
    properties: Record<string, unknown>;
    required?: string[];
  } | undefined;

  return {
    name: defName || 'unknown',
    description: def?.description ?? '',
    parameters: def
      ? {
          type: def.type,
          properties: resolveRefs(def.properties, defs),
          required: def.required,
          additionalProperties: false,
        }
      : jsonSchema,
  };
}

// =============================================================================
// Auto-generated Tool Definitions
// =============================================================================

export const executionSchemas = Object.values(ExecutionSchemas).map((s) =>
  schemaToToolDefinition(s as Schema.Schema<unknown, unknown>),
);

export const explorationSchemas = Object.values(ExplorationSchemas).map((s) =>
  schemaToToolDefinition(s as Schema.Schema<unknown, unknown>),
);

export const mutationSchemas = Object.values(MutationSchemas).map((s) =>
  schemaToToolDefinition(s as Schema.Schema<unknown, unknown>),
);

/** All tool schemas combined - ready for AI tool calling */
export const allToolSchemas = [...executionSchemas, ...explorationSchemas, ...mutationSchemas];

// =============================================================================
// Effect Schemas (for runtime validation)
// =============================================================================

export const EffectSchemas = {
  Execution: ExecutionSchemas,
  Exploration: ExplorationSchemas,
  Mutation: MutationSchemas,
} as const;

// =============================================================================
// Validation Helper
// =============================================================================

const schemaMap: Record<string, Schema.Schema<unknown, unknown>> = Object.fromEntries(
  Object.entries(EffectSchemas).flatMap(([, group]) =>
    Object.entries(group).map(([name, schema]) => [
      name.charAt(0).toLowerCase() + name.slice(1),
      schema as Schema.Schema<unknown, unknown>,
    ]),
  ),
);

/**
 * Validate tool input against the Effect Schema
 */
export function validateToolInput(
  toolName: string,
  input: unknown,
): { success: true; data: unknown } | { success: false; errors: string[] } {
  const schema = schemaMap[toolName];
  if (!schema) {
    return { success: false, errors: [\`Unknown tool: \${toolName}\`] };
  }

  try {
    const decoded = Schema.decodeUnknownSync(schema)(input);
    return { success: true, data: decoded };
  } catch (error) {
    if (error instanceof Error) {
      return { success: false, errors: [error.message] };
    }
    return { success: false, errors: ['Unknown validation error'] };
  }
}
`}
    </SourceFile>
  );
};
