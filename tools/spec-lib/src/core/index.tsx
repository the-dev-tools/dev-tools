import { Children, createContext, useContext } from '@alloy-js/core';
import {
  createTypeSpecLibrary,
  DecoratorContext,
  EnumValue,
  Model,
  ModelProperty,
  Namespace,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { useTsp } from '@typespec/emitter-framework';
import { Array, Match, pipe } from 'effect';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/core',
});

export const $decorators = {
  DevTools: {
    foreignKey,
    primaryKey,
    project,
  },
  'DevTools.Private': {
    copyKeys,
  },
};

const { makeStateSet } = makeStateFactory((_) => $lib.createStateSymbol(_));

export const normalKeys = makeStateSet<ModelProperty>('primaryKeys');

interface Project {
  namespace: Namespace;
  version: number;
}

export const projects = makeStateSet<Project>('projects');

function project({ program }: DecoratorContext, target: Namespace, version = 1) {
  projects(program).add({ namespace: target, version });
}

const ProjectContext = createContext<Project>();

export const useProject = () => useContext(ProjectContext)!;

interface ProjectsProps {
  children: (project_: Project) => Children;
}

export const Projects = ({ children }: ProjectsProps) => {
  const { program } = useTsp();

  return pipe(
    projects(program).values(),
    Array.fromIterable,
    Match.value,
    Match.when(
      (_) => _.length === 1,
      (_) => {
        const project_ = _[0]!;
        return <ProjectContext.Provider value={project_}>{children(_[0]!)}</ProjectContext.Provider>;
      },
    ),
    Match.orElse((_) => _.map((_) => <ProjectContext.Provider value={_}>{children(_)}</ProjectContext.Provider>)),
  );
};

export const primaryKeys = makeStateSet<ModelProperty>('primaryKeys');
export const foreignKeys = makeStateSet<ModelProperty>('foreignKeys');

function primaryKey({ program }: DecoratorContext, target: ModelProperty) {
  primaryKeys(program).add(target);
}

function foreignKey({ program }: DecoratorContext, target: ModelProperty) {
  foreignKeys(program).add(target);
}

function copyKeys(
  { program }: DecoratorContext,
  target: Model,
  source: Model,
  asKeys: { foreign?: EnumValue; primary?: EnumValue },
) {
  type AsKey = 'Foreign' | 'None' | 'Omit' | 'Primary';
  const primaryAs = (asKeys.primary?.value.name as AsKey | undefined) ?? 'Primary';
  const foreignAs = (asKeys.foreign?.value.name as AsKey | undefined) ?? 'Foreign';

  const addKey = (key: ModelProperty, asKey: AsKey) => {
    if (asKey === 'Omit') return;

    const copiedKey = $(program).modelProperty.create(key);
    target.properties.set(key.name, copiedKey);

    if (asKey === 'Primary') copiedKey.decorators.push({ args: [], decorator: primaryKey });
    if (asKey === 'Foreign') copiedKey.decorators.push({ args: [], decorator: foreignKey });
  };

  source.properties.forEach((_) => {
    if (primaryKeys(program).has(_)) addKey(_, primaryAs);
    if (foreignKeys(program).has(_)) addKey(_, foreignAs);
  });
}
