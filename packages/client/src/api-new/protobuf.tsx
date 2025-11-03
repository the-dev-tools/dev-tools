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
import { Option, pipe, Record, Struct } from 'effect';
import { files } from '@the-dev-tools/spec/files';

export * from '@bufbuild/protobuf';

export const registry = createRegistry(...files);

const validator = createValidator({ registry });
export const validate: typeof validator.validate = (...args) => validator.validate(...args);

export const standardSchema = <Desc extends DescMessage>(
  messageDesc: Desc,
  options?: ValidatorOptions,
): StandardSchemaV1<MessageShape<Desc>, MessageValidType<Desc>> =>
  createStandardSchema(messageDesc, { registry, ...options });

export type MessageAlikeInitShape<Desc extends DescMessage> = Omit<MessageInitShape<Desc>, keyof Message> &
  Partial<Message>;

export const createAlike = <Desc extends DescMessage>(schema: Desc, init: MessageAlikeInitShape<Desc>) =>
  create(schema, Struct.omit(init, '$typeName', '$unknown') as MessageInitShape<Desc>);

const fieldByNumberMemo = new Map<DescMessage, Map<number, DescField>>();
const fieldByNumber = (messageDesc: DescMessage, number: number) => {
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

export const toUnion = <T extends MessageUnion>(message: T) => {
  type Keys = keyof Omit<T, 'kind' | keyof Message>;
  type MessageUnion = Exclude<T[Keys], undefined>;

  const messageDesc = registry.getMessage(message.$typeName)!;
  const field = fieldByNumber(messageDesc, message.kind)!;

  return message[field.localName as never] as MessageUnion;
};

export const mergeDelta = <T extends Message>(value: T, delta: Message, unset: DescEnum): T => {
  const messageDesc = registry.getMessage(value.$typeName)!;
  const deltaMessageDesc = registry.getMessage(delta.$typeName)!;

  return pipe(
    Struct.omit(value, '$typeName', '$unknown') as Record<string, unknown>,
    Record.filterMap((value, key) => {
      const deltaValue: unknown = delta[key as keyof typeof delta];

      if (deltaValue === undefined) return Option.some(value);

      if (isUnion(deltaValue)) {
        const deltaField = fieldByNumber(deltaMessageDesc, deltaValue.kind)!;

        if (deltaField.message?.typeName === unset.typeName) return Option.none();

        if (!isUnion(value)) return Option.some(deltaValue[deltaField.localName as never]);
      }

      return Option.some(deltaValue);
    }),
    (_) => create(messageDesc, _) as T,
  );
};
