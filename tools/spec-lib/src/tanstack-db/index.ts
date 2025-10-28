import {
  $withVisibilityFilter,
  createTypeSpecLibrary,
  DecoratorContext,
  EnumMember,
  EnumValue,
  getLifecycleVisibilityEnum,
  Model,
  Operation,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { pipe, Record } from 'effect';
import { deltaProperty, primaryKeys } from '../core/index.jsx';
import { streams } from '../protobuf/lib.js';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/tanstack-db',
});

export const $decorators = {
  'DevTools.TanStackDB': {
    collection,
  },
};

const getOrMake = <Key, Value>(map: Map<Key, Value>, key: Key, make: (key: Key) => Value) => {
  const value = map.get(key) ?? make(key);
  map.set(key, value);
  return value;
};

const { makeStateMap } = makeStateFactory((_) => $lib.createStateSymbol(_));

export interface CollectionOperationMeta {
  operation: Operation;
  request?: Model;
  response?: Model;
  item?: Model;
}

export interface CollectionMeta {
  base: Model;
  options: CollectionOptions;
  operations: {
    collection: CollectionOperationMeta;
    create?: CollectionOperationMeta;
    update?: CollectionOperationMeta;
    delete?: CollectionOperationMeta;
    sync: CollectionOperationMeta;
  };
}

export const collections = makeStateMap<Model, CollectionMeta>('collections');

interface CollectionOptions {
  canCreate?: boolean;
  canDelete?: boolean;
  canUpdate?: boolean;
  isReadOnly?: boolean;
}

function collection(
  { program }: DecoratorContext,
  base: Model,
  { canCreate = true, canDelete = true, canUpdate = true, isReadOnly = false }: CollectionOptions = {},
) {
  const { namespace } = base;
  if (!namespace) return;

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

  const collectionOperation = makeOperation(`${base.name}Collection`, { output: collectionResponse });

  const operations: Partial<CollectionMeta['operations']> = {
    collection: { operation: collectionOperation, response: collectionResponse },
  };

  if (canCreate && !isReadOnly) {
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

    const createOperation = makeOperation(`${base.name}Create`, { input: createRequest });

    operations.create = {
      operation: createOperation,
      request: createRequest,
      item: createItem,
    };
  }

  if (canUpdate && !isReadOnly) {
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

    const updateOperation = makeOperation(`${base.name}Update`, { input: updateRequest });

    operations.update = {
      operation: updateOperation,
      request: updateRequest,
      item: updateItem,
    };
  }

  if (canDelete && !isReadOnly) {
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

    const deleteOperation = makeOperation(`${base.name}Delete`, { input: deleteRequest });

    operations.delete = {
      operation: deleteOperation,
      request: deleteRequest,
      item: deleteItem,
    };
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

  operations.sync = { operation: sync, response: syncResponse };

  collections(program).set(base, {
    base,
    options: { canCreate, canDelete, canUpdate, isReadOnly },
    operations: operations as CollectionMeta['operations'],
  });
}
