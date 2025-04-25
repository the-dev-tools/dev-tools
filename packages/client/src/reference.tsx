import { fromJson, Message, toJson } from '@bufbuild/protobuf';
import { Array, Match, pipe } from 'effect';
import { createContext, useContext } from 'react';
import {
  Collection as AriaCollection,
  Tree as AriaTree,
  TreeItemContent as AriaTreeItemContent,
} from 'react-aria-components';
import { twJoin } from 'tailwind-merge';

import {
  ReferenceContext as ReferenceContextMessage,
  ReferenceKey,
  ReferenceKeyJson,
  ReferenceKeyKind,
  ReferenceKeySchema,
  ReferenceKind,
  ReferenceTreeItem,
} from '@the-dev-tools/spec/reference/v1/reference_pb';
import { referenceTree } from '@the-dev-tools/spec/reference/v1/reference-ReferenceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { ChevronSolidDownIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TreeItemRoot, TreeItemWrapper } from '@the-dev-tools/ui/tree';
import { useConnectSuspenseQuery } from '~/api/connect-query';

export const makeReferenceTreeId = (keys: ReferenceKey[], value: unknown) =>
  pipe(
    keys.map((_) => toJson(ReferenceKeySchema, _)),
    (_) => JSON.stringify([_, value]),
  );

export interface ReferenceContextProps extends Partial<Omit<ReferenceContextMessage, keyof Message>> {}

export const ReferenceContext = createContext<ReferenceContextProps>({});

interface ReferenceTreeProps extends ReferenceContextProps {
  onSelect?: (keys: ReferenceKey[], value: unknown) => void;
}

export const ReferenceTree = ({ onSelect, ...props }: ReferenceTreeProps) => {
  const context = useContext(ReferenceContext);

  const {
    data: { items },
  } = useConnectSuspenseQuery(referenceTree, { ...props, ...context });

  return (
    <AriaTree
      aria-label='Reference Tree'
      items={items}
      onAction={(id) => {
        if (typeof id !== 'string') return;
        const [keysId, value] = JSON.parse(id) as [ReferenceKeyJson[], unknown];
        const keys = Array.map(keysId, (_) => fromJson(ReferenceKeySchema, _));
        onSelect?.(keys, value);
      }}
    >
      {(_) => <ReferenceTreeItemView id={makeReferenceTreeId([_.key!], _.value)} parentKeys={[]} reference={_} />}
    </AriaTree>
  );
};

const getGroupText = (key: ReferenceKey) =>
  pipe(
    Match.value(key),
    Match.when({ kind: ReferenceKeyKind.GROUP }, (_) => _.group),
    Match.when({ kind: ReferenceKeyKind.KEY }, (_) => _.key),
    Match.orElse(() => undefined),
  );

const getIndexText = (key: ReferenceKey) =>
  pipe(
    Match.value(key),
    Match.when({ kind: ReferenceKeyKind.INDEX }, (_) => _.index!.toString()),
    Match.when({ kind: ReferenceKeyKind.ANY }, () => 'any'),
    Match.orElse(() => undefined),
  );

interface ReferenceTreeItemProps {
  id: string;
  parentKeys: ReferenceKey[];
  reference: ReferenceTreeItem;
}

export const ReferenceTreeItemView = ({ id, parentKeys, reference }: ReferenceTreeItemProps) => {
  const key = reference.key!;
  const keys = [...parentKeys, key];

  const keyText = getGroupText(key);

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

  const indexText = getIndexText(key);

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
    <TreeItemRoot className={tw`rounded-none py-1`} id={id} textValue={keyText ?? kindIndexTag ?? ''}>
      <AriaTreeItemContent>
        {({ isExpanded, level }) => (
          <TreeItemWrapper className={tw`flex-wrap gap-1`} level={level}>
            {items && (
              <Button className={tw`p-1`} slot='chevron' variant='ghost'>
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
                className={tw`rounded-sm bg-slate-200 px-2 py-0.5 text-xs font-medium tracking-tight text-slate-500`}
                key={index}
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
          {(_) => (
            <ReferenceTreeItemView
              id={makeReferenceTreeId([...keys, _.key!], _.value)}
              parentKeys={keys}
              reference={_}
            />
          )}
        </AriaCollection>
      )}
    </TreeItemRoot>
  );
};
