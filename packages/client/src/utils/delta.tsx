import { Message, MessageShape } from '@bufbuild/protobuf';
import { debounceStrategy, eq, Ref, useLiveQuery, usePacedMutations } from '@tanstack/react-db';
import { DisplayColumnDef } from '@tanstack/table-core';
import { String } from 'effect';
import { Ulid } from 'id128';
import { Tooltip, TooltipTrigger } from 'react-aria-components';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { RedoIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { ApiCollectionSchema, Connect, useApiCollection } from '~/api';
import { ColumnActionDelete, ColumnActionDrag, columnActions } from '~/form-table';
import { ReferenceField } from '~/reference';
import { rootRouteApi } from '~/routes';
import { eqStruct, pick } from './tanstack-db';
import { Filter, PartialUndefined } from './types';

export interface UseDeltaStateProps<
  Schema extends ApiCollectionSchema,
  Key extends keyof MessageShape<Schema['item']>,
> {
  deltaId: Uint8Array | undefined;
  deltaSchema: ApiCollectionSchema;
  insertExtra?: object | undefined;
  isDelta?: boolean | undefined;
  originId: Uint8Array;
  originSchema: Schema;
  valueKey: Key;
}

export const useDeltaState = <Schema extends ApiCollectionSchema, Key extends keyof MessageShape<Schema['item']>>({
  deltaId,
  deltaSchema,
  insertExtra,
  isDelta = true,
  originId,
  originSchema,
  valueKey,
}: UseDeltaStateProps<Schema, Key>) => {
  type Value = MessageShape<Schema['item']>[Key];

  const { transport } = rootRouteApi.useRouteContext();

  const originIdKey = originSchema.keys[0];
  if (!originIdKey || originSchema.keys.length > 1) throw new Error('Unsupported delta collection');

  const originCollection = useApiCollection(originSchema);
  const origin = useLiveQuery(
    (_) =>
      _.from({ item: originCollection })
        .where((_) => eq(_.item![originIdKey as never], originId))
        .select((_) => pick(_.item as never, valueKey))
        .findOne(),
    [originId, valueKey, originCollection, originIdKey],
  ).data as Record<Key, Value> | undefined;
  const originValue = origin?.[valueKey];

  const deltaIdKey = deltaSchema.keys[0];
  if (!deltaIdKey || deltaSchema.keys.length > 1) throw new Error('Unsupported delta collection');

  const deltaCollection = useApiCollection(deltaSchema);
  const delta = useLiveQuery(
    (_) =>
      _.from({ item: deltaCollection })
        .where((_) => eq(_.item[deltaIdKey as never], deltaId))
        .select((_) => pick(_.item as never, valueKey))
        .findOne(),
    [deltaCollection, deltaId, deltaIdKey, valueKey],
  ).data as Record<Key, Value> | undefined;
  const deltaValue = delta?.[valueKey];

  let value = originValue;
  if (isDelta && deltaValue !== undefined) value = deltaValue;

  const updateOrigin = usePacedMutations({
    mutationFn: async ({ transaction }) => {
      const mutationTime = Date.now();
      const items = transaction.mutations.map(
        (_) =>
          ({
            ...originCollection.utils.parseKeyUnsafe(_.key as string),
            ..._.changes,
          }) as never,
      );
      await Connect.request({ input: { items }, method: originSchema.operations.update!, transport });
      await originCollection.utils.waitForSync(mutationTime);
    },
    onMutate: (value) => {
      const key = originCollection.utils.getKey({ [originIdKey]: originId } as never);
      originCollection.update(key, (_) => {
        _[valueKey as never] = value as never;
      });
    },
    strategy: debounceStrategy({ wait: 200 }),
  });

  const updateDelta = usePacedMutations({
    mutationFn: async ({ transaction }) => {
      const mutationTime = Date.now();
      const items = transaction.mutations.map(
        (_) =>
          ({
            ...deltaCollection.utils.parseKeyUnsafe(_.key as string),
            // TODO: deduplicate spec union kind enums and un-hardcode numeric value
            [valueKey]: { kind: 165745230 /* VALUE */, value: _.changes[valueKey as never] },
          }) as never,
      );
      await Connect.request({ input: { items }, method: deltaSchema.operations.update!, transport });
      await deltaCollection.utils.waitForSync(mutationTime);
    },
    onMutate: (value) => {
      const key = deltaCollection.utils.getKey({ [deltaIdKey]: deltaId } as never);
      deltaCollection.update(key, (_) => {
        _[valueKey as never] = value as never;
      });
    },
    strategy: debounceStrategy({ wait: 200 }),
  });

  const setValue = (value: Value) => {
    if (!isDelta) {
      if (originValue === undefined) {
        originCollection.utils.insert?.({
          [originIdKey]: originId,
          [valueKey]: value,
        } as never);
      } else {
        updateOrigin(value);
      }
    } else if (!deltaId) {
      deltaCollection.utils.insert?.({
        [deltaIdKey]: Ulid.generate().bytes,
        [originIdKey]: originId,
        [valueKey]: value,
        ...insertExtra,
      } as never);
    } else {
      if (delta === undefined) {
        deltaCollection.utils.insert?.({
          [deltaIdKey]: deltaId,
          [originIdKey]: originId,
          [valueKey]: value,
          ...insertExtra,
        } as never);
      } else {
        updateDelta(value);
      }
    }
  };

  return [value, setValue] as const;
};

export interface DeltaResetButtonProps<
  Schema extends ApiCollectionSchema,
  Key extends keyof MessageShape<Schema['item']>,
> {
  deltaId: Uint8Array | undefined;
  deltaSchema: Schema;
  isDelta?: boolean | undefined;
  valueKey: Key;
}

export const DeltaResetButton = <Schema extends ApiCollectionSchema, Key extends keyof MessageShape<Schema['item']>>({
  deltaId,
  deltaSchema,
  isDelta = true,
  valueKey,
}: DeltaResetButtonProps<Schema, Key>) => {
  const idKey = deltaSchema.keys[0];
  if (!idKey || deltaSchema.keys.length > 1) throw new Error('Unsupported delta collection');

  const collection = useApiCollection(deltaSchema);

  const delta = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item![idKey as never], deltaId))
        .select((_) => pick(_.item as never, valueKey))
        .findOne(),
    [collection, deltaId, idKey, valueKey],
  ).data as Record<Key, unknown> | undefined;
  const hasDelta = delta?.[valueKey] !== undefined;

  if (!isDelta) return null;

  return (
    <TooltipTrigger delay={750}>
      <Button
        className={tw`p-1 text-slate-500`}
        isDisabled={!deltaId || !hasDelta}
        onPress={() =>
          void collection.utils.update?.({
            [idKey]: deltaId,
            // TODO: deduplicate spec union kind enums and un-hardcode numeric value
            [valueKey]: { kind: 183079996 /* UNSET */, unset: 0 },
          } as never)
        }
        variant='ghost'
      >
        <RedoIcon />
      </Button>
      <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Reset delta</Tooltip>
    </TooltipTrigger>
  );
};

export interface DeltaOptions<TOriginSchema extends ApiCollectionSchema, TDeltaSchema extends ApiCollectionSchema> {
  deltaKey: keyof Filter<MessageShape<TDeltaSchema['item']>, Uint8Array> & string;
  deltaParentKey: PartialUndefined<MessageShape<TDeltaSchema['item']>>;
  deltaSchema: TDeltaSchema;
  isDelta?: boolean;
  originKey: keyof Filter<MessageShape<TOriginSchema['item']>, Uint8Array> & string;
  originSchema: TOriginSchema;
}

export interface UseDeltaColumnStateProps<
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
> extends DeltaOptions<TOriginSchema, TDeltaSchema> {
  originKeyObject: PartialUndefined<MessageShape<TOriginSchema['item']>>;
  valueKey: keyof MessageShape<TOriginSchema['item']>;
}

export const useDeltaColumnState = <
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
>({
  deltaKey,
  deltaParentKey,
  deltaSchema,
  isDelta,
  originKey,
  originKeyObject,
  originSchema,
  valueKey,
}: UseDeltaColumnStateProps<TOriginSchema, TDeltaSchema>) => {
  const originId = originKeyObject[originKey] as Uint8Array;

  const originCollection = useApiCollection(originSchema as ApiCollectionSchema);

  const isExtra =
    useLiveQuery(
      (_) =>
        _.from({ item: originCollection })
          .where(eqStruct(originKeyObject as Message))
          .select((_) => ({ isExtra: eqStruct(deltaParentKey as Message)(_) }))
          .findOne(),
      [deltaParentKey, originCollection, originKeyObject],
    ).data?.isExtra ?? false;

  const deltaCollection = useApiCollection(deltaSchema as ApiCollectionSchema);

  const deltaId = useLiveQuery(
    (_) =>
      _.from({ item: deltaCollection })
        .where(eqStruct(deltaParentKey as Message))
        .where(eqStruct(originKeyObject as Message))
        .select((_) => ({ deltaId: _.item[deltaKey as never] as Ref<Uint8Array> }))
        .findOne(),
    [deltaCollection, deltaKey, deltaParentKey, originKeyObject],
  ).data?.deltaId as Uint8Array | undefined;

  const deltaOptions = {
    deltaId,
    deltaSchema,
    isDelta: isDelta && !isExtra,
    originId,
    originSchema,
    valueKey: valueKey as never,
  };

  const [value, setValue] = useDeltaState({ ...deltaOptions, insertExtra: deltaParentKey });

  return { deltaOptions, setValue, value };
};

export interface DeltaCheckboxColumnProps<
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
  TData extends Partial<MessageShape<TOriginSchema['item']>>,
> extends DeltaOptions<TOriginSchema, TDeltaSchema>,
    Omit<DisplayColumnDef<TData>, 'id'> {
  isReadOnly?: boolean | undefined;
  valueKey: keyof Filter<MessageShape<TOriginSchema['item']>, boolean> & string;
}

export const deltaCheckboxColumn = <
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
  TData extends Partial<MessageShape<TOriginSchema['item']>>,
>(
  props: DeltaCheckboxColumnProps<TOriginSchema, TDeltaSchema, TData>,
): DisplayColumnDef<TData> => {
  const { isReadOnly = false, valueKey } = props;
  return {
    cell: function Cell({ row }) {
      const { deltaOptions, setValue, value } = useDeltaColumnState({ ...props, originKeyObject: row.original });

      return (
        <div className={tw`flex flex-1 gap-1 px-1`}>
          <Checkbox
            isReadOnly={isReadOnly}
            isSelected={value as unknown as boolean}
            isTableCell
            onChange={(_) => void setValue(_ as never)}
          />

          {!isReadOnly && <DeltaResetButton {...deltaOptions} />}
        </div>
      );
    },
    header: String.capitalize(valueKey),
    id: valueKey,
    size: 0,
    ...props,
  };
};

export interface DeltaTextFieldColumnProps<
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
  TData extends Partial<MessageShape<TOriginSchema['item']>>,
> extends DeltaOptions<TOriginSchema, TDeltaSchema>,
    Omit<DisplayColumnDef<TData>, 'id'> {
  isReadOnly?: boolean | undefined;
  valueKey: keyof Filter<MessageShape<TOriginSchema['item']>, string> & string;
}

export const deltaTextFieldColumn = <
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
  TData extends Partial<MessageShape<TOriginSchema['item']>>,
>(
  props: DeltaTextFieldColumnProps<TOriginSchema, TDeltaSchema, TData>,
): DisplayColumnDef<TData> => {
  const { isReadOnly = false, valueKey } = props;
  return {
    cell: function Cell({ row }) {
      const { deltaOptions, setValue, value } = useDeltaColumnState({ ...props, originKeyObject: row.original });

      return (
        <div className={tw`flex min-w-0 flex-1 gap-1`}>
          <TextInputField
            aria-label={valueKey}
            className={tw`flex-1`}
            isReadOnly={isReadOnly}
            isTableCell
            onChange={(_) => void setValue(_ as never)}
            placeholder={`Enter ${valueKey}`}
            value={value as unknown as string}
          />

          {!isReadOnly && <DeltaResetButton {...deltaOptions} />}
        </div>
      );
    },
    header: String.capitalize(valueKey),
    id: valueKey,
    ...props,
  };
};

export interface DeltaReferenceColumnProps<
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
  TData extends Partial<MessageShape<TOriginSchema['item']>>,
> extends DeltaOptions<TOriginSchema, TDeltaSchema>,
    Omit<DisplayColumnDef<TData>, 'id'> {
  allowFiles?: boolean;
  isReadOnly?: boolean | undefined;
  valueKey: keyof Filter<MessageShape<TOriginSchema['item']>, string> & string;
}

export const deltaReferenceColumn = <
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
  TData extends Partial<MessageShape<TOriginSchema['item']>>,
>(
  props: DeltaReferenceColumnProps<TOriginSchema, TDeltaSchema, TData>,
): DisplayColumnDef<TData> => {
  const { allowFiles, isReadOnly = false, valueKey } = props;
  return {
    cell: function Cell({ row }) {
      const { deltaOptions, setValue, value } = useDeltaColumnState({ ...props, originKeyObject: row.original });

      return (
        <div className={tw`flex min-w-0 flex-1 gap-1`}>
          <ReferenceField
            allowFiles={allowFiles}
            className='flex-1'
            kind='StringExpression'
            onChange={(_) => void setValue(_ as never)}
            placeholder={`Enter ${valueKey}`}
            readOnly={isReadOnly}
            value={value as unknown as string}
            variant='table-cell'
          />

          {!isReadOnly && <DeltaResetButton {...deltaOptions} />}
        </div>
      );
    },
    header: String.capitalize(valueKey),
    id: valueKey,
    ...props,
  };
};

export const deltaActionsColumn = <
  TOriginSchema extends ApiCollectionSchema,
  TDeltaSchema extends ApiCollectionSchema,
>({
  deltaParentKey,
  isDelta,
  originSchema,
}: DeltaOptions<TOriginSchema, TDeltaSchema>) =>
  columnActions<Partial<MessageShape<TOriginSchema['item']>>>({
    cell: function Cell({ row }) {
      const originCollection = useApiCollection(originSchema as ApiCollectionSchema);

      const isExtra =
        useLiveQuery(
          (_) =>
            _.from({ item: originCollection })
              .where(eqStruct(row.original as Message))
              .select((_) => ({ isExtra: eqStruct(deltaParentKey as Message)(_) }))
              .findOne(),
          [originCollection, row.original],
        ).data?.isExtra ?? false;

      return (
        <>
          {(!isDelta || isExtra) && (
            <ColumnActionDelete onDelete={() => void originCollection.utils.delete?.(row.original as never)} />
          )}
          <ColumnActionDrag />
        </>
      );
    },
  });
