import { fromJson, Message, toJson } from '@bufbuild/protobuf';
import { Array, Match, pipe, Struct } from 'effect';
import { createContext, Suspense, useContext } from 'react';
import { mergeProps } from 'react-aria';
import {
  Collection as AriaCollection,
  UNSTABLE_Tree as AriaTree,
  UNSTABLE_TreeItemContent as AriaTreeItemContent,
  Dialog,
  DialogTrigger,
} from 'react-aria-components';
import { FieldPath, FieldValues, useController } from 'react-hook-form';
import { LuLink } from 'react-icons/lu';
import { twJoin } from 'tailwind-merge';

import { useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
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
import { Button, ButtonProps } from '@the-dev-tools/ui/button';
import { DropdownPopover, DropdownPopoverProps } from '@the-dev-tools/ui/dropdown';
import { ChevronSolidDownIcon } from '@the-dev-tools/ui/icons';
import { listBoxStyles } from '@the-dev-tools/ui/list-box';
import { controllerPropKeys } from '@the-dev-tools/ui/react-hook-form';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, TextFieldProps, TextFieldRHFProps } from '@the-dev-tools/ui/text-field';
import { TreeItemRoot, TreeItemWrapper } from '@the-dev-tools/ui/tree';
import { composeRenderPropsTW } from '@the-dev-tools/ui/utils';
import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

const makeId = (keys: ReferenceKey[]) =>
  pipe(
    keys.map((_) => toJson(ReferenceKeySchema, _)),
    JSON.stringify,
  );

export interface ReferenceContextProps extends Partial<Omit<ReferenceGetRequest, keyof Message>> {}

export const ReferenceContext = createContext<ReferenceContextProps>({});

interface ReferenceTreeProps extends ReferenceContextProps {
  onSelect?: (keys: ReferenceKey[]) => void;
}

export const ReferenceTree = ({ onSelect, ...props }: ReferenceTreeProps) => {
  const context = useContext(ReferenceContext);

  const {
    data: { items },
  } = useConnectSuspenseQuery(referenceGet, { ...props, ...context });

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
  reference: Reference;
  parentKeys: ReferenceKey[];
}

const ReferenceTreeItem = ({ id, reference, parentKeys }: ReferenceTreeItemProps) => {
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
    <TreeItemRoot id={id} textValue={keyText ?? kindIndexTag ?? ''} className={tw`rounded-none py-1`}>
      <AriaTreeItemContent>
        {({ level, isExpanded }) => (
          <TreeItemWrapper level={level} className={tw`flex-wrap gap-1`}>
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

interface ReferencePath {
  path: ReferenceKey[];
}

export const ReferencePath = ({ path }: ReferencePath) => {
  const keys = path.map((key, index) => {
    const indexText = getIndexText(key);

    if (indexText) {
      return (
        <span
          key={`${index} ${indexText}`}
          className={tw`mx-0.5 flex-none rounded bg-slate-200 px-2 py-0.5 text-xs font-medium tracking-tight text-slate-500`}
        >
          entry {indexText}
        </span>
      );
    }

    const keyText = getGroupText(key);

    if (keyText) {
      return (
        <span key={`${index} ${keyText}`} className={tw`flex-none text-md leading-5 tracking-tight text-slate-800`}>
          {keyText}
        </span>
      );
    }

    return null;
  });

  return <div className={tw`flex flex-wrap items-center`}>{Array.intersperse(keys, '.')}</div>;
};

interface ReferenceTreePopoverProps extends ReferenceTreeProps, MixinProps<'dropdown', DropdownPopoverProps> {}

const ReferenceTreePopover = ({ onSelect, ...mixProps }: ReferenceTreePopoverProps) => {
  const props = splitProps(mixProps, 'dropdown');

  return (
    <DropdownPopover {...props.dropdown}>
      <Dialog className={listBoxStyles({ className: tw`pointer-events-auto max-h-full w-96` })}>
        {({ close }) => (
          <Suspense fallback='Loading references...'>
            <ReferenceTree
              {...props.rest}
              onSelect={(keys) => {
                onSelect?.(keys);
                close();
              }}
            />
          </Suspense>
        )}
      </Dialog>
    </DropdownPopover>
  );
};

interface ReferenceFieldProps extends ReferenceTreeProps, MixinProps<'button', ButtonProps> {
  path: ReferenceKey[];
}

export const ReferenceField = ({ path, buttonClassName, ...mixProps }: ReferenceFieldProps) => {
  const props = splitProps(mixProps, 'button');

  return (
    <DialogTrigger>
      <Button {...props.button} className={composeRenderPropsTW(buttonClassName, tw`justify-start`)}>
        {path.length > 0 ? <ReferencePath path={path} /> : <span className={tw`p-1`}>Select reference</span>}
      </Button>
      <ReferenceTreePopover dropdownPlacement='bottom left' {...props.rest} />
    </DialogTrigger>
  );
};

interface TextFieldWithReferenceProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends TextFieldRHFProps<TFieldValues, TName> {
  context?: ReferenceContextProps;
}

export const TextFieldWithReference = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({
  context,
  ...props
}: TextFieldWithReferenceProps<TFieldValues, TName>) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const { field, fieldState } = useController({ defaultValue: '' as never, ...controllerProps });

  const fieldProps: TextFieldProps = {
    name: field.name,
    value: field.value,
    onChange: field.onChange,
    onBlur: field.onBlur,
    isDisabled: field.disabled ?? false,
    validationBehavior: 'aria',
    isInvalid: fieldState.invalid,
    error: fieldState.error?.message,
  };

  return (
    <div className='flex'>
      <TextField {...mergeProps(fieldProps, forwardedProps)} ref={field.ref} />
      <DialogTrigger>
        <Button variant='ghost'>
          <LuLink />
        </Button>
        <ReferenceTreePopover
          {...context}
          dropdownPlacement='bottom right'
          onSelect={(path) => {
            const pathString = pipe(
              path,
              Array.flatMapNullable((key) => getIndexText(key) ?? getGroupText(key)),
              Array.join('.'),
            );
            field.onChange(`{{ ${pathString} }}`);
          }}
        />
      </DialogTrigger>
    </div>
  );
};
