import { EnumShape, enumToJson, JsonValue } from '@bufbuild/protobuf';
import { GenEnum } from '@bufbuild/protobuf/codegenv1';
import { pipe, String } from 'effect';

export const enumToString = <RuntimeShape extends number, JsonType extends JsonValue, Name extends string>(
  schema: GenEnum<RuntimeShape, JsonType>,
  name: Name,
  value: EnumShape<GenEnum<RuntimeShape, JsonType>>,
) =>
  pipe(
    enumToJson(schema, value) as string,
    String.substring(`${name}_`.length),
    (_) => {
      if (_ === 'UNSPECIFIED') throw Error(`Enum ${name} is unspecified`);
      return _ as JsonType extends `${Name}_${infer Kind}` ? Exclude<Kind, 'UNSPECIFIED'> : never;
    },
    String.toLowerCase,
  );
