import { Array, pipe } from 'effect';
import { useState } from 'react';
import { Dialog, DialogTrigger, Key, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { TbFileExport } from 'react-icons/tb';
import { ExportService } from '@the-dev-tools/spec/buf/api/export/v1/export_pb';
import { FileCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import { Button } from '@the-dev-tools/ui/button';
import { Modal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { saveFile } from '@the-dev-tools/ui/utils';
import { FileTree } from '~/features/file-system';
import { useApiCollection, useConnectMutation } from '~/shared/api';
import { routes } from '~/shared/routes';

export const ExportDialog = () => {
  const fileCollection = useApiCollection(FileCollectionSchema);

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const [isOpen, setOpen] = useState(false);
  const [fileKeys, setFileKeys] = useState(new Set<Key>());

  const onOpenChange = (isOpen: boolean) => {
    setOpen(isOpen);
    if (!isOpen) return;
    setFileKeys(new Set());
  };

  const exportMutation = useConnectMutation(ExportService.method.export);

  return (
    <DialogTrigger isOpen={isOpen} onOpenChange={onOpenChange}>
      <TooltipTrigger delay={750}>
        <Button className={tw`p-1`} variant='ghost'>
          <TbFileExport className={tw`size-4 text-muted-foreground`} />
        </Button>
        <Tooltip className={tw`rounded-md bg-popover px-2 py-1 text-xs text-popover-foreground`}>Export Files</Tooltip>
      </TooltipTrigger>

      <Modal style={{ maxHeight: 'max(40vh, min(32rem, 90vh))', maxWidth: 'max(40vw, min(40rem, 90vw))' }}>
        <Dialog className={tw`flex h-full flex-col overflow-auto outline-hidden`}>
          <div className={tw`flex h-full min-h-0 flex-1 flex-col overflow-auto p-6`}>
            <div className={tw`flex items-center justify-between`}>
              <div className={tw`text-xl leading-6 font-semibold tracking-tighter text-foreground`}>Export Files</div>

              <Button className={tw`p-1`} onPress={() => void onOpenChange(false)} variant='ghost'>
                <FiX className={tw`size-5 text-muted-foreground`} />
              </Button>
            </div>

            <div className={tw`text-xs leading-5 tracking-tight text-muted-foreground`}>
              Please select the files that you would like to export.
            </div>

            <div className={tw`flex-1 overflow-auto py-1.5`}>
              <FileTree
                onSelectionChange={(selection) => {
                  if (selection === 'all') return;
                  setFileKeys(selection);
                }}
                selectedKeys={fileKeys}
                selectionMode='multiple'
              />
            </div>
          </div>

          <div className={tw`flex justify-end gap-2 border-t border-border px-6 py-3`}>
            <Button onPress={() => void onOpenChange(false)}>Cancel</Button>

            <Button
              isDisabled={fileKeys.size === 0}
              onPress={async () => {
                const fileIds = pipe(
                  Array.fromIterable(fileKeys),
                  Array.map((_) => fileCollection.utils.parseKeyUnsafe(_.toString()).fileId),
                );

                const { data, name } = await exportMutation.mutateAsync({ fileIds, workspaceId });
                saveFile({ blobParts: [data], name });
                onOpenChange(false);
              }}
              variant='primary'
            >
              Export
            </Button>
          </div>
        </Dialog>
      </Modal>
    </DialogTrigger>
  );
};
