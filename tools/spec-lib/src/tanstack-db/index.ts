import {
  $withVisibilityFilter,
  createTypeSpecLibrary,
  DecoratorContext,
  EnumMember,
  EnumValue,
  getLifecycleVisibilityEnum,
  isKey,
  Model,
  ModelProperty,
  Program,
  Type,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { pipe, Record } from 'effect';
import { streams } from '../protobuf/lib.js';

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

const toDeltaProperty = (program: Program, unset: Type) => (property: ModelProperty) => {
  if (isKey(program, property)) return property;

  let type = property.type;

  if (property.optional) {
    const variants = $(program).union.is(type)
      ? type.variants.values().toArray()
      : [$(program).unionVariant.create({ type })];

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

    makeOperation(`${base.name}Create`, { input: createRequest });
  }

  if (canUpdate && !isReadOnly) {
    const updateItem = getOrMake(namespace.models, `${base.name}Update`, (name) =>
      $(program).model.create({
        decorators: [[$withVisibilityFilter, { all: [lifecycle.Update] }]],
        name,
        properties: pipe(base.properties.entries(), Record.fromEntries, Record.map(toDeltaProperty(program, unset))),
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

  if (canDelete && !isReadOnly) {
    const deleteItem = getOrMake(namespace.models, `${base.name}Delete`, (name) =>
      $(program).model.create({
        name,
        properties: pipe(
          base.properties.entries(),
          Record.fromEntries,
          Record.filter((_) => isKey(program, _)),
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
      properties: pipe(base.properties.entries(), Record.fromEntries, Record.map(toDeltaProperty(program, unset))),
    }),
  );

  const syncDeleteItem = getOrMake(namespace.models, `${base.name}SyncDelete`, (name) =>
    $(program).model.create({
      name,
      properties: pipe(
        base.properties.entries(),
        Record.fromEntries,
        Record.filter((_) => isKey(program, _)),
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
