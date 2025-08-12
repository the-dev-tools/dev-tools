import { Model, Operation, Program, Type } from '@typespec/compiler';
import { state } from './lib.js';

export const externals = (program: Program) => program.stateMap(state.externals) as Map<Type, [string, string]>;

export const maps = (program: Program) => program.stateMap(state.maps) as Map<Type, [Type, Type]>;

export const streams = (program: Program) =>
  program.stateMap(state.streams) as Map<Operation, 'Duplex' | 'In' | 'None' | 'Out'>;

export const templateInstances = (program: Program) =>
  program.stateMap(state.templateInstances) as Map<Model, Map<Model, Model>>;

export const templateNames = (program: Program) =>
  program.stateMap(state.templateNames) as Map<Model, Map<Type, string>>;

export const templates = (program: Program) => program.stateSet(state.templates);

export const instances = (program: Program) => program.stateSet(state.instances);
