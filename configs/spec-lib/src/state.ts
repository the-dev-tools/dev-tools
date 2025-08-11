import { Operation, Program, Type } from '@typespec/compiler';
import { state } from './lib.js';

export const externals = (program: Program) => program.stateMap(state.externals) as Map<Type, [string, string]>;

export const maps = (program: Program) => program.stateMap(state.maps) as Map<Type, [Type, Type]>;

export const streams = (program: Program) =>
  program.stateMap(state.streams) as Map<Operation, 'Duplex' | 'In' | 'None' | 'Out'>;
