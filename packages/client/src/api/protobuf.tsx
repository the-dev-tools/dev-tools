import {
  create,
  createRegistry,
  DescEnum,
  DescField,
  DescMessage,
  isMessage,
  Message,
  MessageInitShape,
  MessageShape,
  MessageValidType,
} from '@bufbuild/protobuf';
import { createStandardSchema, createValidator, ValidatorOptions } from '@bufbuild/protovalidate';
import { StandardSchemaV1 } from '@standard-schema/spec';
import { Array, Option, pipe, Record, Struct } from 'effect';
import { files } from '@the-dev-tools/spec/buf/files';

export * from '@bufbuild/protobuf';
export * as WKT from '@bufbuild/protobuf/wkt';

// https://protobuf.dev/programming-guides/proto3/#scalar
// https://stdlib.io/docs/api/latest/@stdlib/constants/float32/max
export const MAX_FLOAT = 3.4028234663852886e38;
export const MAX_DOUBLE = Number.MAX_VALUE;

export const registry = createRegistry(...files);

const validator = createValidator({ registry });
export const validate: typeof validator.validate = (...args) => validator.validate(...args);

export const standardSchema = <Desc extends DescMessage>(
  messageDesc: Desc,
  options?: ValidatorOptions,
): StandardSchemaV1<MessageShape<Desc>, MessageValidType<Desc>> =>
  createStandardSchema(messageDesc, { registry, ...options });

export const messageMetaKeys = ['$typeName', '$unknown'] as const;

export type MessageMetaKeys = (typeof messageMetaKeys)[number];

export type MessageAlikeInitShape<Desc extends DescMessage> = Omit<MessageInitShape<Desc>, keyof Message> &
  Partial<Message>;

export const createAlike = <Desc extends DescMessage>(schema: Desc, init: MessageAlikeInitShape<Desc>) =>
  create(schema, Struct.omit(init, ...messageMetaKeys) as MessageInitShape<Desc>);

export type MessageData<T extends Message> = Omit<T, MessageMetaKeys>;

export const messageData = <T extends Message>(message: T) =>
  Struct.omit(message, ...messageMetaKeys) as MessageData<T>;

export const enumToString = (schema: DescEnum, value: number) => schema.value[value]?.localName;

const fieldByNumberMemo = new Map<DescMessage, Map<number, DescField>>();
const fieldByNumber = (message: Message, number: number) => {
  const messageDesc = registry.getMessage(message.$typeName)!;

  let localFieldByNumberMemo = fieldByNumberMemo.get(messageDesc);

  if (!localFieldByNumberMemo) {
    const entries = messageDesc.fields.map((_) => [_.number, _] as const);
    localFieldByNumberMemo = new Map(entries);
    fieldByNumberMemo.set(messageDesc, localFieldByNumberMemo);
  }

  return localFieldByNumberMemo.get(number);
};

export interface MessageUnion extends Message {
  kind: number;
}

export const isUnion = (value: unknown): value is MessageUnion => isMessage(value) && 'kind' in value;

export const isUnionDesc = (value?: DescMessage) => value && 'kind' in value.field;

export const toUnion = <T extends MessageUnion>(message: T) => {
  type Keys = keyof Omit<T, 'kind' | keyof Message>;
  type MessageUnion = Exclude<T[Keys], undefined>;

  const field = fieldByNumber(message, message.kind)!;

  return message[field.localName as never] as MessageUnion;
};

export const mergeDelta = <T extends Message>(
  value: Record<string, unknown> & T,
  delta: Message & Record<string, unknown>,
  unset: DescEnum,
): T => {
  const messageDesc = registry.getMessage(value.$typeName)!;

  return pipe(
    Array.filterMap(messageDesc.fields, ({ localName: key }): Option.Option<[string, unknown]> => {
      const deltaValue = delta[key];

      if (deltaValue === undefined) return Option.some([key, value[key]]);

      if (isUnion(deltaValue)) {
        const deltaField = fieldByNumber(deltaValue, deltaValue.kind)!;

        if (deltaField.enum?.typeName === unset.typeName) return Option.none();

        if (!isUnionDesc(messageDesc.field[key]?.message))
          return Option.some([key, deltaValue[deltaField.localName as keyof typeof deltaValue]]);
      }

      return Option.some([key, deltaValue]);
    }),
    Record.fromEntries,
    (_) => create(messageDesc, _) as T,
  );
};

export const draftDelta = (
  draft: Message & Record<string, unknown>,
  delta: Message & Record<string, unknown>,
  unset: DescEnum,
) => {
  const messageDesc = registry.getMessage(draft.$typeName)!;

  Array.forEach(messageDesc.fields, ({ localName: key }) => {
    const deltaValue = delta[key];

    if (deltaValue === undefined) return;

    if (isUnion(deltaValue)) {
      const deltaField = fieldByNumber(deltaValue, deltaValue.kind)!;

      if (deltaField.enum?.typeName === unset.typeName) return (draft[key] = undefined);

      if (!isUnionDesc(messageDesc.field[key]?.message))
        return (draft[key] = deltaValue[deltaField.localName as keyof typeof deltaValue]);
    }

    return (draft[key] = deltaValue);
  });
};
