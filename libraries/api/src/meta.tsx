import { createRegistry, Message } from '@bufbuild/protobuf';
import { Array, HashMap, Option, pipe, Schema } from 'effect';

import { files } from '@the-dev-tools/spec/files';
import metaJson from '@the-dev-tools/spec/meta/meta.json';

export const registry = createRegistry(...files);

interface Meta {
  key?: string;
  normalKeys?: string[];
  base?: string;
}

const metaMap = pipe(Array.fromRecord<string, Meta>(metaJson), HashMap.fromIterable);

export const getMessageMeta = (message: Message): Option.Option<Meta> =>
  pipe(
    HashMap.get(metaMap, message.$typeName),
    Option.flatMap((_) => {
      if (_.base) return getMessageMeta({ $typeName: _.base });
      return Option.some({ ..._, base: message.$typeName });
    }),
  );

export const getMessageKey = (message: Message): Option.Option<string> =>
  pipe(
    getMessageMeta(message),
    Option.flatMapNullable((_) => _.key),
  );

export const getMessageId = (message: Message) =>
  pipe(
    getMessageKey(message),
    Option.filter((_) => _ in message),
    Option.map((_) => message[_ as keyof Message] as unknown),
    Option.flatMap(Schema.validateOption(Schema.Uint8Array)),
  );

export const setMessageId = <T extends Message>(message: T, id: Uint8Array) =>
  pipe(
    getMessageKey(message),
    Option.match({
      onNone: () => message,
      onSome: (key) => ({ ...message, [key]: id }),
    }),
  );
