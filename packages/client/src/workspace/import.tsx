import { create, MessageInitShape } from '@bufbuild/protobuf';
import { useNavigate } from '@tanstack/react-router';
import { Array, HashMap, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, useState, useTransition } from 'react';
import { Dialog, Heading, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiInfo, FiX } from 'react-icons/fi';
import {
  ImportDomainData,
  ImportDomainDataSchema,
  ImportMissingDataKind,
  ImportRequestSchema,
} from '@the-dev-tools/spec/import/v1/import_pb';
import { ExampleListEndpoint } from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.js';
import { CollectionItemListEndpoint } from '@the-dev-tools/spec/meta/collection/item/v1/item.endpoints.js';
import { CollectionListEndpoint } from '@the-dev-tools/spec/meta/collection/v1/collection.endpoints.js';
import { FlowListEndpoint } from '@the-dev-tools/spec/meta/flow/v1/flow.endpoints.js';
import { ImportEndpoint } from '@the-dev-tools/spec/meta/import/v1/import.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { FileDropZone } from '@the-dev-tools/ui/file-drop-zone';
import { FileImportIcon } from '@the-dev-tools/ui/icons';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { matchAllEndpoint, setQueryChild } from '~data-client';
import { columnCheckboxField, columnText, columnTextField, ReactTableNoMemo, useFormTable } from '~form-table';
import { flowLayoutRouteApi, rootRouteApi, workspaceRouteApi } from '~routes';

export const ImportDialog = () => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const modal = useProgrammaticModal();

  const successAction = async () => {
    modal.onOpenChange(false);
    await dataClient.controller.expireAll({
      testKey: (_) => {
        if (matchAllEndpoint(CollectionListEndpoint)(_)) return true;
        if (matchAllEndpoint(CollectionItemListEndpoint)(_)) return true;
        if (matchAllEndpoint(ExampleListEndpoint)(_)) return true;
        return false;
      },
    });
  };

  return (
    <>
      <TooltipTrigger delay={750}>
        <Button
          className={tw`p-1`}
          onPress={() =>
            void modal.onOpenChange(
              true,
              <InitialDialog setModal={(node) => void modal.onOpenChange(true, node)} successAction={successAction} />,
            )
          }
          variant='ghost'
        >
          <FileImportIcon className={tw`size-4 text-slate-500`} />
        </Button>
        <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
          Import Collections and Flows
        </Tooltip>
      </TooltipTrigger>

      {modal.children && (
        <Modal
          {...modal}
          style={{ maxHeight: 'max(40vh, min(32rem, 90vh))', maxWidth: 'max(40vw, min(40rem, 90vw))' }}
        />
      )}
    </>
  );
};

interface InnerDialogProps {
  action: ReactNode;
  children: ReactNode;
}

const InnerDialog = ({ action, children }: InnerDialogProps) => (
  <Dialog className={tw`flex h-full flex-col overflow-auto outline-hidden`}>
    {({ close }) => (
      <>
        <div className={tw`flex h-full min-h-0 flex-1 flex-col overflow-auto p-6`}>
          <div className={tw`flex items-center justify-between`}>
            <Heading className={tw`text-xl leading-6 font-semibold tracking-tighter text-slate-800`} slot='title'>
              Import Collections and Flows
            </Heading>

            <Button className={tw`p-1`} onPress={() => void close()} variant='ghost'>
              <FiX className={tw`size-5 text-slate-500`} />
            </Button>
          </div>

          {children}
        </div>

        <div className={tw`flex justify-end gap-2 border-t border-slate-200 px-6 py-3`}>
          <Button onPress={() => void close()}>Cancel</Button>

          {action}
        </div>
      </>
    )}
  </Dialog>
);

interface InitialDialogProps {
  setModal: (node: ReactNode) => void;
  successAction: () => Promise<void>;
}

const InitialDialog = ({ setModal, successAction }: InitialDialogProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const [text, setText] = useState('');
  const [files, setFiles] = useState<File[]>();
  const file = files?.[0];
  const [isPending, startTransition] = useTransition();

  const data = pipe(
    Option.fromNullable(files?.[0]),
    Option.map((_) => _.arrayBuffer().then((_) => new Uint8Array(_))),
    Option.getOrElse(() => Promise.resolve(undefined)),
  );

  const importAction = async () => {
    const input: MessageInitShape<typeof ImportRequestSchema> = {
      data: (await data) ?? new Uint8Array(),
      name: file?.name ?? '',
      textData: text,
      workspaceId,
    };

    const result = await dataClient.fetch(ImportEndpoint, input);

    if (result.missingData === ImportMissingDataKind.DOMAIN)
      setModal(<DomainDialog domains={result.domains} input={input} successAction={successAction} />);
    else await successAction();
  };

  return (
    <InnerDialog
      action={
        <Button
          isDisabled={!files?.length && !text}
          isPending={isPending}
          onPress={() => void startTransition(importAction)}
          variant='primary'
        >
          Import
        </Button>
      }
    >
      <div
        className={tw`
          mt-6 rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm leading-4 font-medium tracking-tight
          text-slate-500
        `}
      >
        <FiInfo className={tw`mr-1.5 inline-block size-4 align-bottom`} />
        Import Postman or HAR files
      </div>

      <TextInputField
        className={tw`mt-4`}
        label='Text value'
        onChange={setText}
        placeholder='Paste cURL, Raw text or URL...'
        value={text}
      />

      <FileDropZone className={tw`mt-4 flex-1`} files={files} onChange={setFiles} />
    </InnerDialog>
  );
};

interface DomainDialogProps {
  domains: string[];
  input: MessageInitShape<typeof ImportRequestSchema>;
  successAction: () => Promise<void>;
}

const DomainDialog = ({ domains, input, successAction }: DomainDialogProps) => {
  const { dataClient, transport } = rootRouteApi.useRouteContext();
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const navigate = useNavigate();

  const [isPending, startTransition] = useTransition();
  const [domainData, setDomainData] = useState(
    pipe(
      Array.map(domains, (_) => [_, create(ImportDomainDataSchema, { domain: _, enabled: true })] as const),
      HashMap.fromIterable,
    ),
  );

  const formTable = useFormTable<ImportDomainData>({
    onUpdate: (_) => void setDomainData(HashMap.modify(_.domain, () => _)),
  });

  const importAction = async () => {
    const { flow } = await dataClient.fetch(ImportEndpoint, { ...input, domainData: HashMap.toValues(domainData) });

    if (flow) {
      await setQueryChild(
        dataClient.controller,
        FlowListEndpoint.schema.items,
        'push',
        { controller: () => dataClient.controller, input: { workspaceId }, transport },
        flow,
      );

      const flowIdCan = Ulid.construct(flow.flowId).toCanonical();

      await navigate({
        from: workspaceRouteApi.id,
        to: flowLayoutRouteApi.id,

        params: { flowIdCan },
      });
    }

    await successAction();
  };

  return (
    <InnerDialog
      action={
        <Button isPending={isPending} onPress={() => void startTransition(importAction)} variant='primary'>
          Import
        </Button>
      }
    >
      <div className={tw`text-xs leading-5 tracking-tight text-slate-500`}>
        Please deselect the domain names to be excluded in the flow. There might be requests that you may not want to
        import.
      </div>

      <ReactTableNoMemo
        columns={[
          columnCheckboxField<ImportDomainData>('enabled', { meta: { divider: false } }),
          columnText<ImportDomainData>('domain', { meta: { isRowHeader: true } }),
          columnTextField<ImportDomainData>('variable'),
        ]}
        data={HashMap.toValues(domainData)}
        getRowId={(_) => _.domain}
      >
        {(table) => (
          <DataTable {...formTable} aria-label='Import domains' containerClassName={tw`mt-4`} table={table} />
        )}
      </ReactTableNoMemo>
    </InnerDialog>
  );
};
