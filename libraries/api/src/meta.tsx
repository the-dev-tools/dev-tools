import { createRegistry, Message } from '@bufbuild/protobuf';
import { Array, HashMap, Option, pipe, Schema } from 'effect';

import { files } from '@the-dev-tools/spec/files';
import messageIdMap from '@the-dev-tools/spec/meta/message-id-map.json';

export const registry = createRegistry(...files);

const messageIdHashMap = pipe(Array.fromRecord<string, string>(messageIdMap), HashMap.fromIterable);

export const getMessageIdKey = (message: Message) => HashMap.get(messageIdHashMap, message.$typeName);

export const getMessageId = (message: Message) =>
  pipe(
    getMessageIdKey(message),
    Option.filter((_) => _ in message),
    Option.map((_) => message[_ as keyof Message] as unknown),
    Option.flatMap(Schema.validateOption(Schema.Uint8Array)),
  );

export const setMessageId = <T extends Message>(message: T, id: Uint8Array) => {
  const maybeKey = HashMap.get(messageIdHashMap, message.$typeName);
  if (Option.isNone(maybeKey)) return message;
  return { ...message, [maybeKey.value]: id };
};
