import { create, DescMessage, MessageShape } from '@bufbuild/protobuf';
import { EntityMixin } from '@data-client/endpoint';
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
