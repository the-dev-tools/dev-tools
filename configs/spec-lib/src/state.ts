import { Model, Operation, Program, Type } from '@typespec/compiler';
import { state } from './lib.js';

// Lib

interface AddInstanceProps {
  base: Model;
  instance: Model;
  override?: boolean;
  program: Program;
  template: Model;
}

export const addInstance = ({ base, instance, override, program, template }: AddInstanceProps) => {
  if (instances(program).has(instance) && override !== true) return;

  let baseInstances = instancesByModel(program).get(base);
  baseInstances ??= new Set();

  const oldInstance = instancesByTemplate(program).get(template);
  if (oldInstance) baseInstances.delete(oldInstance);

  baseInstances.add(instance);
  instances(program).add(instance);
  instancesByModel(program).set(base, baseInstances);
  instancesByTemplate(program).set(template, instance);
};

export const instances = (program: Program) => program.stateSet(state.instances);

export const instancesByModel = (program: Program) =>
  program.stateMap(state.instancesByModel) as Map<Model, Set<Model>>;

export const instancesByTemplate = (program: Program) =>
  program.stateMap(state.instancesByTemplate) as Map<Type, Model>;

export const templates = (program: Program) => program.stateSet(state.templates);

// TypeSpec

export const streams = (program: Program) =>
  program.stateMap(state.streams) as Map<Operation, 'Duplex' | 'In' | 'None' | 'Out'>;

// TypeSpec.Private

export const externals = (program: Program) => program.stateMap(state.externals) as Map<Type, [string, string]>;

export const maps = (program: Program) => program.stateMap(state.maps) as Map<Type, [Type, Type]>;
