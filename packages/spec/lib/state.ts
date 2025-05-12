import { Interface, Model, Namespace, Operation, Program, Type } from '@typespec/compiler';

import { $lib } from './lib.js';

// TypeSpec

export const keyMap = (program: Program) => program.stateMap(Symbol.for('TypeSpec.key')) as Map<Type, string>;

// Protobuf

export const messageSet = (program: Program) =>
  program.stateSet(Symbol.for('@typespec/protobuf.message')) as Set<Model>;

export const packageMap = (program: Program) =>
  program.stateMap(Symbol.for('@typespec/protobuf.package')) as Map<Namespace, Model>;

export const serviceSet = (program: Program) =>
  program.stateSet(Symbol.for('@typespec/protobuf.service')) as Set<Interface>;

// Custom

export const moveMap = (program: Program) => program.stateMap($lib.stateKeys.move) as Map<Model | Operation, Namespace>;

export const normalKeysMap = (program: Program) => program.stateMap($lib.stateKeys.normalKeys) as Map<Model, string[]>;

export const baseMap = (program: Program) => program.stateMap($lib.stateKeys.base) as Map<Model, Model>;

export const autoChangesMap = (program: Program) =>
  program.stateMap($lib.stateKeys.autoChanges) as Map<Model, unknown[]>;

export const entityMap = (program: Program) => program.stateMap($lib.stateKeys.entity) as Map<Model, Model>;

export interface EndpointMeta {
  method: string;
  options: Model | undefined;
}

export const endpointMap = (program: Program) =>
  program.stateMap($lib.stateKeys.endpoint) as Map<Operation, EndpointMeta>;
