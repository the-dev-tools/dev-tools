import { createCollection, localOnlyCollectionOptions, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Schema } from 'effect';
import { Ulid } from 'id128';
import { use, useCallback, useEffect } from 'react';
import { eqStruct } from '~/utils/tanstack-db';
import { FlowContext } from './context';

export const VIEWPORT_MIN_ZOOM = 0.1;
export const VIEWPORT_MAX_ZOOM = 2;

class ViewportSchema extends Schema.Class<ViewportSchema>('ViewportSchema')({
  flowId: Schema.Uint8ArrayFromSelf,

  x: Schema.Number,
  y: Schema.Number,

  zoom: Schema.Number,
}) {}

const viewportCollection = createCollection(
  localOnlyCollectionOptions({
    getKey: (_) => Ulid.construct(_.flowId).toCanonical(),
    schema: Schema.standardSchemaV1(ViewportSchema),
  }),
);

export const useViewport = () => {
  const { flowId } = use(FlowContext);
  const key = Ulid.construct(flowId).toCanonical();

  const store = XF.useStoreApi();
  const nodesInitialized = XF.useStore((_) => _.nodesInitialized);

  const viewport = useLiveQuery(
    (_) => _.from({ item: viewportCollection }).where(eqStruct({ flowId })).findOne(),
    [flowId],
  ).data ?? { x: 0, y: 0, zoom: 1 };

  const onViewportChange = useCallback(
    (viewport: XF.Viewport) => {
      if (!viewportCollection.has(key)) return;

      viewportCollection.update(key, (draft) => {
        draft.x = viewport.x;
        draft.y = viewport.y;
        draft.zoom = viewport.zoom;
      });
    },
    [key],
  );

  useEffect(() => {
    if (viewportCollection.has(key)) return;

    const { domNode, nodeLookup, nodeOrigin, nodes } = store.getState();
    const container = domNode?.getBoundingClientRect();

    if (!container || !nodesInitialized) return;

    const bounds = XF.getNodesBounds(nodes, { nodeLookup, nodeOrigin });

    const viewport = XF.getViewportForBounds(
      bounds,
      container.width,
      container.height,
      VIEWPORT_MIN_ZOOM,
      VIEWPORT_MAX_ZOOM,
      {
        x: 0.05,
        y: 0.05,
      },
    );

    viewportCollection.insert({ flowId, ...viewport });
  }, [flowId, key, nodesInitialized, store]);

  return { onViewportChange, viewport };
};
