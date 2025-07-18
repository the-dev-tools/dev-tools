import { create, DescMessage, DescMethodUnary, MessageInitShape, MessageShape, toJson } from '@bufbuild/protobuf';
import { ContextValues, Transport } from '@connectrpc/connect';
import { Controller } from '@data-client/core';
import { EntityMixin, schema, SchemaSimple } from '@data-client/endpoint';
import { Equivalence, Option, pipe, Predicate, Record, Struct } from 'effect';

type EntityOptions = Omit<Parameters<typeof EntityMixin>[1], 'pk'>;

interface MakeEntityProps<Desc extends DescMessage> extends EntityOptions {
  message: Desc;
  primaryKeys: (keyof MessageShape<Desc>)[];
  requiredKeys: (keyof MessageShape<Desc>)[];
}

export const makeEntity = <Desc extends DescMessage>({
  message,
  primaryKeys,
  requiredKeys,
  ...props
}: MakeEntityProps<Desc>) => {
  const MessageClass = function (this: MessageShape<Desc>, init?: MessageInitShape<Desc>) {
    const value = create(message, init);
    Object.assign(this, value);
  } as unknown as new (init?: MessageInitShape<Desc>) => MessageShape<Desc>;

  const pk = (value: MessageInitShape<Desc> | undefined) =>
    pipe(create(message, value), (_) => toJson(message, _), Struct.pick(...primaryKeys), JSON.stringify);

  const validate = (value: object): string | undefined => {
    const missingKeys = requiredKeys.filter((_) => !Object.hasOwn(value, _));
    if (!missingKeys.length) return;
    return `Missing keys: ${missingKeys.join(', ')}`;
  };

  return EntityMixin(MessageClass, { pk, validate, ...props });
};

const transportKeys = new WeakMap<Transport, string>();
let transportCount = 0;

export const createTransportKey = (transport: Transport) => {
  let transportKey = transportKeys.get(transport);
  if (!transportKey) {
    transportKey = `t${++transportCount}`;
    transportKeys.set(transport, transportKey);
  }
  return transportKey;
};

export const createMessageKey = <T extends DescMessage>(schema: T, message: MessageInitShape<T>) =>
  pipe(create(schema, message), (_) => toJson(schema, _));

export const createMethodKey = <I extends DescMessage, O extends DescMessage>(
  transport: Transport,
  method: DescMethodUnary<I, O>,
  input: MessageInitShape<I>,
) => {
  const transportKey = createTransportKey(transport);
  const inputKey = createMessageKey(method.input, input);
  return JSON.stringify([transportKey, inputKey]);
};

export const createMethodKeyRecord = <M extends DescMethodUnary>(
  transport: Transport,
  method: M,
  input: MessageInitShape<M['input']>,
  inputPrimaryKeys?: (keyof MessageShape<M['input']>)[],
) =>
  pipe(
    createMessageKey(method.input, input),
    Option.liftPredicate(Predicate.isRecord),
    Option.map((_) => {
      if (inputPrimaryKeys) return Struct.pick(_, ...inputPrimaryKeys);
      return _;
    }),
    Option.getOrElse(() => ({})),
    (_): Record<string, unknown> => ({ ..._, transport: createTransportKey(transport) }),
    // eslint-disable-next-line @typescript-eslint/restrict-template-expressions
    Record.map((_) => `${_}`),
  );

export interface EndpointProps<M extends DescMethodUnary, N = Record<string, unknown>> {
  contextValues?: ContextValues;
  controller: () => Controller;
  header?: HeadersInit;
  input: MessageInitShape<M['input']> & N;
  signal?: AbortSignal;
  timeoutMs?: number;
  transport: Transport;
}

export const makeEndpointFn =
  <M extends DescMethodUnary>(method: M) =>
  async ({ contextValues, header, input, signal, timeoutMs, transport }: EndpointProps<M>) => {
    const response = await transport.unary(method, signal, timeoutMs, header, input, contextValues);
    return response.message as MessageShape<M['output']>;
  };

export const makeKey =
  <M extends DescMethodUnary>(method: M, name: string) =>
  ({ input, transport }: EndpointProps<M>) => {
    const transportKey = createTransportKey(transport);
    const inputKey = createMessageKey(method.input, input);
    return JSON.stringify([name, transportKey, inputKey]);
  };

interface MakeListCollectionProps<S extends SchemaSimple, M extends DescMethodUnary> {
  argsKey?: (props: EndpointProps<M> | null) => Record<string, string>;
  inputPrimaryKeys?: (keyof MessageShape<M['input']>)[];
  itemSchema: S;
  method: M;
}

export const makeListCollection = <S extends SchemaSimple, M extends DescMethodUnary>({
  argsKey: argsKeyCustom,
  inputPrimaryKeys,
  itemSchema,
  method,
}: MakeListCollectionProps<S, M>) => {
  const argsKeyDefault = (props: EndpointProps<M> | null) => {
    if (props === null) return {};
    const { input, transport } = props;
    return createMethodKeyRecord(transport, method, input, inputPrimaryKeys);
  };

  const argsKey = argsKeyCustom ?? argsKeyDefault;

  const createCollectionFilter = (props: EndpointProps<M>) => (collectionKey: Record<string, string>) => {
    const compare = Record.getEquivalence(Equivalence.string);
    return compare(argsKey(props), collectionKey);
  };

  return new schema.Collection([itemSchema], { argsKey, createCollectionFilter });
};
