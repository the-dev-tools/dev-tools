import { create } from '@bufbuild/protobuf';
import { json } from '@codemirror/lang-json';
import { eq, useLiveQuery } from '@tanstack/react-db';
import CodeMirror from '@uiw/react-codemirror';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use, useMemo } from 'react';
import { FiPlus, FiX, FiZap } from 'react-icons/fi';
import { NodeSubFlowTriggerSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeSubFlowTriggerCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { PlayIcon } from '@the-dev-tools/ui/icons';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { useTheme } from '@the-dev-tools/ui/theme';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, SimpleNode } from '../node';

const defaultNodeSubFlowTrigger = create(NodeSubFlowTriggerSchema);

const paramTypes = ['string', 'number', 'boolean', 'json'] as const;

const placeholderByType: Record<string, string> = {
  boolean: 'true or false',
  json: '{"key": "value"} or [1, 2, 3]',
  number: '0',
  string: 'Default value',
};

export const SubFlowTriggerNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`text-green-500`}
      handles={
        <>
          <div className={tw`absolute top-1/2 -translate-x-full -translate-y-1/2 p-1`}>
            <FiZap className={tw`size-5 text-accent`} />
          </div>

          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
        </>
      }
      icon={<PlayIcon />}
      nodeId={nodeId}
      selected={selected}
      title='Sub-Flow Trigger'
    />
  );
};

export const SubFlowTriggerSettings = ({ nodeId }: NodeSettingsProps) => {
  const collection = useApiCollection(NodeSubFlowTriggerCollectionSchema);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'params'))
          .findOne(),
      [collection, nodeId],
    ).data ?? defaultNodeSubFlowTrigger;

  const { isReadOnly = false } = use(FlowContext);

  return (
    <NodeSettingsBody nodeId={nodeId} title='Sub-Flow Trigger'>
      <div className={tw`flex flex-col gap-4`}>
        <FieldLabel>Input Parameters</FieldLabel>
        <div className={tw`text-xs text-on-neutral-low`}>
          Define parameters that callers must provide when invoking this sub-flow.
        </div>

        <div className={tw`flex flex-col gap-3`}>
          {data.params.map((param, index) => (
            <ParamRow
              collection={collection}
              index={index}
              isReadOnly={isReadOnly}
              key={index}
              nodeId={nodeId}
              param={param}
              params={data.params}
            />
          ))}
        </div>

        {!isReadOnly && (
          <Button
            className={tw`w-full justify-start`}
            onPress={() => {
              const params = [...data.params, { defaultValue: '', name: '', required: false, type: 'string' }];
              collection.utils.updatePaced({ nodeId, params });
            }}
            variant='ghost'
          >
            <FiPlus className={tw`size-4 text-on-neutral-low`} />
            Add parameter
          </Button>
        )}
      </div>
    </NodeSettingsBody>
  );
};

interface ParamRowProps {
  collection: ReturnType<typeof useApiCollection<typeof NodeSubFlowTriggerCollectionSchema>>;
  index: number;
  isReadOnly: boolean;
  nodeId: Uint8Array;
  param: (typeof defaultNodeSubFlowTrigger.params)[number];
  params: typeof defaultNodeSubFlowTrigger.params;
}

const ParamRow = ({ collection, index, isReadOnly, nodeId, param, params }: ParamRowProps) => {
  const { theme } = useTheme();
  const jsonExtensions = useMemo(() => [json()], []);

  const updateParam = (patch: Partial<typeof param>) => {
    const next = [...params];
    next[index] = { ...next[index]!, ...patch };
    collection.utils.updatePaced({ nodeId, params: next });
  };

  // Normalize legacy 'any' type to 'string' for display
  const displayType = param.type === 'any' || param.type === '' ? 'string' : param.type;

  return (
    <div className={tw`flex items-start gap-2 rounded-lg border border-neutral bg-neutral-lowest p-3`}>
      <div className={tw`flex flex-1 flex-col gap-2`}>
        <TextInputField
          aria-label='Parameter name'
          isReadOnly={isReadOnly}
          onChange={(name) => void updateParam({ name })}
          placeholder='Parameter name'
          value={param.name}
        />

        <div className={tw`flex items-center gap-2`}>
          <Select
            aria-label='Type'
            isDisabled={isReadOnly}
            items={paramTypes.map((_) => ({ id: _, name: _ }))}
            onChange={(_) => {
              if (_ === null) return;
              updateParam({ defaultValue: '', type: _ });
            }}
            triggerClassName={tw`w-24 shrink-0 px-2 py-1.5 text-xs`}
            value={displayType}
          >
            {(_) => <SelectItem id={_.id}>{_.name}</SelectItem>}
          </Select>

          {displayType === 'json' ? (
            <div className={tw`min-h-20 flex-1 overflow-hidden rounded-md border border-neutral`}>
              <CodeMirror
                extensions={jsonExtensions}
                height='80px'
                indentWithTab={false}
                onChange={(defaultValue) => void updateParam({ defaultValue })}
                placeholder={placeholderByType.json}
                readOnly={isReadOnly}
                theme={theme}
                value={param.defaultValue}
              />
            </div>
          ) : (
            <TextInputField
              aria-label='Default value'
              className={tw`flex-1`}
              isReadOnly={isReadOnly}
              onChange={(defaultValue) => void updateParam({ defaultValue })}
              placeholder={placeholderByType[displayType] ?? 'Default value'}
              value={param.defaultValue}
            />
          )}

          <Button
            className={tw`shrink-0 px-2 py-1 text-xs`}
            isDisabled={isReadOnly}
            onPress={() => void updateParam({ required: !param.required })}
            variant={param.required ? 'primary' : 'ghost'}
          >
            {param.required ? 'Required' : 'Optional'}
          </Button>
        </div>
      </div>

      {!isReadOnly && (
        <Button
          className={tw`mt-1 p-1 text-danger`}
          onPress={() => {
            const next = params.filter((_, i) => i !== index);
            collection.utils.updatePaced({ nodeId, params: next });
          }}
          variant='ghost'
        >
          <FiX className={tw`size-4`} />
        </Button>
      )}
    </div>
  );
};
