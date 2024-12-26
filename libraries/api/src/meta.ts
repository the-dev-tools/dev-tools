import { Message } from '@bufbuild/protobuf';
import { Array, HashMap, Option, pipe, Schema } from 'effect';

import messageIdMap from '@the-dev-tools/spec/meta/message-id-map.json';

const messageIdHashMap = pipe(Array.fromRecord<string, string>(messageIdMap), HashMap.fromIterable);

export const getMessageId = (message: Message) =>
  pipe(
    HashMap.get(messageIdHashMap, message.$typeName),
    Option.filter((_) => _ in message),
    Option.map((_) => message[_ as keyof Message] as unknown),
    Option.flatMap(Schema.validateOption(Schema.Uint8Array)),
  );
