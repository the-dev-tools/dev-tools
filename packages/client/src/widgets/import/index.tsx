import { create, MessageInitShape } from '@bufbuild/protobuf';
import { createCollection, localOnlyCollectionOptions, Query } from '@tanstack/react-db';
import { useNavigate, useRouter } from '@tanstack/react-router';
import { Array, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, useState, useTransition } from 'react';
import { Dialog, Heading, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiInfo, FiX } from 'react-icons/fi';
import {
  ImportDomainDataSchema,
  ImportMissingDataKind,
  ImportRequestSchema,
  ImportService,
} from '@the-dev-tools/spec/buf/api/import/v1/import_pb';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { FileDropZone } from '@the-dev-tools/ui/file-drop-zone';
import { FileImportIcon } from '@the-dev-tools/ui/icons';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from '@the-dev-tools/ui/table';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
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
          <FileImportIcon className={tw`size-4 text-on-neutral-low`} />
        </Button>
        <Tooltip className={tw`rounded-md bg-inverse px-2 py-1 text-xs text-on-inverse`}>
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
            <Heading className={tw`text-xl leading-6 font-semibold tracking-tighter text-on-neutral`} slot='title'>
              Import Collections and Flows
            </Heading>

            <Button className={tw`p-1`} onPress={() => void close()} variant='ghost'>
              <FiX className={tw`size-5 text-on-neutral-low`} />
            </Button>
          </div>

          {children}
        </div>

        <div className={tw`flex justify-end gap-2 border-t border-neutral px-6 py-3`}>
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
          mt-6 rounded-lg border border-neutral bg-neutral-lower p-4 text-sm leading-4 font-medium tracking-tight
          text-on-neutral-low
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
      <div className={tw`text-xs leading-5 tracking-tight text-on-neutral-low`}>
        Please deselect the domain names to be excluded in the flow. There might be requests that you may not want to
        import.
      </div>

      <Table aria-label='Import domains' containerClassName={tw`mt-4`}>
        <TableHeader>
          <TableColumn width={32} />
          <TableColumn isRowHeader>Domain</TableColumn>
          <TableColumn>Variable</TableColumn>
        </TableHeader>

        <TableBody items={domains.map((_) => ({ domain: _ }))}>
          {({ domain }) => {
            const query = new Query().from({ item: collection }).where(eqStruct({ domain })).findOne();

            return (
              <TableRow id={domain}>
                <TableCell className={tw`border-r-0`}>
                  <LiveQuery query={() => query.select(pickStruct('enabled'))}>
                    {({ data }) => (
                      <Checkbox
                        aria-label='Enabled'
                        isSelected={data?.enabled ?? false}
                        isTableCell
                        onChange={(_) => void collection.update(domain, (draft) => (draft.enabled = _))}
                      />
                    )}
                  </LiveQuery>
                </TableCell>

                <TableCell className={tw`px-5 py-1.5`}>{domain}</TableCell>

                <TableCell>
                  <LiveQuery query={() => query.select(pickStruct('variable'))}>
                    {({ data }) => (
                      <TextInputField
                        aria-label='Variable'
                        className='flex-1'
                        isTableCell
                        onChange={(_) => void collection.update(domain, (draft) => (draft.variable = _))}
                        placeholder={`Enter variable`}
                        value={data?.variable ?? ''}
                      />
                    )}
                  </LiveQuery>
                </TableCell>
              </TableRow>
            );
          }}
        </TableBody>
      </Table>
    </InnerDialog>
  );
};
