import { JSONSchemaType, Program } from '@typespec/compiler';
import { JSONSchema, Schema } from 'effect';

export const makeStateFactory = (createStateSymbol: (name: string) => symbol) => {
  const makeStateMap = <K, V>(name: string) => {
    const key = createStateSymbol(name);
    return (program: Program) => program.stateMap(key) as Map<K, V>;
  };

  const makeStateSet = <T>(name: string) => {
    const key = createStateSymbol(name);
    return (program: Program) => program.stateSet(key) as Set<T>;
  };

  return { makeStateMap, makeStateSet };
};

export const makeEmitterOptions = <A, I, R>(schema: Schema.Schema<A, I, R>) => {
  const definitions: Record<string, never> = {};
  const jsonSchema = JSONSchema.fromAST(schema.ast, { additionalPropertiesStrategy: 'allow', definitions });
  return { ...jsonSchema, $defs: definitions } as JSONSchemaType<A>;
};
