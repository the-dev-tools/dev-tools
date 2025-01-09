import { fromJson, Message, toJson } from '@bufbuild/protobuf';
import { useSuspenseQuery as useConnectSuspenseQuery } from '@connectrpc/connect-query';
import { Array, Match, pipe } from 'effect';
import {
  Collection as AriaCollection,
  UNSTABLE_Tree as AriaTree,
  UNSTABLE_TreeItemContent as AriaTreeItemContent,
} from 'react-aria-components';
import { twJoin } from 'tailwind-merge';

import {
  Reference,
  ReferenceGetRequest,
  ReferenceKey,
  ReferenceKeyJson,
  ReferenceKeyKind,
  ReferenceKeySchema,
  ReferenceKind,
} from '@the-dev-tools/spec/reference/v1/reference_pb';
import { referenceGet } from '@the-dev-tools/spec/reference/v1/reference-ReferenceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { ChevronSolidDownIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TreeItemRoot, TreeItemWrapper } from '@the-dev-tools/ui/tree';

const makeId = (keys: ReferenceKey[]) =>
  pipe(
    keys.map((_) => toJson(ReferenceKeySchema, _)),
    JSON.stringify,
  );

interface ReferenceTreeProps extends Partial<Omit<ReferenceGetRequest, keyof Message>> {
  onSelect?: (keys: ReferenceKey[]) => void;
}

export const ReferenceTree = ({ onSelect, ...props }: ReferenceTreeProps) => {
  const {
    data: { items },
  } = useConnectSuspenseQuery(referenceGet, props);

  return (
    <AriaTree
      aria-label='Reference Tree'
      items={items}
      onAction={(id) => {
        if (typeof id !== 'string') return;
        const keys = pipe(
          JSON.parse(id) as ReferenceKeyJson[],
          Array.map((_) => fromJson(ReferenceKeySchema, _)),
        );
        onSelect?.(keys);
      }}
    >
      {(_) => <ReferenceTreeItem id={makeId([_.key!])} reference={_} parentKeys={[]} />}
    </AriaTree>
  );
};

interface ReferenceTreeItemProps {
  id: string;
  reference: Reference;
  parentKeys: ReferenceKey[];
}

const ReferenceTreeItem = ({ id, reference, parentKeys }: ReferenceTreeItemProps) => {
  const key = reference.key!;
  const keys = [...parentKeys, key];

  const keyText = pipe(
    Match.value(key),
    Match.when({ kind: ReferenceKeyKind.GROUP }, (_) => _.group),
    Match.when({ kind: ReferenceKeyKind.KEY }, (_) => _.key),
    Match.orElse(() => undefined),
  );

  const items = pipe(
    Match.value(reference),
    Match.when({ kind: ReferenceKind.MAP }, (_) => _.map),
    Match.when({ kind: ReferenceKind.ARRAY }, (_) => _.array),
    Match.orElse(() => undefined),
  );

  const kindText = pipe(
    Match.value(reference),
    Match.when({ kind: ReferenceKind.MAP }, () => 'object'),
    Match.when({ kind: ReferenceKind.ARRAY }, () => 'array'),
    Match.orElse(() => undefined),
  );

  const indexText = pipe(
    Match.value(key),
    Match.when({ kind: ReferenceKeyKind.INDEX }, (_) => _.index.toString()),
    Match.when({ kind: ReferenceKeyKind.ANY }, () => 'any'),
    Match.orElse(() => undefined),
  );

  const kindIndexTag = pipe(
    Array.fromNullable(kindText),
    Array.appendAll(Array.fromNullable(indexText)),
    Array.join(' '),
    (_) => _ || undefined,
  );

  const tags = pipe(
    Array.fromNullable(kindIndexTag),
    Array.appendAll(reference.kind === ReferenceKind.VARIABLE ? reference.variable : []),
  );

  const quantity = pipe(
    Match.value(reference),
    Match.when({ kind: ReferenceKind.MAP }, (_) => `${_.map.length} keys`),
    Match.when({ kind: ReferenceKind.ARRAY }, (_) => `${_.array.length} entries`),
    Match.orElse(() => undefined),
  );

  return (
    <TreeItemRoot id={id} textValue={keyText ?? kindIndexTag ?? ''} className={tw`py-1`}>
      <AriaTreeItemContent>
        {({ level, isExpanded }) => (
          <TreeItemWrapper level={level} className={tw`gap-1`}>
            {items && (
              <Button variant='ghost' slot='chevron' className={tw`p-1`}>
                <ChevronSolidDownIcon
                  className={twJoin(
                    tw`size-3 text-slate-500 transition-transform`,
                    !isExpanded ? tw`rotate-0` : tw`rotate-90`,
                  )}
                />
              </Button>
            )}

            {key.kind === ReferenceKeyKind.GROUP && (
              <span className={tw`text-xs font-semibold leading-5 tracking-tight text-slate-800`}>{key.group}</span>
            )}

            {key.kind === ReferenceKeyKind.KEY && (
              <span className={tw`font-mono text-xs leading-5 text-red-700`}>{key.key}</span>
            )}

            {tags.map((tag, index) => (
              <span
                key={index}
                className={tw`rounded bg-slate-200 px-2 py-0.5 text-xs font-medium tracking-tight text-slate-500`}
              >
                {tag}
              </span>
            ))}

            {quantity && (
              <span className={tw`text-xs font-medium leading-5 tracking-tight text-slate-500`}>{quantity}</span>
            )}

            {reference.kind === ReferenceKind.VALUE && (
              <>
                <span className={tw`font-mono text-xs leading-5 text-slate-800`}>:</span>
                <span className={tw`flex-1 break-all font-mono text-xs leading-5 text-blue-700`}>
                  {reference.value}
                </span>
              </>
            )}
          </TreeItemWrapper>
        )}
      </AriaTreeItemContent>

      {items && (
        <AriaCollection items={items}>
          {(_) => <ReferenceTreeItem id={makeId([...keys, _.key!])} reference={_} parentKeys={keys} />}
        </AriaCollection>
      )}
    </TreeItemRoot>
  );
};
