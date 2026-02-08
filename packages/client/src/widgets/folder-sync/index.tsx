import { ReactNode, useState, useTransition } from 'react';
import { Dialog, Heading, Label, Radio, RadioGroup } from 'react-aria-components';
import { FiFolder } from 'react-icons/fi';
import { Ulid } from 'id128';
import { WorkspaceCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/workspace';
import { Button } from '@the-dev-tools/ui/button';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { useApiCollection } from '~/shared/api';
import { getNextOrder } from '~/shared/lib';

type SyncFormat = 'openyaml' | 'bruno';

// --- Folder Sync Dialog (for existing workspaces) ---

interface FolderSyncDialogProps {
  workspaceId: Uint8Array;
  currentPath?: string;
  currentFormat?: string;
  currentEnabled?: boolean;
}

export const useFolderSyncDialog = () => {
  const modal = useProgrammaticModal();

  const open = (props: FolderSyncDialogProps): void =>
    void modal.onOpenChange(true, <FolderSyncDialogContent {...props} />);

  const render: ReactNode = modal.children && (
    <Modal {...modal} className={tw`h-auto`} size='sm' />
  );

  return { open, render };
};

const FolderSyncDialogContent = ({
  workspaceId,
  currentPath,
  currentFormat,
  currentEnabled,
}: FolderSyncDialogProps) => {
  const workspaceCollection = useApiCollection(WorkspaceCollectionSchema);

  const [folderPath, setFolderPath] = useState(currentPath ?? '');
  const [format, setFormat] = useState<SyncFormat>((currentFormat as SyncFormat) ?? 'openyaml');
  const [isPending, startTransition] = useTransition();

  const browseFolder = async () => {
    if (!window.electron?.dialog) return;
    const result = await window.electron.dialog('showOpenDialog', {
      properties: ['openDirectory'],
      title: 'Select folder to sync',
    });
    if (!result.canceled && result.filePaths[0]) {
      setFolderPath(result.filePaths[0]);
    }
  };

  const enableSync = () =>
    startTransition(async () => {
      await workspaceCollection.utils.update({
        workspaceId,
        syncPath: folderPath,
        syncFormat: format,
        syncEnabled: true,
      });
    });

  const disableSync = () =>
    startTransition(async () => {
      await workspaceCollection.utils.update({
        workspaceId,
        syncEnabled: false,
      });
    });

  return (
    <Dialog className={tw`flex flex-col p-5 outline-hidden`}>
      {({ close }) => (
        <>
          <Heading
            className={tw`text-base leading-5 font-semibold tracking-tight text-slate-800`}
            slot='title'
          >
            Folder Sync
          </Heading>

          <div className={tw`mt-3 flex flex-col gap-3`}>
            <div className={tw`flex items-end gap-2`}>
              <TextInputField
                aria-label='Folder path'
                className={tw`flex-1`}
                label='Folder Path'
                onChange={setFolderPath}
                placeholder='/path/to/your/collection'
                value={folderPath}
              />
              <Button onPress={() => void browseFolder()} variant='secondary'>
                <FiFolder className={tw`mr-1 size-4`} />
                Browse
              </Button>
            </div>

            <RadioGroup
              aria-label='Sync format'
              onChange={(v) => setFormat(v as SyncFormat)}
              value={format}
            >
              <Label className={tw`text-sm font-medium text-slate-700`}>Format</Label>
              <div className={tw`mt-1 flex gap-4`}>
                <Radio
                  className={tw`flex cursor-pointer items-center gap-2 text-sm text-slate-700`}
                  value='openyaml'
                >
                  <div
                    className={tw`size-4 rounded-full border-2 border-slate-300 data-[selected]:border-violet-600 data-[selected]:bg-violet-600`}
                  />
                  OpenYAML
                </Radio>
                <Radio
                  className={tw`flex cursor-pointer items-center gap-2 text-sm text-slate-700`}
                  value='bruno'
                >
                  <div
                    className={tw`size-4 rounded-full border-2 border-slate-300 data-[selected]:border-violet-600 data-[selected]:bg-violet-600`}
                  />
                  Bruno
                </Radio>
              </div>
            </RadioGroup>
          </div>

          <div className={tw`mt-5 flex justify-end gap-2`}>
            {currentEnabled && (
              <Button
                isPending={isPending}
                onPress={() => {
                  disableSync();
                  close();
                }}
                variant='danger'
              >
                Disable Sync
              </Button>
            )}
            <div className={tw`flex-1`} />
            <Button onPress={() => void close()}>Cancel</Button>
            <Button
              isDisabled={!folderPath}
              isPending={isPending}
              onPress={() => {
                enableSync();
                close();
              }}
              variant='primary'
            >
              {currentEnabled ? 'Update Sync' : 'Enable Sync'}
            </Button>
          </div>
        </>
      )}
    </Dialog>
  );
};

// --- Import from Folder Dialog (creates new workspace) ---

export const useImportFolderDialog = () => {
  const modal = useProgrammaticModal();

  const open = (): void =>
    void modal.onOpenChange(true, <ImportFolderDialogContent />);

  const render: ReactNode = modal.children && (
    <Modal {...modal} className={tw`h-auto`} size='sm' />
  );

  return { open, render };
};

const ImportFolderDialogContent = () => {
  const workspaceCollection = useApiCollection(WorkspaceCollectionSchema);

  const [folderPath, setFolderPath] = useState('');
  const [workspaceName, setWorkspaceName] = useState('');
  const [format, setFormat] = useState<SyncFormat>('openyaml');
  const [isPending, startTransition] = useTransition();

  const browseFolder = async () => {
    if (!window.electron?.dialog) return;
    const result = await window.electron.dialog('showOpenDialog', {
      properties: ['openDirectory'],
      title: 'Select collection folder',
    });
    if (!result.canceled && result.filePaths[0]) {
      const path = result.filePaths[0];
      setFolderPath(path);
      // Auto-fill name from folder name if empty
      if (!workspaceName) {
        const folderName = path.split('/').pop() ?? path.split('\\').pop() ?? '';
        setWorkspaceName(folderName);
      }
    }
  };

  const importFolder = () =>
    startTransition(async () => {
      const name = workspaceName || folderPath.split('/').pop() || 'Imported Workspace';
      await workspaceCollection.utils.insert({
        name,
        order: await getNextOrder(workspaceCollection),
        workspaceId: Ulid.generate().bytes,
        syncPath: folderPath,
        syncFormat: format,
        syncEnabled: true,
      });
    });

  return (
    <Dialog className={tw`flex flex-col p-5 outline-hidden`}>
      {({ close }) => (
        <>
          <Heading
            className={tw`text-base leading-5 font-semibold tracking-tight text-slate-800`}
            slot='title'
          >
            Import from Folder
          </Heading>

          <div className={tw`mt-1 text-sm leading-5 text-slate-500`}>
            Create a workspace synced to a local folder. Changes in the folder will automatically appear in DevTools.
          </div>

          <div className={tw`mt-4 flex flex-col gap-3`}>
            <div className={tw`flex items-end gap-2`}>
              <TextInputField
                aria-label='Folder path'
                className={tw`flex-1`}
                label='Folder Path'
                onChange={setFolderPath}
                placeholder='/path/to/your/collection'
                value={folderPath}
              />
              <Button onPress={() => void browseFolder()} variant='secondary'>
                <FiFolder className={tw`mr-1 size-4`} />
                Browse
              </Button>
            </div>

            <TextInputField
              aria-label='Workspace name'
              label='Workspace Name'
              onChange={setWorkspaceName}
              placeholder='My Collection'
              value={workspaceName}
            />

            <RadioGroup
              aria-label='Collection format'
              onChange={(v) => setFormat(v as SyncFormat)}
              value={format}
            >
              <Label className={tw`text-sm font-medium text-slate-700`}>Format</Label>
              <div className={tw`mt-1 flex gap-4`}>
                <Radio
                  className={tw`flex cursor-pointer items-center gap-2 text-sm text-slate-700`}
                  value='openyaml'
                >
                  <div
                    className={tw`size-4 rounded-full border-2 border-slate-300 data-[selected]:border-violet-600 data-[selected]:bg-violet-600`}
                  />
                  OpenYAML
                </Radio>
                <Radio
                  className={tw`flex cursor-pointer items-center gap-2 text-sm text-slate-700`}
                  value='bruno'
                >
                  <div
                    className={tw`size-4 rounded-full border-2 border-slate-300 data-[selected]:border-violet-600 data-[selected]:bg-violet-600`}
                  />
                  Bruno
                </Radio>
              </div>
            </RadioGroup>
          </div>

          <div className={tw`mt-5 flex justify-end gap-2`}>
            <Button onPress={() => void close()}>Cancel</Button>
            <Button
              isDisabled={!folderPath}
              isPending={isPending}
              onPress={() => {
                importFolder();
                close();
              }}
              variant='primary'
            >
              Import
            </Button>
          </div>
        </>
      )}
    </Dialog>
  );
};
