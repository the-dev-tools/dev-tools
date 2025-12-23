import * as XF from '@xyflow/react';
import { Match, pipe } from 'effect';
import { use, useRef } from 'react';
import * as RAC from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { HandleKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { focusVisibleRingStyles } from '@the-dev-tools/ui/focus-ring';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { AddNodeSidebar } from './add-node';
import { FlowContext } from './context';

interface HandleProps extends Omit<XF.HandleProps, 'children' | 'className' | 'id'> {
  kind?: HandleKind;
  nodeId: Uint8Array;
}

export const Handle = (props: HandleProps) => {
  const { kind = HandleKind.UNSPECIFIED, nodeId, position, type } = props;
  const { setSidebar } = use(FlowContext);
  const { screenToFlowPosition } = XF.useReactFlow();

  const ref = useRef<HTMLDivElement>(null);

  const id = kind === HandleKind.UNSPECIFIED ? null : kind.toString();

  const label = pipe(
    Match.value(kind),
    Match.when(HandleKind.ELSE, () => 'Else'),
    Match.when(HandleKind.THEN, () => 'Then'),
    Match.when(HandleKind.LOOP, () => 'Loop'),
    Match.orElse(() => null),
  );

  const isConnected = XF.useNodeConnections({ ...(id && { handleId: id }), handleType: type }).length > 0;

  return (
    <div
      className={twJoin(
        tw`absolute inset-0 -z-10 m-auto size-0`,
        position === XF.Position.Right && tw`left-auto`,
        position === XF.Position.Left && tw`right-auto`,
        position === XF.Position.Bottom && tw`top-auto`,
        position === XF.Position.Top && tw`bottom-auto`,
      )}
      ref={ref}
    >
      {!isConnected && (
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
                  tw`h-12 w-16 bg-slate-800`,
                  (position === XF.Position.Right || position === XF.Position.Left) && tw`h-px`,
                  (position === XF.Position.Top || position === XF.Position.Bottom) && tw`w-px`,
                )}
              />

              <div className={tw`size-0`}>
                <div className={tw`pointer-events-none size-1.5 -translate-1/2 rounded-full bg-slate-800`} />
              </div>

              <RAC.Button
                className={focusVisibleRingStyles({
                  className: tw`
                    pointer-events-auto flex size-5 cursor-pointer items-center justify-center rounded-full border
                    border-slate-800 bg-white
                  `,
                })}
                onPress={() => {
                  const box = ref.current?.parentElement?.getBoundingClientRect();
                  let nodePosition: undefined | XF.XYPosition;

                  if (box) {
                    nodePosition = screenToFlowPosition({ x: box.x + box.width / 2, y: box.y });
                    if (position === XF.Position.Right) nodePosition.x += 250;
                    if (position === XF.Position.Left) nodePosition.x -= 250;
                    if (position === XF.Position.Bottom) nodePosition.y += 150;
                    if (position === XF.Position.Top) nodePosition.y -= 150;
                    if (position === XF.Position.Bottom || position === XF.Position.Top) nodePosition.x += 150;
                  }

                  setSidebar?.(<AddNodeSidebar handleKind={kind} position={nodePosition} sourceId={nodeId} />);
                }}
              >
                <FiPlus className={tw`size-3 text-slate-800`} />
              </RAC.Button>
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
          <div className={tw`mx-4 my-3 rounded-sm bg-white p-1 text-xs leading-4 tracking-tight text-slate-500`}>
            {label}
          </div>
        </div>
      )}

      <XF.Handle className={tw`absolute size-0 min-h-0 min-w-0 border-none bg-transparent`} id={id} {...props}>
        <div className={tw`size-10 -translate-1/2 rounded-full`}>
          <div
            className={twJoin(
              tw`absolute inset-0 m-auto bg-slate-800`,
              type === 'source' && tw`size-2 rounded-full`,
              type === 'target' && tw`size-2.5`,
            )}
          />
        </div>
      </XF.Handle>
    </div>
  );
};

export const HandleHalo = () => (
  <div className={tw`absolute size-5 -translate-1/2 rounded-full border border-slate-300 bg-slate-200 shadow-xs`} />
);
