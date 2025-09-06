import { Value } from '@bufbuild/protobuf/wkt';
import { Array, Match, pipe, Record, Tuple } from 'effect';
import * as RAC from 'react-aria-components';
import { tw } from './tailwind-literal';
import { TreeItem, TreeItemProps } from './tree';

export const jsonTreeItemProps = (jsonValue?: Value) => {
  if (!jsonValue) return undefined;
  return pipe(
    Match.value(jsonValue.kind),
    Match.when({ case: 'structValue' }, (_) =>
      pipe(
        Record.toEntries(_.value.fields),
        Array.map(([jsonKey, jsonValue]): JsonTreeItemProps => ({ id: jsonKey, jsonKey, jsonValue })),
      ),
    ),
    Match.when({ case: 'listValue' }, (_) =>
      Array.map(_.value.values, (jsonValue, jsonIndex): JsonTreeItemProps => ({ id: jsonIndex, jsonIndex, jsonValue })),
    ),
    Match.orElse((): JsonTreeItemProps[] => [{ id: 'root', jsonValue }]),
  );
};

export interface JsonTreeItemProps {
  id?: RAC.Key;
  jsonIndex?: number | undefined;
  jsonKey?: string | undefined;
  jsonValue: Value;
}

export const JsonTreeItem = ({ id = 'root', jsonIndex, jsonKey, jsonValue }: JsonTreeItemProps) => {
  const itemProps = pipe(
    Match.value(jsonValue.kind),
    Match.when({ case: 'structValue' }, (_) => ({
      item: ([key, value]: [string, Value]) => <JsonTreeItem id={`${id}.${key}`} jsonKey={key} jsonValue={value} />,
      items: Record.toEntries(_.value.fields),
    })),
    Match.when({ case: 'listValue' }, (_) => ({
      item: ([value, index]: [Value, number]) => (
        <JsonTreeItem id={`${id}.${index}`} jsonIndex={index} jsonValue={value} />
      ),
      items: pipe(_.value.values, Array.map(Tuple.make)),
    })),
    Match.orElse(() => ({})),
  );

  const kindText = pipe(
    Match.value(jsonValue.kind),
    Match.when({ case: 'structValue' }, () => 'object'),
    Match.when({ case: 'listValue' }, () => 'array'),
    Match.orElse(() => undefined),
  );

  const indexText = pipe(
    Array.fromNullable(kindText),
    Array.appendAll(Array.fromNullable(jsonIndex?.toString())),
    Array.join(' '),
    (_) => _ || undefined,
  );

  const quantity = pipe(
    Match.value(jsonValue.kind),
    Match.when({ case: 'structValue' }, (_) => `${Record.size(_.value.fields)} keys`),
    Match.when({ case: 'listValue' }, (_) => `${_.value.values.length} entries`),
    Match.orElse(() => undefined),
  );

  const valueText = pipe(
    Match.value(jsonValue.kind),
    Match.when({ case: 'nullValue' }, () => 'null'),
    Match.whenOr({ case: 'boolValue' }, { case: 'numberValue' }, { case: 'stringValue' }, (_) => _.value.toString()),
    Match.orElse(() => undefined),
  );

  return (
    <TreeItem id={id} textValue={valueText ?? jsonKey ?? indexText ?? ''} {...(itemProps as TreeItemProps<object>)}>
      {jsonKey && <span className={tw`font-mono text-xs leading-5 text-red-700`}>{jsonKey}</span>}

      {indexText && (
        <span className={tw`rounded-sm bg-slate-200 px-2 py-0.5 text-xs font-medium tracking-tight text-slate-500`}>
          {indexText}
        </span>
      )}

      {quantity && <span className={tw`text-xs leading-5 font-medium tracking-tight text-slate-500`}>{quantity}</span>}

      {valueText && (
        <>
          <span className={tw`font-mono text-xs leading-5 text-slate-800`}>:</span>
          <span className={tw`flex-1 font-mono text-xs leading-5 break-all text-blue-700`}>{valueText}</span>
        </>
      )}
    </TreeItem>
  );
};
