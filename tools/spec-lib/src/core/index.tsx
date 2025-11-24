import { Children, createContext, useContext } from '@alloy-js/core';
import {
  createTypeSpecLibrary,
  DecoratorContext,
  DecoratorFunction,
  EnumValue,
  Model,
  ModelProperty,
  Namespace,
  Program,
  Type,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { useTsp } from '@typespec/emitter-framework';
import { Array, Match, Option, pipe, Record, String } from 'effect';
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
    withDelta,
  },
  'DevTools.Private': {
    copyKeys,
  },
};

const { makeStateSet } = makeStateFactory((_) => $lib.createStateSymbol(_));

export const normalKeys = makeStateSet<ModelProperty>('primaryKeys');

function withDelta({ program }: DecoratorContext, target: Model) {
  const { namespace } = target;
  if (!namespace) return;

  const unset = $(program).type.resolve('DevTools.Global.UnsetDelta')!;

  const properties = pipe(
    target.properties.values().toArray(),
    Array.map((_) => {
      if (primaryKeys(program).has(_)) {
        const primaryKey = $(program).modelProperty.create({ ..._, name: 'delta' + String.capitalize(_.name) });
        primaryKeys(program).add(primaryKey);

        const foreignKey = $(program).modelProperty.create(_);
        foreignKeys(program).add(foreignKey);

        return [primaryKey, foreignKey];
      }

      if (foreignKeys(program).has(_)) return [_];

      return [deltaProperty(_, program, unset)];
    }),
    Array.flatten,
    Array.map((_) => [_.name, _] as const),
    Record.fromEntries,
  );

  $(program).model.create({
    decorators: pipe(
      Array.filterMap(target.decorators, (_) => {
        if (_.decorator === withDelta) return Option.none();
        return Option.some<[DecoratorFunction, ...unknown[]]>([_.decorator, ..._.args]);
      }),
      Array.prepend<DecoratorFunction>((_, target: Model) => {
        namespace.models.set(target.name, target);
        target.namespace = namespace;
      }),
    ),
    name: `${target.name}Delta`,
    properties,
    sourceModels: [{ model: target, usage: 'is' }],
  });
}

export interface Project {
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

export const deltaProperty = (property: ModelProperty, program: Program, unset: Type) => {
  let type = property.type;

  if (property.optional) {
    const variants = $(program).union.is(type)
      ? type.variants.values().toArray()
      : [$(program).unionVariant.create({ name: 'value', type })];

    const unsetVariant = $(program).unionVariant.create({ type: unset });
    variants.unshift(unsetVariant);

    type = $(program).union.create({ variants });
  }

  return $(program).modelProperty.create({
    name: property.name,
    optional: true,
    type,
  });
};
