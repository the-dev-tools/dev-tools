import { ModelProperty, Program } from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';

export interface FieldSchemaResult {
  expression: string;
  importFrom: 'common' | 'effect' | 'none';
  includesOptional: boolean;
  needsDescription: boolean;
  schemaName: string;
}

export function getFieldSchema(property: ModelProperty, program: Program): FieldSchemaResult {
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

export function formatStringLiteral(str: string): string {
  // Check if we need multi-line formatting
  if (str.length > 80 || str.includes('\n')) {
    return '`' + str.replace(/`/g, '\\`').replace(/\$/g, '\\$') + '`';
  }
  // Use single quotes for short strings
  return "'" + str.replace(/'/g, "\\'") + "'";
}
