import { create, DescMessage, DescMethodUnary, MessageShape } from '@bufbuild/protobuf';
import { Transport, UnaryResponse } from '@connectrpc/connect';
import { Endpoint, EndpointOptions, EntityMixin, Schema } from '@data-client/endpoint';
import { pipe, Struct } from 'effect';

type EntityOptions = Omit<Parameters<typeof EntityMixin>[1], 'pk'>;

interface MakeEntityProps<Desc extends DescMessage> extends EntityOptions {
  message: Desc;
  primaryKeys: (keyof MessageShape<Desc>)[];
}

export const makeEntity = <Desc extends DescMessage>({ message, primaryKeys, ...props }: MakeEntityProps<Desc>) => {
  const MessageClass = function () {
    return create(message);
  } as unknown as new () => MessageShape<Desc>;

  const pk = (_: MessageShape<Desc>) => pipe(Struct.pick(_, ...primaryKeys), JSON.stringify);

  return EntityMixin(MessageClass, { pk, ...props });
};

type FetchFunction<I extends DescMessage, O extends DescMessage> = (
  transport: Transport,
  input: MessageShape<I>,
) => Promise<UnaryResponse<I, O>>;

interface MakeEndpointProps<
  I extends DescMessage,
  O extends DescMessage,
  S extends Schema | undefined = undefined,
  M extends boolean | undefined = false,
> extends EndpointOptions<FetchFunction<I, O>, S, M> {
  method: DescMethodUnary<I, O>;
}

export const makeEndpoint = <
  I extends DescMessage,
  O extends DescMessage,
  S extends Schema | undefined = undefined,
  M extends boolean | undefined = false,
>({
  method,
  ...options
}: MakeEndpointProps<I, O, S, M>) => {
  const fetchFunction: FetchFunction<I, O> = (transport, input) => {
    return transport.unary(method, undefined, undefined, undefined, input);
  };

  return new Endpoint(fetchFunction, options);
};
