import { create, MessageInitShape } from '@bufbuild/protobuf';
import { createCollection, localOnlyCollectionOptions, Query } from '@tanstack/react-db';
import { useNavigate, useRouter } from '@tanstack/react-router';
import { Array, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, useState, useTransition } from 'react';
import { Dialog, Heading, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiInfo, FiX } from 'react-icons/fi';
import {
  ImportDomainData,
  ImportDomainDataSchema,
  ImportMissingDataKind,
  ImportRequestSchema,
  ImportService,
} from '@the-dev-tools/spec/buf/api/import/v1/import_pb';
import { Button } from '@the-dev-tools/ui/button';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { FileDropZone } from '@the-dev-tools/ui/file-drop-zone';
import { FileImportIcon } from '@the-dev-tools/ui/icons';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { columnCheckboxField, columnText, columnTextField, ReactTableNoMemo } from '~/features/form-table';
import { request } from '~/shared/api';
import { eqStruct, LiveQuery, pickStruct } from '~/shared/lib';
import { routes } from '~/shared/routes';

export const useImportDialog = () => {
  const modal = useProgrammaticModal();

  const open = (): void =>
    void modal.onOpenChange(
      true,
      <InitialDialog
        setModal={(node) => void modal.onOpenChange(true, node)}
        successAction={() => Promise.resolve(void modal.onOpenChange(false))}
      />,
    );

  const render: ReactNode = modal.children && (
    <Modal {...modal} style={{ maxHeight: 'max(40vh, min(32rem, 90vh))', maxWidth: 'max(40vw, min(40rem, 90vw))' }} />
  );

  return { open, render };
};

export const ImportDialogTrigger = () => {
  const dialog = useImportDialog();

  return (
    <>
      <TooltipTrigger delay={750}>
        <Button className={tw`p-1`} onPress={() => void dialog.open()} variant='ghost'>
          <FileImportIcon className={tw`size-4 text-slate-500`} />
        </Button>
        <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
          Import Collections and Flows
        </Tooltip>
      </TooltipTrigger>

      {dialog.render}
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
  const { transport } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

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

    const { message: result } = await request({ input, method: ImportService.method.import, transport });

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
        Import Postman, HAR, Swagger or OpenAPI files, or paste a URL
      </div>

      <TextInputField
        className={tw`mt-4`}
        label='Text value'
        onChange={setText}
        placeholder='Paste cURL, Swagger/OpenAPI URL, or raw text...'
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
  const router = useRouter();
  const navigate = useNavigate();

  const { transport } = routes.root.useRouteContext();

  const [isPending, startTransition] = useTransition();

  const collection = createCollection(
    localOnlyCollectionOptions({
      getKey: (_) => _.domain,
      initialData: domains.map((_) => create(ImportDomainDataSchema, { domain: _, enabled: true })),
    }),
  );

  const baseQuery = (_: string) =>
    new Query()
      .from({ item: collection })
      .where(eqStruct({ domain: _ }))
      .findOne();

  const importAction = async () => {
    const {
      message: { flowId },
    } = await request({
      input: { ...input, domainData: Array.fromIterable(collection.values()) },
      method: ImportService.method.import,
      transport,
    });

    if (flowId) {
      await navigate({
        from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
        params: { flowIdCan: Ulid.construct(flowId).toCanonical() },
        to: router.routesById[routes.dashboard.workspace.flow.route.id].fullPath,
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
          columnCheckboxField<ImportDomainData>(
            'enabled',
            {
              onChange: (enabled, { row: { original } }) =>
                collection.update(original.domain, (_) => (_.enabled = enabled)),
              value: (provide, { row: { original } }) => (
                <LiveQuery query={() => baseQuery(original.domain).select(pickStruct('enabled'))}>
                  {(_) => provide(_.data?.enabled ?? false)}
                </LiveQuery>
              ),
            },
            { meta: { divider: false } },
          ),
          columnText<ImportDomainData>('domain', {
            cell: ({ row: { original } }) => original.domain,
            meta: { isRowHeader: true },
          }),
          columnTextField<ImportDomainData>('variable', {
            onChange: (variable, { row: { original } }) =>
              collection.update(original.domain, (_) => (_.variable = variable)),
            value: (provide, { row: { original } }) => (
              <LiveQuery query={() => baseQuery(original.domain).select(pickStruct('variable'))}>
                {(_) => provide(_.data?.variable ?? '')}
              </LiveQuery>
            ),
          }),
        ]}
        data={domains.map((_) => create(ImportDomainDataSchema, { domain: _ }))}
        getRowId={(_) => _.domain}
      >
        {(table) => <DataTable aria-label='Import domains' containerClassName={tw`mt-4`} table={table} />}
      </ReactTableNoMemo>
    </InnerDialog>
  );
};
