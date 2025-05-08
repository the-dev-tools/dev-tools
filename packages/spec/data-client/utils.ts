import { create, DescMessage, MessageShape } from '@bufbuild/protobuf';
import { EntityMixin } from '@data-client/endpoint';
import { pipe, Struct } from 'effect';

interface MakeEntityProps<Desc extends DescMessage> {
  key: string;
  primaryKeys: (keyof MessageShape<Desc>)[];
  schema: Desc;
}

export const makeEntity = <Desc extends DescMessage>({ key, primaryKeys, schema }: MakeEntityProps<Desc>) => {
  const MessageClass = function () {
    return create(schema);
  } as unknown as new () => MessageShape<Desc>;

  const pk = (_: MessageShape<Desc>) => pipe(Struct.pick(_, ...primaryKeys), JSON.stringify);

  return EntityMixin(MessageClass, { key, pk });
};
