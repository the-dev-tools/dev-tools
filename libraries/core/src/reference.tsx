import { fromJson, Message, toJson } from '@bufbuild/protobuf';
import { useSuspenseQuery as useConnectSuspenseQuery } from '@connectrpc/connect-query';
import { Array, Match, pipe } from 'effect';
import {
  Collection as AriaCollection,
  UNSTABLE_Tree as AriaTree,
  UNSTABLE_TreeItem as AriaTreeItem,
  UNSTABLE_TreeItemContent as AriaTreeItemContent,
} from 'react-aria-components';
import { LuChevronRight } from 'react-icons/lu';
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
import { tw } from '@the-dev-tools/ui/tailwind-literal';

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
      className={tw`flex flex-col gap-1`}
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

  const tags = pipe(Array.fromNullable(kindIndexTag), Array.appendAll(reference.variable));

  const quantity = pipe(
    Match.value(reference),
    Match.when({ kind: ReferenceKind.MAP }, (_) => `${_.map.length} keys`),
    Match.when({ kind: ReferenceKind.ARRAY }, (_) => `${_.array.length} entries`),
    Match.orElse(() => undefined),
  );

  return (
    <AriaTreeItem id={id} textValue={keyText ?? indexText ?? ''}>
      <AriaTreeItemContent>
        {({ level, isExpanded }) => (
          <div
            className={tw`flex cursor-pointer items-center gap-2`}
            style={{ marginInlineStart: (level - 1).toString() + 'rem' }}
          >
            {items && (
              <Button variant='ghost' slot='chevron'>
                <LuChevronRight
                  className={twJoin(tw`transition-transform`, !isExpanded ? tw`rotate-0` : tw`rotate-90`)}
                />
              </Button>
            )}

            {keyText && <span className={tw`font-mono text-red-700`}>{keyText}</span>}

            {tags.map((tag, index) => (
              <span key={index} className={tw`bg-gray-300 p-1`}>
                {tag}
              </span>
            ))}

            {quantity && <span className={tw`text-gray-700`}>{quantity}</span>}

            {reference.kind === ReferenceKind.VALUE && (
              <>
                {': '}
                <span className={tw`flex-1 break-all font-mono text-blue-700`}>{reference.value}</span>
              </>
            )}
          </div>
        )}
      </AriaTreeItemContent>

      {items && (
        <AriaCollection items={items}>
          {(_) => <ReferenceTreeItem id={makeId([...keys, _.key!])} reference={_} parentKeys={keys} />}
        </AriaCollection>
      )}
    </AriaTreeItem>
  );
};
