import { fromJson, Message, toJson } from '@bufbuild/protobuf';
import { startCompletion } from '@codemirror/autocomplete';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import CodeMirror, { EditorView, ReactCodeMirrorProps, ReactCodeMirrorRef } from '@uiw/react-codemirror';
import { Array, Match, pipe, Struct } from 'effect';
import { createContext, RefAttributes, use, useContext, useRef } from 'react';
import { mergeProps } from 'react-aria';
import { Tree as AriaTree } from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
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
} from '@the-dev-tools/spec/buf/api/reference/v1/reference_pb';
import { controllerPropKeys, ControllerPropKeys } from '@the-dev-tools/ui/react-hook-form';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TreeItem } from '@the-dev-tools/ui/tree';
import { useConnectSuspenseQuery } from '~/shared/api';
import { useReactRender } from '~/shared/lib';
import { BaseCodeMirrorExtensionProps, baseCodeMirrorExtensions } from './code-mirror/extensions';

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
  } = useConnectSuspenseQuery(ReferenceService.method.referenceTree, { ...props, ...context });

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
    <TreeItem
      className={tw`rounded-none py-1`}
      id={id}
      item={(_) => (
        <ReferenceTreeItemView id={makeReferenceTreeId([...keys, _.key!], _.value)} parentKeys={keys} reference={_} />
      )}
      items={items!}
      textValue={keyText ?? kindIndexTag ?? ''}
    >
      {key.kind === ReferenceKeyKind.GROUP && (
        <span className={tw`text-xs leading-5 font-semibold tracking-tight text-slate-800`}>{key.group}</span>
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

      {quantity && <span className={tw`text-xs leading-5 font-medium tracking-tight text-slate-500`}>{quantity}</span>}

      {reference.kind === ReferenceKind.VALUE && (
        <>
          <span className={tw`font-mono text-xs leading-5 text-slate-800`}>:</span>
          <span className={tw`flex-1 font-mono text-xs leading-5 break-all text-blue-700`}>{reference.value}</span>
        </>
      )}
    </TreeItem>
  );
};

const fieldStyles = tv({
  base: tw`min-w-0 rounded-md border border-slate-200 px-3 py-0.5 text-md text-slate-800`,
  variants: {
    variant: {
      'table-cell': tw`w-full rounded-none border-transparent px-5 py-0.5 -outline-offset-4`,
    },
  },
});

interface ReferenceFieldProps
  extends
    Partial<BaseCodeMirrorExtensionProps>,
    ReactCodeMirrorProps,
    RefAttributes<ReactCodeMirrorRef>,
    VariantProps<typeof fieldStyles> {}

export const ReferenceField = ({
  allowFiles,
  kind,

  className,
  extensions = [],
  onFocus: onFocusParent,
  ref: refProp,
  ...forwardedProps
}: ReferenceFieldProps) => {
  const props = Struct.omit(forwardedProps, ...fieldStyles.variantKeys);
  const variantProps = Struct.pick(forwardedProps, ...fieldStyles.variantKeys);

  const transport = useTransport();
  const client = createClient(ReferenceService, transport);

  const ref = useRef<ReactCodeMirrorRef>(null);

  const onFocus: typeof onFocusParent = (event) => {
    onFocusParent?.(event);

    setTimeout(() => {
      if (!ref.current?.view) return;
      startCompletion(ref.current.view);
    }, 0);
  };

  const context = use(ReferenceContext);

  const reactRender = useReactRender();

  return (
    <CodeMirror
      basicSetup={false}
      className={fieldStyles({ className, ...variantProps })}
      extensions={[
        ...baseCodeMirrorExtensions({ allowFiles, client, context, kind, reactRender, singleLineMode: true }),
        EditorView.theme({ '.cm-scroller': { overflow: 'hidden' } }),
        ...extensions,
      ]}
      height='100%'
      indentWithTab={false}
      onFocus={onFocus}
      ref={(_) => {
        if (typeof refProp === 'function') refProp(_);
        else if (refProp) refProp.current = _;
        ref.current = _;
      }}
      {...props}
    />
  );
};

interface ReferenceFieldRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>
  extends Omit<ReferenceFieldProps, ControllerPropKeys>, UseControllerProps<TFieldValues, TName> {}

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
    ref: field.ref,
    value: field.value,
  };

  return <ReferenceField {...mergeProps(fieldProps, forwardedProps)} />;
};
