import { useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Match, pipe } from 'effect';
import { ReactNode, use, useRef } from 'react';
import { FiPlus } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { HandleKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { EdgeCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { focusVisibleRingStyles } from '@the-dev-tools/ui/focus-ring';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { eqStruct, pick } from '~/shared/lib';
import { AddNodeSidebar, AddNodeSidebarProps } from './add-node';
import { FlowContext } from './context';

interface HandleProps extends Omit<XF.HandleProps, 'children' | 'id'> {
  alwaysVisible?: boolean;
  kind?: HandleKind;
  nodeId: Uint8Array;
  nodeOffset?: { x?: number; y?: number };
  Sidebar?: (props: AddNodeSidebarProps) => ReactNode;
}

export const Handle = ({
  alwaysVisible,
  className,
  kind = HandleKind.UNSPECIFIED,
  nodeId,
  nodeOffset,
  Sidebar = AddNodeSidebar,
  ...handleProps
}: HandleProps) => {
  const { position, type } = handleProps;
  const { setSidebar } = use(FlowContext);
  const { screenToFlowPosition } = XF.useReactFlow();

  const ref = useRef<HTMLDivElement>(null);

  const id = kind === HandleKind.UNSPECIFIED ? null : kind.toString();

  const label = pipe(
    Match.value(kind),
    Match.when(HandleKind.ELSE, () => 'Else'),
    Match.when(HandleKind.THEN, () => 'Then'),
    Match.when(HandleKind.LOOP, () => 'Loop'),
    Match.when(HandleKind.AI_PROVIDER, () => 'Provider'),
    Match.when(HandleKind.AI_MEMORY, () => 'Memory'),
    Match.when(HandleKind.AI_TOOLS, () => 'Tools'),
    Match.orElse(() => null),
  );

  const edgeCollection = useApiCollection(EdgeCollectionSchema);

  const isConnected =
    useLiveQuery(
      (_) => {
        let query = _.from({ item: edgeCollection });

        if (type === 'source') query = query.where(eqStruct({ sourceHandle: kind, sourceId: nodeId }));
        else query = query.where(eqStruct({ targetId: nodeId }));

        return query.select((_) => pick(_.item, 'edgeId')).findOne();
      },
      [edgeCollection, kind, nodeId, type],
    ).data !== undefined;

  return (
    <XF.Handle
      className={twJoin(
        tw`absolute inset-0 -z-10 m-auto size-0 min-h-0 min-w-0 border-none bg-transparent`,
        position === XF.Position.Right && tw`left-auto`,
        position === XF.Position.Left && tw`right-auto`,
        position === XF.Position.Bottom && tw`top-auto`,
        position === XF.Position.Top && tw`bottom-auto`,
        className,
      )}
      id={id}
      ref={ref}
      {...handleProps}
    >
      {(!isConnected || alwaysVisible) && (
        <>
          <HandleHalo />

          {type === 'source' && (
            <div
              className={twJoin(
                tw`absolute flex -translate-1/2 items-center`,
                position === XF.Position.Right && tw`translate-x-0 flex-row`,
                position === XF.Position.Left && tw`-translate-x-full flex-row-reverse`,
                position === XF.Position.Bottom && tw`translate-y-0 flex-col`,
                position === XF.Position.Top && tw`-translate-y-full flex-col-reverse`,
              )}
            >
              <div
                className={twJoin(
                  tw`h-12 w-16 bg-fg-muted`,
                  (position === XF.Position.Right || position === XF.Position.Left) && tw`h-px`,
                  (position === XF.Position.Top || position === XF.Position.Bottom) && tw`w-px`,
                )}
              />

              <div className={tw`size-0`}>
                <div className={tw`pointer-events-none size-1.5 -translate-1/2 rounded-full bg-fg-muted`} />
              </div>

              <button
                className={focusVisibleRingStyles({
                  className: tw`
                    pointer-events-auto flex size-5 cursor-pointer items-center justify-center rounded-full border
                    border-border-emphasis bg-surface
                  `,
                })}
                onClick={() => {
                  const box = ref.current?.parentElement?.parentElement?.getBoundingClientRect();
                  let nodePosition: undefined | XF.XYPosition;

                  if (box) {
                    nodePosition = screenToFlowPosition({ x: box.x + box.width / 2, y: box.y });

                    if (nodeOffset?.x !== undefined) nodePosition.x += nodeOffset.x;
                    else {
                      if (position === XF.Position.Right) nodePosition.x += 250;
                      if (position === XF.Position.Left) nodePosition.x -= 250;
                      if (position === XF.Position.Bottom || position === XF.Position.Top) nodePosition.x += 150;
                    }

                    if (nodeOffset?.y !== undefined) nodePosition.y += nodeOffset.y;
                    else {
                      if (position === XF.Position.Bottom) nodePosition.y += 200;
                      if (position === XF.Position.Top) nodePosition.y -= 200;
                    }
                  }

                  setSidebar?.(<Sidebar handleKind={kind} position={nodePosition} sourceId={nodeId} />);
                }}
              >
                <FiPlus className={tw`size-3 text-fg-muted`} />
              </button>
            </div>
          )}
        </>
      )}

      {label && (
        <div
          className={twJoin(
            tw`absolute -translate-1/2`,
            position === XF.Position.Right && tw`translate-x-0`,
            position === XF.Position.Left && tw`-translate-x-full`,
            position === XF.Position.Bottom && tw`translate-y-0`,
            position === XF.Position.Top && tw`-translate-y-full`,
          )}
        >
          <div
            className={tw`
              mx-4 my-3 rounded-sm bg-surface p-1 text-xs leading-4 tracking-tight whitespace-nowrap text-fg-muted
            `}
          >
            {label}
          </div>
        </div>
      )}

      <div className={tw`absolute size-10 min-h-0 min-w-0 -translate-1/2 rounded-full border-none bg-transparent`}>
        <div
          className={twJoin(
            tw`absolute inset-0 m-auto bg-fg`,
            type === 'source' && tw`size-2 rounded-full`,
            type === 'target' && tw`size-2.5`,
          )}
        />
      </div>
    </XF.Handle>
  );
};

export const HandleHalo = () => (
  <div
    className={tw`
    absolute size-5 -translate-1/2 rounded-full border border-border-emphasis bg-surface-active shadow-xs
  `}
  />
);
