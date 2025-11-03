import {
  $withVisibilityFilter,
  createTypeSpecLibrary,
  DecoratorContext,
  EnumMember,
  EnumValue,
  getLifecycleVisibilityEnum,
  Model,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { pipe, Record, Schema } from 'effect';
import { deltaProperty, primaryKeys } from '../core/index.jsx';
import { streams } from '../protobuf/lib.js';
import { makeStateFactory } from '../utils.js';

export class EmitterOptions extends Schema.Class<EmitterOptions>('EmitterOptions')({
  bufTypeScriptPath: Schema.String,
}) {}

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/tanstack-db',
});

export const $decorators = {
  'DevTools.TanStackDB': {
    collection,
  },
};

const { makeStateMap } = makeStateFactory((_) => $lib.createStateSymbol(_));

const getOrMake = <Key, Value>(map: Map<Key, Value>, key: Key, make: (key: Key) => Value) => {
  const value = map.get(key) ?? make(key);
  map.set(key, value);
  return value;
};

export const collections = makeStateMap<Model, CollectionOptions>('collections');

interface CollectionOptions {
  canCreate: boolean;
  canDelete: boolean;
  canUpdate: boolean;
  isReadOnly: boolean;
}

function collection({ program }: DecoratorContext, base: Model, optionsMaybe?: Partial<CollectionOptions>) {
  const { namespace } = base;
  if (!namespace) return;

  const options: CollectionOptions = pipe(optionsMaybe ?? {}, (_) => ({
    canCreate: (_.canCreate ?? true) && !_.isReadOnly,
    canDelete: (_.canDelete ?? true) && !_.isReadOnly,
    canUpdate: (_.canUpdate ?? true) && !_.isReadOnly,
    isReadOnly: _.isReadOnly ?? false,
  }));

  collections(program).set(base, options);

  base.properties.forEach((_) => void $(program).type.finishType(_));

  const lifecycle = pipe(
    getLifecycleVisibilityEnum(program).members.entries(),
    (_) => Record.fromEntries(_) as Record<'Create' | 'Delete' | 'Query' | 'Read' | 'Update', EnumMember>,
    Record.map((_): EnumValue => ({ entityKind: 'Value', type: _, value: _, valueKind: 'EnumValue' })),
  );

  const unset = $(program).type.resolve('DevTools.Global.Unset')!;

  const makeOperation = (name: string, { input, output }: { input?: Model; output?: Model }) => {
    const opertion = $(program).operation.create({
      name,
      parameters: input?.properties.values().toArray() ?? [],
      returnType: output ?? $(program).model.create({ properties: {} }),
    });

    if (input) opertion.parameters.sourceModels = [{ model: input, usage: 'spread' }];

    namespace.operations.set(opertion.name, opertion);

    return opertion;
  };

  const collectionResponse = getOrMake(namespace.models, `${base.name}CollectionResponse`, (name) =>
    $(program).model.create({
      name,
      properties: {
        items: $(program).modelProperty.create({
          name: 'items',
          type: $(program).array.create(base),
        }),
      },
    }),
  );

  makeOperation(`${base.name}Collection`, { output: collectionResponse });

  if (options.canCreate) {
    const createItem = getOrMake(namespace.models, `${base.name}Create`, (name) =>
      $(program).model.create({
        decorators: [[$withVisibilityFilter, { all: [lifecycle.Create] }]],
        name,
        properties: Record.fromEntries(base.properties.entries()),
      }),
    );

    const createRequest = getOrMake(namespace.models, `${base.name}CreateRequest`, (name) =>
      $(program).model.create({
        name,
        properties: {
          items: $(program).modelProperty.create({
            name: 'items',
            type: $(program).array.create(createItem),
          }),
        },
      }),
    );

    makeOperation(`${base.name}Create`, { input: createRequest });
  }

  if (options.canUpdate) {
    const updateItem = getOrMake(namespace.models, `${base.name}Update`, (name) =>
      $(program).model.create({
        decorators: [[$withVisibilityFilter, { all: [lifecycle.Update] }]],
        name,
        properties: pipe(
          base.properties.entries(),
          Record.fromEntries,
          Record.map((_) => {
            if (primaryKeys(program).has(_)) return _;
            return deltaProperty(_, program, unset);
          }),
        ),
      }),
    );

    const updateRequest = getOrMake(namespace.models, `${base.name}UpdateRequest`, (name) =>
      $(program).model.create({
        name,
        properties: {
          items: $(program).modelProperty.create({
            name: 'items',
            type: $(program).array.create(updateItem),
          }),
        },
      }),
    );

    makeOperation(`${base.name}Update`, { input: updateRequest });
  }

  if (options.canDelete) {
    const deleteItem = getOrMake(namespace.models, `${base.name}Delete`, (name) =>
      $(program).model.create({
        name,
        properties: pipe(
          base.properties.entries(),
          Record.fromEntries,
          Record.filter((_) => primaryKeys(program).has(_)),
        ),
      }),
    );

    const deleteRequest = getOrMake(namespace.models, `${base.name}DeleteRequest`, (name) =>
      $(program).model.create({
        name,
        properties: {
          items: $(program).modelProperty.create({
            name: 'items',
            type: $(program).array.create(deleteItem),
          }),
        },
      }),
    );

    makeOperation(`${base.name}Delete`, { input: deleteRequest });
  }

  const syncCreateItem = getOrMake(namespace.models, `${base.name}SyncCreate`, (name) =>
    $(program).model.create({
      name,
      properties: Record.fromEntries(base.properties.entries()),
    }),
  );

  const syncUpdateItem = getOrMake(namespace.models, `${base.name}SyncUpdate`, (name) =>
    $(program).model.create({
      name,
      properties: pipe(
        base.properties.entries(),
        Record.fromEntries,
        Record.map((_) => {
          if (primaryKeys(program).has(_)) return _;
          return deltaProperty(_, program, unset);
        }),
      ),
    }),
  );

  const syncDeleteItem = getOrMake(namespace.models, `${base.name}SyncDelete`, (name) =>
    $(program).model.create({
      name,
      properties: pipe(
        base.properties.entries(),
        Record.fromEntries,
        Record.filter((_) => primaryKeys(program).has(_)),
      ),
    }),
  );

  const syncItem = getOrMake(namespace.models, `${base.name}Sync`, (name) =>
    $(program).model.create({
      name,
      properties: {
        value: $(program).modelProperty.create({
          name: 'value',
          type: $(program).union.create({
            variants: [
              $(program).unionVariant.create({ name: 'create', type: syncCreateItem }),
              $(program).unionVariant.create({ name: 'update', type: syncUpdateItem }),
              $(program).unionVariant.create({ name: 'delete', type: syncDeleteItem }),
            ],
          }),
        }),
      },
    }),
  );

  const syncResponse = getOrMake(namespace.models, `${base.name}SyncResponse`, (name) =>
    $(program).model.create({
      name,
      properties: {
        items: $(program).modelProperty.create({
          name: 'items',
          type: $(program).array.create(syncItem),
        }),
      },
    }),
  );

  const sync = makeOperation(`${base.name}Sync`, { output: syncResponse });
  streams(program).set(sync, 'Out');
}
