import { Array, flow, Option, pipe } from 'effect';
import { DropEvent } from 'react-aria';
import * as RAC from 'react-aria-components';
import { FiFile } from 'react-icons/fi';
import { Button } from './button';
import { focusVisibleRingStyles } from './focus-ring';
import { CloudUploadIcon, DeleteIcon } from './icons';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps, formatSize } from './utils';

export interface FileDropZoneProps
  extends Omit<RAC.FileTriggerProps, 'children'>, Pick<RAC.DropZoneProps, 'className' | 'isDisabled'> {
  files?: File[] | undefined;
  onChange?: (files: File[] | undefined) => void;
}

export const FileDropZone = ({
  allowsMultiple = false,
  className,
  files,
  isDisabled = false,
  onChange,
  onSelect,
  ...props
}: FileDropZoneProps) => {
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
    <RAC.DropZone
      className={composeTailwindRenderProps(
        className,
        focusVisibleRingStyles(),
        tw`
          flex min-h-40 flex-col items-center justify-center gap-2 rounded-md border border-dashed
          border-border-emphasis bg-surface p-4

          drop-target:bg-accent-soft drop-target:outline-4 drop-target:outline-ring
        `,
      )}
      isDisabled={isDisabled || (hasFiles && !allowsMultiple)}
      onDrop={onDropChange!}
    >
      {hasFiles ? (
        <div className={tw`flex flex-wrap justify-around gap-4`}>
          {Array.fromIterable(files).map((file, index) => (
            <FilePreview
              file={file}
              key={index.toString() + file.name + file.size.toString()}
              {...(onChange && {
                onRemove: () => {
                  const newFiles = Array.remove(files, index);
                  onChange(newFiles.length ? newFiles : undefined);
                },
              })}
            />
          ))}
        </div>
      ) : (
        <>
          <CloudUploadIcon className={tw`size-7 text-fg-muted`} />

          <RAC.Text className={tw`mb-1 text-sm leading-5 font-semibold tracking-tight text-fg`} slot='label'>
            Drag and drop your files or
          </RAC.Text>

          <RAC.FileTrigger onSelect={(onSelect ?? onSelectChange)!} {...props}>
            <Button>Browse Files</Button>
          </RAC.FileTrigger>
        </>
      )}
    </RAC.DropZone>
  );
};

interface FilePreviewProps {
  file: File;
  onRemove?: () => void;
}

const FilePreview = ({ file, onRemove }: FilePreviewProps) => (
  <div className={tw`flex w-40 flex-col items-center`}>
    <div className={tw`mb-3 rounded-md border border-border bg-surface p-1.5`}>
      <FiFile className={tw`size-5 text-fg-muted`} />
    </div>

    <div className={tw`w-full truncate text-center text-md leading-5 font-medium tracking-tight text-fg`}>
      {file.name}
    </div>

    <div className={tw`text-xs leading-4 tracking-tight text-fg-muted`}>{formatSize(file.size)}</div>

    {onRemove && (
      <Button className={tw`mt-1 p-1`} onPress={() => void onRemove()} variant='ghost'>
        <DeleteIcon className={tw`size-4 text-danger-soft-fg`} />
      </Button>
    )}
  </div>
);
