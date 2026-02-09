import { create } from '@bufbuild/protobuf';
import { and, createLiveQueryCollection, eq, isUndefined, useLiveQuery } from '@tanstack/react-db';
import { useRouter } from '@tanstack/react-router';
import * as XF from '@xyflow/react';
import { Array, HashMap, pipe } from 'effect';
import { Ulid } from 'id128';
import { use, useState } from 'react';
import * as RAC from 'react-aria-components';
import { FiExternalLink, FiPlus } from 'react-icons/fi';
import { RiAnthropicFill, RiGeminiFill, RiOpenaiFill } from 'react-icons/ri';
import { TbFile, TbRobotFace } from 'react-icons/tb';
import { CredentialKind } from '@the-dev-tools/spec/buf/api/credential/v1/credential_pb';
import { FileKind } from '@the-dev-tools/spec/buf/api/file_system/v1/file_system_pb';
import {
  AiMemoryType,
  AiModel,
  HandleKind,
  NodeAiMemorySchema,
  NodeAiProviderSchema,
  NodeAiProviderUpdate_MaxTokensUnion_Kind,
  NodeAiProviderUpdate_TemperatureUnion_Kind,
  NodeAiSchema,
  NodeKind,
} from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { Unset } from '@the-dev-tools/spec/buf/global/v1/global_pb';
import { CredentialCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/credential';
import { FileCollectionSchema, FolderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import {
  NodeAiCollectionSchema,
  NodeAiMemoryCollectionSchema,
  NodeAiProviderCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button, ButtonAsRouteLink } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { listBoxStyles } from '@the-dev-tools/ui/list-box';
import { NumberField } from '@the-dev-tools/ui/number-field';
import { Popover } from '@the-dev-tools/ui/popover';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ReferenceField } from '~/features/expression';
import { FileTree } from '~/features/file-system';
import { useApiCollection } from '~/shared/api';
import { eqStruct, getNextOrder, pick, queryCollection } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { AddNodeSidebarProps, SidebarHeader, SidebarItem, useInsertNode } from '../add-node';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsContainer, NodeSettingsProps, NodeTitle, SimpleNode } from '../node';

export const AiNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`w-48 text-purple-500`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
          <Handle
            className={tw`right-auto left-6`}
            kind={HandleKind.AI_PROVIDER}
            nodeId={nodeId}
            nodeOffset={{ x: -100 }}
            position={XF.Position.Bottom}
            Sidebar={AiProviderSidebar}
            type='source'
          />
          <Handle
            kind={HandleKind.AI_MEMORY}
            nodeId={nodeId}
            nodeOffset={{ x: 0 }}
            position={XF.Position.Bottom}
            Sidebar={AiMemorySidebar}
            type='source'
          />
          <Handle
            alwaysVisible
            className={tw`right-6 left-auto`}
            kind={HandleKind.AI_TOOLS}
            nodeId={nodeId}
            nodeOffset={{ x: 200 }}
            position={XF.Position.Bottom}
            type='source'
          />
        </>
      }
      icon={<TbRobotFace />}
      nodeId={nodeId}
      selected={selected}
    >
      <NodeTitle className={tw`text-left`}>AI Agent</NodeTitle>
    </SimpleNode>
  );
};

export const AiSettings = ({ nodeId }: NodeSettingsProps) => {
  const collection = useApiCollection(NodeAiCollectionSchema);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ nodeId }))
          .select((_) => pick(_.item, 'prompt', 'maxIterations'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeAiSchema);

  const { isReadOnly = false } = use(FlowContext);

  return (
    <NodeSettingsBody nodeId={nodeId} title='AI Agent'>
      <div className={tw`flex flex-col gap-y-5`}>
        <div>
          <FieldLabel>Prompt</FieldLabel>
          <ReferenceField
            className={tw`h-64`}
            kind='StringExpression'
            onChange={(_) => collection.utils.updatePaced({ nodeId, prompt: _ })}
            readOnly={isReadOnly}
            singleLineMode={false}
            value={data.prompt}
          />
        </div>

        <NumberField
          isReadOnly={isReadOnly}
          label='Max Iterations'
          onChange={(_) => collection.utils.updatePaced({ maxIterations: _, nodeId })}
          value={data.maxIterations}
        />
      </div>
    </NodeSettingsBody>
  );
};

const modelProviderMap = pipe(
  [
    {
      credentialKind: CredentialKind.ANTHROPIC,
      credentialKindTitle: 'Anthropic',
      icon: <RiAnthropicFill />,
      model: AiModel.CLAUDE_HAIKU45,
      title: 'Claude Haiku 4.5',
    },
    {
      credentialKind: CredentialKind.ANTHROPIC,
      credentialKindTitle: 'Anthropic',
      icon: <RiAnthropicFill />,
      model: AiModel.CLAUDE_OPUS45,
      title: 'Claude Opus 4.5',
    },
    {
      credentialKind: CredentialKind.ANTHROPIC,
      credentialKindTitle: 'Anthropic',
      icon: <RiAnthropicFill />,
      model: AiModel.CLAUDE_SONNET45,
      title: 'Claude Sonnet 4.5',
    },
    {
      credentialKind: CredentialKind.UNSPECIFIED,
      credentialKindTitle: 'N/A',
      icon: <TbRobotFace />,
      model: AiModel.CUSTOM,
      title: 'Custom',
    },
    {
      credentialKind: CredentialKind.GEMINI,
      credentialKindTitle: 'Gemini',
      icon: <RiGeminiFill />,
      model: AiModel.GEMINI3_FLASH,
      title: 'Gemini 3 Flash',
    },
    {
      credentialKind: CredentialKind.GEMINI,
      credentialKindTitle: 'Gemini',
      icon: <RiGeminiFill />,
      model: AiModel.GEMINI3_PRO,
      title: 'Gemini 3 Pro',
    },
    {
      credentialKind: CredentialKind.OPEN_AI,
      credentialKindTitle: 'OpenAI',
      icon: <RiOpenaiFill />,
      model: AiModel.GPT52_CODEX,
      title: 'GPT-5.2 Codex',
    },
    {
      credentialKind: CredentialKind.OPEN_AI,
      credentialKindTitle: 'OpenAI',
      icon: <RiOpenaiFill />,
      model: AiModel.GPT52,
      title: 'GPT-5.2',
    },
    {
      credentialKind: CredentialKind.OPEN_AI,
      credentialKindTitle: 'OpenAI',
      icon: <RiOpenaiFill />,
      model: AiModel.GPT52_PRO,
      title: 'GPT-5.2 Pro',
    },
    {
      credentialKind: CredentialKind.OPEN_AI,
      credentialKindTitle: 'OpenAI',
      icon: <RiOpenaiFill />,
      model: AiModel.O3,
      title: 'OpenAI o3',
    },
    {
      credentialKind: CredentialKind.OPEN_AI,
      credentialKindTitle: 'OpenAI',
      icon: <RiOpenaiFill />,
      model: AiModel.O4_MINI,
      title: 'OpenAI o4-mini',
    },
    {
      credentialKind: CredentialKind.UNSPECIFIED,
      credentialKindTitle: 'unspecified',
      icon: <TbRobotFace />,
      model: AiModel.UNSPECIFIED,
      title: 'N/A',
    },
  ],
  Array.map(({ model, ...info }) => [model, info] as const),
  HashMap.fromIterable,
);

export const AiProviderSidebar = ({ handleKind, position, sourceId, targetId }: AddNodeSidebarProps) => {
  const insertNode = useInsertNode();

  const collection = useApiCollection(NodeAiProviderCollectionSchema);

  return (
    <>
      <SidebarHeader title='AI Provider' />

      <RAC.ListBox aria-label='AI Providers' className={tw`mt-3`}>
        {pipe(
          HashMap.remove(modelProviderMap, AiModel.UNSPECIFIED),
          HashMap.map(({ icon, title }, model) => (
            <SidebarItem
              icon={icon}
              key={model}
              onAction={() => {
                const nodeId = Ulid.generate().bytes;
                collection.utils.insert({ model, nodeId });
                insertNode({
                  handleKind,
                  kind: NodeKind.AI_PROVIDER,
                  name: 'ai-provider',
                  nodeId,
                  position,
                  sourceId,
                  targetId,
                });
              }}
              title={title}
            />
          )),
          HashMap.values,
        )}
      </RAC.ListBox>
    </>
  );
};

export const AiProviderNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  const collection = useApiCollection(NodeAiProviderCollectionSchema);

  const { model } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ nodeId }))
          .select((_) => pick(_.item, 'model'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeAiProviderSchema);

  const { icon, title } = HashMap.unsafeGet(modelProviderMap, model);

  return (
    <SimpleNode
      className={tw`rounded-full text-sky-500`}
      handles={<Handle nodeId={nodeId} position={XF.Position.Top} type='target' />}
      icon={icon}
      nodeId={nodeId}
      selected={selected}
      title={title}
    />
  );
};

export const AiProviderSettings = ({ nodeId }: NodeSettingsProps) => {
  const router = useRouter();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const providerCollection = useApiCollection(NodeAiProviderCollectionSchema);
  const fileCollection = useApiCollection(FileCollectionSchema);
  const folderCollection = useApiCollection(FolderCollectionSchema);
  const credentialCollection = useApiCollection(CredentialCollectionSchema);

  const { isReadOnly = false } = use(FlowContext);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: providerCollection })
          .where(eqStruct({ nodeId }))
          .select((_) => pick(_.item, 'model', 'credentialId', 'maxTokens', 'temperature'))
          .findOne(),
      [providerCollection, nodeId],
    ).data ?? create(NodeAiProviderSchema);

  const { credentialKind, credentialKindTitle, title } = HashMap.unsafeGet(modelProviderMap, data.model);

  const credential = useLiveQuery(
    (_) =>
      _.from({ item: credentialCollection })
        .where(eqStruct({ credentialId: data.credentialId }))
        .select((_) => pick(_.item, 'name'))
        .findOne(),
    [credentialCollection, data.credentialId],
  ).data;

  const [credentialIsOpen, setCredentialIsOpen] = useState(false);

  return (
    <NodeSettingsBody nodeId={nodeId} title={title}>
      <div className={tw`flex flex-col items-start gap-y-5`}>
        {credentialKind !== CredentialKind.UNSPECIFIED && (
          <div>
            <FieldLabel>Credential</FieldLabel>

            <RAC.DialogTrigger isOpen={credentialIsOpen} onOpenChange={setCredentialIsOpen}>
              <div className={tw`flex gap-2`}>
                <Button>{credential ? credential.name : 'Select file'}</Button>

                {credential && (
                  <ButtonAsRouteLink
                    from={router.routesById[routes.dashboard.workspace.route.id].fullPath}
                    params={{ credentialIdCan: Ulid.construct(data.credentialId).toCanonical() }}
                    to={router.routesById[routes.dashboard.workspace.credential.id].fullPath}
                  >
                    <FiExternalLink className={tw`size-4 text-muted-foreground`} />
                    Open
                  </ButtonAsRouteLink>
                )}
              </div>

              <Popover className={listBoxStyles({ className: tw`max-w-2xs` })} placement='bottom left'>
                <Button
                  className={tw`justify-start gap-3 px-3`}
                  onPress={async () => {
                    let [{ credentialFolderId } = {}] = await queryCollection((_) => {
                      const file = createLiveQueryCollection((_) =>
                        _.from({ file: fileCollection })
                          .where((_) =>
                            and(
                              eq(_.file.workspaceId, workspaceId),
                              eq(_.file.kind, FileKind.FOLDER),
                              isUndefined(_.file.parentId),
                            ),
                          )
                          .fn.select((_) => ({
                            fileId: _.file.fileId,
                            id: Ulid.construct(_.file.fileId).toCanonical(),
                          })),
                      );

                      const folder = createLiveQueryCollection((_) =>
                        _.from({ folder: folderCollection })
                          .where((_) => eq(_.folder.name, 'Credentials'))
                          .fn.select((_) => ({ id: Ulid.construct(_.folder.folderId).toCanonical() })),
                      );

                      return _.from({ file })
                        .join({ folder }, (_) => eq(_.file.id, _.folder.id), 'inner')
                        .select((_) => ({ credentialFolderId: _.file.fileId }));
                    });

                    if (!credentialFolderId) {
                      credentialFolderId = Ulid.generate().bytes;

                      folderCollection.utils.insert({
                        folderId: credentialFolderId,
                        name: 'Credentials',
                      });
                    }

                    const credentialId = Ulid.generate().bytes;

                    credentialCollection.utils.insert({
                      credentialId,
                      kind: credentialKind,
                      name: `${credentialKindTitle} credential`,
                      workspaceId,
                    });

                    fileCollection.utils.insert({
                      fileId: credentialId,
                      kind: FileKind.CREDENTIAL,
                      order: await getNextOrder(fileCollection),
                      parentId: credentialFolderId,
                      workspaceId,
                    });

                    providerCollection.utils.update({ credentialId, nodeId });
                    setCredentialIsOpen(false);
                  }}
                  variant='ghost'
                >
                  <FiPlus className={tw`size-4 text-muted-foreground`} />
                  New {credentialKindTitle} credential
                </Button>

                <FileTree
                  kind={FileKind.CREDENTIAL}
                  onAction={(key) => {
                    const file = fileCollection.get(key.toString())!;
                    if (file.kind !== FileKind.CREDENTIAL) return;

                    const credential = credentialCollection.get(
                      credentialCollection.utils.getKey({ credentialId: file.fileId }),
                    );

                    if (credential?.kind !== credentialKind) return;

                    providerCollection.utils.update({ credentialId: credential.credentialId, nodeId });
                    setCredentialIsOpen(false);
                  }}
                  showControls
                />
              </Popover>
            </RAC.DialogTrigger>
          </div>
        )}

        <NumberField
          isReadOnly={isReadOnly}
          label='Max Tokens'
          onChange={(_) =>
            providerCollection.utils.updatePaced({
              maxTokens: _
                ? { kind: NodeAiProviderUpdate_MaxTokensUnion_Kind.VALUE, value: _ }
                : { kind: NodeAiProviderUpdate_MaxTokensUnion_Kind.UNSET, unset: Unset.UNSET },
              nodeId,
            })
          }
          value={data.maxTokens ?? 0}
        />

        <NumberField
          isReadOnly={isReadOnly}
          label='Temperature'
          onChange={(_) =>
            providerCollection.utils.updatePaced({
              nodeId,
              temperature: _
                ? { kind: NodeAiProviderUpdate_TemperatureUnion_Kind.VALUE, value: _ }
                : { kind: NodeAiProviderUpdate_TemperatureUnion_Kind.UNSET, unset: Unset.UNSET },
            })
          }
          value={data.temperature ?? 0}
        />
      </div>
    </NodeSettingsBody>
  );
};

const memoryProviderMap = pipe(
  [
    {
      icon: <TbFile />,
      title: 'Window Buffer',
      type: AiMemoryType.WINDOW_BUFFER,
    },
    {
      icon: <TbFile />,
      title: 'N/A',
      type: AiMemoryType.UNSPECIFIED,
    },
  ],
  Array.map(({ type, ...info }) => [type, info] as const),
  HashMap.fromIterable,
);

export const AiMemorySidebar = ({ handleKind, position, sourceId, targetId }: AddNodeSidebarProps) => {
  const insertNode = useInsertNode();

  const collection = useApiCollection(NodeAiMemoryCollectionSchema);

  return (
    <>
      <SidebarHeader title='AI Memory' />

      <RAC.ListBox aria-label='AI Memory' className={tw`mt-3`}>
        {pipe(
          HashMap.remove(memoryProviderMap, AiMemoryType.UNSPECIFIED),
          HashMap.map(({ icon, title }, memoryType) => (
            <SidebarItem
              icon={icon}
              key={memoryType}
              onAction={() => {
                const nodeId = Ulid.generate().bytes;
                collection.utils.insert({ memoryType, nodeId });
                insertNode({
                  handleKind,
                  kind: NodeKind.AI_MEMORY,
                  name: 'ai-memory',
                  nodeId,
                  position,
                  sourceId,
                  targetId,
                });
              }}
              title={title}
            />
          )),
          HashMap.values,
        )}
      </RAC.ListBox>
    </>
  );
};

export const AiMemoryNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  const collection = useApiCollection(NodeAiMemoryCollectionSchema);

  const { memoryType } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ nodeId }))
          .select((_) => pick(_.item, 'memoryType'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeAiMemorySchema);

  const { icon, title } = HashMap.unsafeGet(memoryProviderMap, memoryType);

  return (
    <SimpleNode
      className={tw`rounded-full text-lime-500`}
      handles={<Handle nodeId={nodeId} position={XF.Position.Top} type='target' />}
      icon={icon}
      nodeId={nodeId}
      selected={selected}
      title={title}
    />
  );
};

export const AiMemorySettings = ({ nodeId }: NodeSettingsProps) => {
  const collection = useApiCollection(NodeAiMemoryCollectionSchema);

  const { isReadOnly = false } = use(FlowContext);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ nodeId }))
          .select((_) => pick(_.item, 'memoryType', 'windowSize'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeAiMemorySchema);

  const { title } = HashMap.unsafeGet(memoryProviderMap, data.memoryType);

  return (
    <NodeSettingsContainer nodeId={nodeId} title={title}>
      <div className={tw`flex flex-col items-start gap-y-5`}>
        <NumberField
          isReadOnly={isReadOnly}
          label='Window Size'
          onChange={(_) => collection.utils.updatePaced({ nodeId, windowSize: _ })}
          value={data.windowSize}
        />
      </div>
    </NodeSettingsContainer>
  );
};
