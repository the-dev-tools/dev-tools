import { Array, flow, Option, pipe } from 'effect';
import { DropEvent } from 'react-aria';
import { DropZone, DropZoneProps, FileTrigger, FileTriggerProps, Text } from 'react-aria-components';
import { FiFile } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { tv } from 'tailwind-variants';

import { Button } from './button';
import { isFocusedStyle, isFocusVisibleRingStyles } from './focus-ring';
import { CloudUploadIcon, DeleteIcon } from './icons';
import { MixinProps, splitProps } from './mixin-props';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV, formatSize  } from './utils';

const dropZoneStyles = tv({
  base: tw`flex min-h-40 flex-col items-center justify-center gap-2 rounded-md border border-dashed border-slate-300 bg-white p-4`,
  extend: isFocusVisibleRingStyles,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    isDropTarget: { true: twJoin(isFocusedStyle, tw`bg-violet-100`) },
  },
});

export interface FileDropZoneProps
  extends MixinProps<'dropZone', Omit<DropZoneProps, 'children'>>,
    Omit<FileTriggerProps, 'children'> {
  files?: File[] | undefined;
  onChange?: (files: File[] | undefined) => void;
}

export const FileDropZone = ({
  dropZoneClassName,
  dropZoneIsDisabled = false,
  dropZoneOnDrop,
  files,
  onChange,
  onSelect,
  ...mixProps
}: FileDropZoneProps) => {
  const props = splitProps(mixProps, 'dropZone');
  const { allowsMultiple = false } = props.rest;
  const hasFiles = files !== undefined && files.length > 0;

  const onDropChange =
    onChange &&
    ((event: DropEvent) =>
      void pipe(
        event.items,
        Array.filterMap(
          flow(
            Option.liftPredicate((_) => _.kind === 'file'),
            Option.map((_) => _.getFile()),
          ),
        ),
        (_) => Promise.all(_),
        (_) => _.then((_) => void onChange(_.length ? _ : undefined)),
      ));

  const onSelectChange = onChange && ((_: FileList | null) => void onChange(_?.length ? [..._] : undefined));

  return (
    <DropZone
      className={composeRenderPropsTV(dropZoneClassName, dropZoneStyles)}
      isDisabled={dropZoneIsDisabled || (hasFiles && !allowsMultiple)}
      onDrop={(dropZoneOnDrop ?? onDropChange)!}
      {...props.dropZone}
    >
      {hasFiles ? (
        <div className={tw`flex flex-wrap justify-around gap-4`}>
          {Array.fromIterable(files).map((file, index) => (
            <div
              className={tw`flex w-40 flex-col items-center`}
              key={index.toString() + file.name + file.size.toString()}
            >
              <div className={tw`mb-3 rounded-md border border-slate-200 bg-white p-1.5`}>
                <FiFile className={tw`size-5 text-slate-500`} />
              </div>

              <div
                className={tw`text-md w-full truncate text-center font-medium leading-5 tracking-tight text-slate-800`}
              >
                {file.name}
              </div>

              <div className={tw`text-xs leading-4 tracking-tight text-slate-500`}>{formatSize(file.size)}</div>

              {onChange && (
                <Button
                  className={tw`mt-1 p-1`}
                  onPress={() => {
                    const newFiles = Array.remove(files, index);
                    onChange(newFiles.length ? newFiles : undefined);
                  }}
                  variant='ghost'
                >
                  <DeleteIcon className={tw`size-4 text-rose-700`} />
                </Button>
              )}
            </div>
          ))}
        </div>
      ) : (
        <>
          <CloudUploadIcon className={tw`size-7 text-slate-500`} />

          <Text className={tw`mb-1 text-sm font-semibold leading-5 tracking-tight text-slate-800`} slot='label'>
            Drag and drop your files or
          </Text>

          <FileTrigger onSelect={(onSelect ?? onSelectChange)!} {...props.rest}>
            <Button>Browse Files</Button>
          </FileTrigger>
        </>
      )}
    </DropZone>
  );
};
