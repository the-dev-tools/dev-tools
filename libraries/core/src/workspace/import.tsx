import { useState } from 'react';
import { Dialog, DialogTrigger } from 'react-aria-components';
import { FiInfo, FiX } from 'react-icons/fi';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import { import$ } from '@the-dev-tools/spec/import/v1/import-ImportService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { FileDropZone } from '@the-dev-tools/ui/file-drop-zone';
import { FileImportIcon } from '@the-dev-tools/ui/icons';
import { Modal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

export const ImportDialog = () => {
  const [files, setFiles] = useState<File[]>();
  const importMutation = useConnectMutation(import$);

  return (
    <DialogTrigger>
      <Button className={tw`p-1`} variant='ghost'>
        <FileImportIcon className={tw`size-4 text-slate-500`} />
      </Button>

      <Modal modalSize='sm'>
        <Dialog className={tw`flex h-full flex-col outline-none`}>
          {({ close }) => {
            const cancel = () => {
              close();
              setFiles(undefined);
            };

            return (
              <>
                <div className={tw`flex h-full flex-col p-6`}>
                  <div className={tw`flex items-center justify-between`}>
                    <div className={tw`text-xl font-semibold leading-6 tracking-tighter text-slate-800`}>
                      Import Collections and Flows
                    </div>

                    <Button className={tw`p-1`} variant='ghost' onPress={() => void cancel()}>
                      <FiX className={tw`size-5 text-slate-500`} />
                    </Button>
                  </div>

                  {/* <div className={tw`text-xs leading-5 tracking-tight text-slate-500`}>
                    Lorem ipsum dolor sit amet consectur adipiscing elit.
                  </div> */}

                  <div
                    className={tw`mt-6 rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm font-medium leading-4 tracking-tight text-slate-500`}
                  >
                    <FiInfo className={tw`mr-1.5 inline-block size-4 align-bottom`} />
                    Import Postman or HAR files
                  </div>

                  {/* <TextField className={tw`mt-4`} inputPlaceholder='Paste cURL, Raw text or URL...' /> */}

                  <FileDropZone dropZoneClassName={tw`mt-4 flex-1`} onChange={setFiles} files={files} allowsMultiple />
                </div>

                <div className={tw`flex justify-end gap-2 border-t border-slate-200 px-6 py-3`}>
                  <Button onPress={() => void cancel()}>Cancel</Button>

                  <Button
                    variant='primary'
                    isDisabled={!files?.length}
                    onPress={async () => {
                      const file = files?.[0];
                      if (!file) return;
                      await importMutation.mutateAsync({ data: await file.bytes() });
                      cancel();
                    }}
                  >
                    Import
                  </Button>
                </div>
              </>
            );
          }}
        </Dialog>
      </Modal>
    </DialogTrigger>
  );
};
