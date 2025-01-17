import { createRegistry, Message } from '@bufbuild/protobuf';
import { Array, HashMap, Option, pipe, Schema } from 'effect';

import { files } from '@the-dev-tools/spec/files';
import metaJson from '@the-dev-tools/spec/meta/meta.json';

export const registry = createRegistry(...files);

const meta = pipe(
  Array.fromRecord<string, { base?: string; key?: string; normalKeys?: string[] }>(metaJson),
  HashMap.fromIterable,
);

export const getMessageIdKey = (message: Message): Option.Option<string> =>
  pipe(
    HashMap.get(meta, message.$typeName),
    Option.flatMap((_) => {
      if (_.key) return Option.some(_.key);
      if (_.base) return getMessageIdKey({ $typeName: _.base });
      return Option.none();
    }),
  );

export const getMessageId = (message: Message) =>
  pipe(
    getMessageIdKey(message),
    Option.filter((_) => _ in message),
    Option.map((_) => message[_ as keyof Message] as unknown),
    Option.flatMap(Schema.validateOption(Schema.Uint8Array)),
  );

export const setMessageId = <T extends Message>(message: T, id: Uint8Array) => {
  const maybeKey = getMessageIdKey(message);
  if (Option.isNone(maybeKey)) return message;
  return { ...message, [maybeKey.value]: id };
};
