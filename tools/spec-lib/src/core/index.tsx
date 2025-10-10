import { Children, createContext, SourceDirectory, useContext } from '@alloy-js/core';
import {
  createTypeSpecLibrary,
  DecoratorContext,
  isKey,
  Model,
  ModelProperty,
  Namespace,
  Operation,
  Program,
  Type,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { useTsp } from '@typespec/emitter-framework';
import { Array, Match, Option, pipe } from 'effect';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/core',
});

export const $decorators = {
  DevTools: {
    copyParent,
    normalKey,
    parent,
    project,
    rename,
  },
  'DevTools.Private': {
    copyKey,
    copyParentKey,
    omitKey,
  },
};

const { makeStateMap, makeStateSet } = makeStateFactory((_) => $lib.createStateSymbol(_));

export const normalKeys = makeStateSet<ModelProperty>('normalKeys');
export const parents = makeStateMap<Model, Model>('parents');

const getModelKey = (program: Program, model: Model) =>
  pipe(
    $(program).model.getProperties(model),
    Array.fromIterable,
    Array.findFirst(([_key, value]) => isKey(program, value)),
  );

function parent({ program }: DecoratorContext, target: Model, parent: Model) {
  parents(program).set(target, parent);
}

function copyParent({ program }: DecoratorContext, target: Model, base: Model) {
  const parent = parents(program).get(base);
  if (parent) parents(program).set(target, parent);
}

function copyKey({ program }: DecoratorContext, target: Model, source: Model) {
  Option.gen(function* () {
    const [key, value] = yield* getModelKey(program, source);
    target.properties.set(key, value);
  });
}

function copyParentKey({ program }: DecoratorContext, target: Model, base: Model) {
  Option.gen(function* () {
    const parent = yield* Option.fromNullable(parents(program).get(base));
    const [key, value] = yield* getModelKey(program, parent);
    target.properties.set(key, value);
    normalKeys(program).add(value);
  });
}

function omitKey({ program }: DecoratorContext, target: Model) {
  Option.gen(function* () {
    const [key] = yield* getModelKey(program, target);
    target.properties.delete(key);
  });
}

function normalKey({ program }: DecoratorContext, target: ModelProperty) {
  normalKeys(program).add(target);
}

function rename(_: DecoratorContext, target: Model | Operation, name: string, sourceObject: Type | undefined) {
  if (sourceObject)
    name = name.replace(/{(\w+)}/g, (_, propName) => (sourceObject as never)[propName as never] as string);

  target.name = name;
}

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
    Match.orElse((_) =>
      _.map((_) => (
        <ProjectContext.Provider value={_}>
          <SourceDirectory path={_.namespace.name}>{children(_)}</SourceDirectory>
        </ProjectContext.Provider>
      )),
    ),
  );
};
