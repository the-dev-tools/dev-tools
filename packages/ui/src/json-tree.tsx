import { Value } from '@bufbuild/protobuf/wkt';
import { Array, Match, pipe, Record, Tuple } from 'effect';
import { useState } from 'react';
import { Collection, Key, Tree, TreeItemContent } from 'react-aria-components';
import { twJoin } from 'tailwind-merge';
import { Button } from '@the-dev-tools/ui/button';
import { ChevronSolidDownIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TreeItemRoot, TreeItemWrapper } from '@the-dev-tools/ui/tree';

interface JsonTreeProps {
  value: Value;
}

export const JsonTree = ({ value }: JsonTreeProps) => (
  <Tree aria-label='JSON tree view'>
    <JsonTreeItem id='root' isRoot jsonValue={value} />
  </Tree>
);

interface JsonTreeItemProps {
  id: Key;
  isRoot?: boolean;
  jsonIndex?: number | undefined;
  jsonKey?: string | undefined;
  jsonValue: Value;
}

const JsonTreeItem = ({ id, isRoot, jsonIndex, jsonKey, jsonValue }: JsonTreeItemProps) => {
  const [isEnabled, setEnabled] = useState(false);

  const items = pipe(
    Match.value(jsonValue.kind),
    Match.when({ case: 'structValue' }, (_) => (
      <Collection items={Record.toEntries(_.value.fields)}>
        {([key, value]) => <JsonTreeItem id={`${id}.${key}`} jsonKey={key} jsonValue={value} />}
      </Collection>
    )),
    Match.when({ case: 'listValue' }, (_) => (
      <Collection items={pipe(_.value.values, Array.map(Tuple.make))}>
        {([value, index]) => <JsonTreeItem id={`${id}.${index}`} jsonIndex={index} jsonValue={value} />}
      </Collection>
    )),
    Match.orElse(() => undefined),
  );

  if (isRoot) return items;

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
    <TreeItemRoot className={tw`rounded-none py-1`} id={id} textValue={valueText ?? jsonKey ?? indexText ?? ''}>
      <TreeItemContent>
        {({ isExpanded, level }) => (
          <TreeItemWrapper className={tw`flex-wrap gap-1`} level={level}>
            {items && (
              <Button className={tw`p-1`} onPress={() => void setEnabled(true)} slot='chevron' variant='ghost'>
                <ChevronSolidDownIcon
                  className={twJoin(
                    tw`size-3 text-slate-500 transition-transform`,
                    !isExpanded ? tw`rotate-0` : tw`rotate-90`,
                  )}
                />
              </Button>
            )}

            {jsonKey && <span className={tw`font-mono text-xs leading-5 text-red-700`}>{jsonKey}</span>}

            {indexText && (
              <span
                className={tw`rounded-sm bg-slate-200 px-2 py-0.5 text-xs font-medium tracking-tight text-slate-500`}
              >
                {indexText}
              </span>
            )}

            {quantity && (
              <span className={tw`text-xs leading-5 font-medium tracking-tight text-slate-500`}>{quantity}</span>
            )}

            {valueText && (
              <>
                <span className={tw`font-mono text-xs leading-5 text-slate-800`}>:</span>
                <span className={tw`flex-1 font-mono text-xs leading-5 break-all text-blue-700`}>{valueText}</span>
              </>
            )}
          </TreeItemWrapper>
        )}
      </TreeItemContent>

      {isEnabled && items}
    </TreeItemRoot>
  );
};
