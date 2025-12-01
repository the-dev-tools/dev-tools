import { MessageInitShape } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import { Array, flow, MutableHashSet, Option, pipe, Record, String, Struct } from 'effect';
import { Ulid } from 'id128';
import { useForm } from 'react-hook-form';
import {
  HttpMethod,
  HttpMethodSchema,
  HttpSearchParamInsertSchema,
  HttpSearchParamUpdateSchema,
} from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import {
  HttpCollectionSchema,
  HttpDeltaCollectionSchema,
  HttpSearchParamCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { Protobuf, useApiCollection } from '~/api';
import { ReferenceFieldRHF } from '~/reference';
import { DeltaResetButton, useDeltaState } from '~/utils/delta';
import { pick, queryCollection } from '~/utils/tanstack-db';

export interface HttpUrlProps {
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const HttpUrl = ({ deltaHttpId, httpId, isReadOnly = false }: HttpUrlProps) => {
  const deltaOptions = {
    deltaId: deltaHttpId,
    deltaSchema: HttpDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originId: httpId,
    originSchema: HttpCollectionSchema,
  };

  const [method, setMethod] = useDeltaState({ ...deltaOptions, valueKey: 'method' });
  const [url, setUrl] = useDeltaState({ ...deltaOptions, valueKey: 'url' });

  const searchParamCollection = useApiCollection(HttpSearchParamCollectionSchema);

  const { data: searchParams } = useLiveQuery(
    (_) =>
      _.from({ item: searchParamCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'httpSearchParamId', 'order', 'enabled', 'key', 'value')),
    [httpId, searchParamCollection],
  );

  const searchParamString = pipe(
    searchParams,
    Array.filterMap(
      flow(
        Option.liftPredicate((_) => _.enabled),
        Option.map((_) => `${_.key}=${_.value}`),
      ),
    ),
    Array.join('&'),
  );

  let urlString = url ?? '';
  if (searchParamString.length > 0) urlString += '?' + searchParamString;

  const form = useForm({ values: { urlString } });

  const submit = form.handleSubmit(async ({ urlString }) => {
    const { searchParamString, url } = pipe(
      urlString,
      String.indexOf('?'),
      Option.match({
        onNone: () => ({ searchParamString: '', url: urlString }),
        onSome: (separator) => ({
          searchParamString: urlString.slice(separator + 1),
          url: urlString.slice(0, separator),
        }),
      }),
    );

    setUrl(url);

    const searchParamSet = pipe(
      searchParamString,
      Option.liftPredicate(String.isNonEmpty),
      Option.map(String.split('&')),
      Option.getOrElse(Array.empty),
      MutableHashSet.fromIterable,
    );

    pipe(
      Array.filterMap(searchParams, (_) => {
        const searchParamString = `${_.key}=${_.value}`;
        const enabled = MutableHashSet.has(searchParamSet, searchParamString);
        MutableHashSet.remove(searchParamSet, searchParamString);
        if (_.enabled === enabled) return Option.none();
        return Option.some<MessageInitShape<typeof HttpSearchParamUpdateSchema>>({
          enabled,
          httpSearchParamId: _.httpSearchParamId,
        });
      }),
      (_) => searchParamCollection.utils.update(_),
    );

    const lastOrder = pipe(
      await queryCollection((_) =>
        _.from({ item: searchParamCollection })
          .orderBy((_) => _.item.order, 'desc')
          .select((_) => ({ order: _.item.order }))
          .limit(1)
          .findOne(),
      ),
      Array.head,
      Option.map((_) => _.order),
      Option.getOrElse(() => 0),
    );

    const orderSpacing = (Protobuf.MAX_FLOAT - lastOrder) / (MutableHashSet.size(searchParamSet) + 1);

    pipe(
      Array.fromIterable(searchParamSet),
      Array.map((_, index): MessageInitShape<typeof HttpSearchParamInsertSchema> => {
        const separator = _.indexOf('=');
        return {
          enabled: true,
          httpId,
          httpSearchParamId: Ulid.generate().bytes,
          key: separator ? _.slice(0, separator) : _,
          order: lastOrder + orderSpacing * (index + 1),
          value: separator ? _.slice(separator + 1) : '',
        };
      }),
      (_) => searchParamCollection.utils.insert(_),
    );
  });

  return (
    <div className={tw`flex flex-1 items-center gap-3 rounded-lg border border-slate-300 px-3 py-2 shadow-xs`}>
      <Select
        aria-label='Method'
        isDisabled={isReadOnly}
        items={pipe(Struct.omit(HttpMethodSchema.value, 0), Record.values)}
        onSelectionChange={(method) => {
          if (typeof method !== 'number') return;
          setMethod(method);
        }}
        selectedKey={method ?? HttpMethod.UNSPECIFIED}
        triggerClassName={tw`border-none p-0`}
      >
        {(_) => (
          <SelectItem id={_.number} textValue={_.localName}>
            <MethodBadge method={_.number} size='lg' />
          </SelectItem>
        )}
      </Select>

      <DeltaResetButton {...deltaOptions} valueKey='method' />

      <Separator className={tw`h-7`} orientation='vertical' />

      <ReferenceFieldRHF
        aria-label='URL'
        className={tw`flex-1 border-none font-medium tracking-tight`}
        control={form.control}
        kind='StringExpression'
        name='urlString'
        onBlur={() => void submit()}
        readOnly={isReadOnly}
      />

      <DeltaResetButton {...deltaOptions} valueKey='url' />
    </div>
  );
};
