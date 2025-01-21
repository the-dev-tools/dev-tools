import { createRegistry, Message } from '@bufbuild/protobuf';
import { Array, HashMap, Option, pipe, Schema } from 'effect';

import { ChangeJson, ListChangeJson } from '@the-dev-tools/spec/change/v1/change_pb';
import { files } from '@the-dev-tools/spec/files';
import metaJson from '@the-dev-tools/spec/meta/meta.json';

export const registry = createRegistry(...files);

type AutoChangeSourceKind = 'REQUEST' | 'RESPONSE' | 'MERGE';

export interface AutoChangeSource {
  kind: AutoChangeSourceKind;
  $type: string;
}

interface AutoListChange extends Omit<ListChangeJson, 'parent'> {
  $parent: AutoChangeSource;
}

interface AutoChange extends Omit<ChangeJson, 'data' | 'list'> {
  $data?: AutoChangeSource;
  $list?: AutoListChange[];
}

interface Meta {
  autoChanges?: AutoChange[];
  base?: string;
  key?: string;
  normalKeys?: string[];
}

const metaMap = pipe(Array.fromRecord(metaJson as Record<string, Meta>), HashMap.fromIterable);

export const getMessageMeta = (message: Message) => HashMap.get(metaMap, message.$typeName);

export const getBaseMessageMeta = (message: Message): Option.Option<Meta> =>
  pipe(
    getMessageMeta(message),
    Option.flatMap((_) => {
      if (_.base && _.base !== message.$typeName) return getBaseMessageMeta({ $typeName: _.base });
      return Option.some(_);
    }),
  );

export const getMessageKey = (message: Message): Option.Option<string> =>
  pipe(
    getBaseMessageMeta(message),
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
