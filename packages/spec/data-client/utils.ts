import { create, DescMessage, DescMethodUnary, MessageInitShape, MessageShape, toJson } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { EntityMixin } from '@data-client/endpoint';
import { Option, pipe, Predicate, Record, Struct } from 'effect';

type EntityOptions = Omit<Parameters<typeof EntityMixin>[1], 'pk'>;

interface MakeEntityProps<Desc extends DescMessage> extends EntityOptions {
  message: Desc;
  primaryKeys: (keyof MessageShape<Desc>)[];
}

export const makeEntity = <Desc extends DescMessage>({ message, primaryKeys, ...props }: MakeEntityProps<Desc>) => {
  const MessageClass = function (init?: MessageInitShape<Desc>) {
    return create(message, init);
  } as unknown as new (init?: MessageInitShape<Desc>) => MessageShape<Desc>;

  const pk = (_: MessageShape<Desc>) => pipe(Struct.pick(_, ...primaryKeys), JSON.stringify);

  return EntityMixin(MessageClass, { pk, ...props });
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

export const createMethodKeyRecord = <I extends DescMessage, O extends DescMessage>(
  transport: Transport,
  method: DescMethodUnary<I, O>,
  input: MessageInitShape<I>,
  inputPrimaryKeys?: (keyof MessageShape<I>)[],
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

export const fetchMethod = async <I extends DescMessage, O extends DescMessage>(
  transport: Transport,
  method: DescMethodUnary<I, O>,
  input: MessageInitShape<I>,
) => {
  const response = await transport.unary(method, undefined, undefined, undefined, input);
  return response.message;
};
