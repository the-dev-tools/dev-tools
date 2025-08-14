import { Program } from '@typespec/compiler';

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
