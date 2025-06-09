import { fromJson, Message, toJson } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import CodeMirror, { EditorView, ReactCodeMirrorProps, ReactCodeMirrorRef } from '@uiw/react-codemirror';
import { Array, Match, pipe, Struct } from 'effect';
import { createContext, RefAttributes, use, useContext } from 'react';
import { mergeProps } from 'react-aria';
import {
  Collection as AriaCollection,
  Tree as AriaTree,
  TreeItemContent as AriaTreeItemContent,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { twJoin } from 'tailwind-merge';
import { tv, VariantProps } from 'tailwind-variants';

import {
  ReferenceContext as ReferenceContextMessage,
  ReferenceKey,
  ReferenceKeyJson,
  ReferenceKeyKind,
  ReferenceKeySchema,
  ReferenceKind,
  ReferenceService,
  ReferenceTreeItem,
} from '@the-dev-tools/spec/reference/v1/reference_pb';
import { referenceTree } from '@the-dev-tools/spec/reference/v1/reference-ReferenceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { ChevronSolidDownIcon } from '@the-dev-tools/ui/icons';
import { controllerPropKeys, ControllerPropKeys } from '@the-dev-tools/ui/react-hook-form';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TreeItemRoot, TreeItemWrapper } from '@the-dev-tools/ui/tree';
import { useConnectSuspenseQuery } from '~/api/connect-query';
import { baseCodeMirrorExtensions } from '~code-mirror/extensions';
import { useReactRender } from '~react-render';

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

  // TODO: switch to Data Client Endpoint
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

const fieldStyles = tv({
  base: tw`text-md rounded-md border border-slate-200 px-3 py-0.5 text-slate-800`,
  variants: {
    variant: {
      'table-cell': tw`w-full min-w-0 rounded-none border-transparent px-5 py-0.5 -outline-offset-4`,
    },
  },
});

interface ReferenceFieldProps
  extends ReactCodeMirrorProps,
    RefAttributes<ReactCodeMirrorRef>,
    VariantProps<typeof fieldStyles> {
  allowFiles?: boolean | undefined;
}

export const ReferenceField = ({ allowFiles, className, extensions = [], ...forwardedProps }: ReferenceFieldProps) => {
  const props = Struct.omit(forwardedProps, ...fieldStyles.variantKeys);
  const variantProps = Struct.pick(forwardedProps, ...fieldStyles.variantKeys);

  const transport = useTransport();
  const client = createClient(ReferenceService, transport);

  const context = use(ReferenceContext);

  const reactRender = useReactRender();

  return (
    <CodeMirror
      basicSetup={false}
      className={fieldStyles({ className, ...variantProps })}
      extensions={[
        ...baseCodeMirrorExtensions({ allowFiles, client, context, reactRender }),
        EditorView.theme({ '.cm-scroller': { overflow: 'hidden' } }),
        ...extensions,
      ]}
      height='100%'
      indentWithTab={false}
      {...props}
    />
  );
};

interface ReferenceFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<ReferenceFieldProps, ControllerPropKeys>,
    UseControllerProps<TFieldValues, TName> {}

export const ReferenceFieldRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: ReferenceFieldRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const { field } = useController({ defaultValue: '' as never, ...controllerProps });

  const fieldProps: ReferenceFieldProps = {
    onBlur: field.onBlur,
    onChange: field.onChange,
    readOnly: field.disabled ?? false,
    value: field.value,
  };

  return <ReferenceField {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />;
};
