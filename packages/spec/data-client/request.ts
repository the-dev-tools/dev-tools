import { create } from '@bufbuild/protobuf';
import { Endpoint } from '@data-client/endpoint';
import { Array, Match, Option, pipe } from 'effect';
import {
  BodyFormDeltaMoveRequestSchema,
  BodyFormMoveRequestSchema,
  BodyService,
  BodyUrlEncodedDeltaMoveRequestSchema,
  BodyUrlEncodedMoveRequestSchema,
} from '../dist/buf/typescript/collection/item/body/v1/body_pb';
import {
  HeaderMoveRequestSchema,
  QueryDeltaMoveRequestSchema,
  QueryMoveRequestSchema,
  RequestService,
} from '../dist/buf/typescript/collection/item/request/v1/request_pb';
import { MovePosition } from '../dist/buf/typescript/resources/v1/resources_pb';
import {
  BodyFormDeltaListItemEntity,
  BodyFormListItemEntity,
  BodyUrlEncodedDeltaListItemEntity,
  BodyUrlEncodedListItemEntity,
} from '../dist/meta/collection/item/body/v1/body.entities';
import {
  HeaderListItemEntity,
  QueryDeltaListItemEntity,
  QueryListItemEntity,
} from '../dist/meta/collection/item/request/v1/request.entities';
import { MakeEndpointProps } from './resource';
import { EndpointProps, makeEndpointFn, makeKey, makeListCollection } from './utils';

export const moveBodyForm = ({ method, name }: MakeEndpointProps<typeof BodyService.method.bodyFormMove>) => {
  const list = makeListCollection({ inputPrimaryKeys: ['exampleId'], itemSchema: BodyFormListItemEntity, method });

  const endpointFn = async (props: EndpointProps<typeof BodyService.method.bodyFormMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { bodyId, position, targetBodyId } = create(BodyFormMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.bodyId.toString() === bodyId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.bodyId.toString() === targetBodyId.toString());

      const [before, after] = Array.splitAt(rest, index + offset);

      return [...before, ...move, ...after];
    }).pipe(
      Option.match({
        onNone: () => ({}),
        onSome: (_) => ({ items: _ }),
      }),
    );
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { items: list },
    sideEffect: true,
  });
};

export const moveBodyFormDelta = ({ method, name }: MakeEndpointProps<typeof BodyService.method.bodyFormDeltaMove>) => {
  const list = makeListCollection({
    inputPrimaryKeys: ['exampleId', 'originId'],
    itemSchema: BodyFormDeltaListItemEntity,
    method,
  });

  const endpointFn = async (props: EndpointProps<typeof BodyService.method.bodyFormDeltaMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { bodyId, position, targetBodyId } = create(BodyFormDeltaMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.bodyId.toString() === bodyId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.bodyId.toString() === targetBodyId.toString());

      const [before, after] = Array.splitAt(rest, index + offset);

      return [...before, ...move, ...after];
    }).pipe(
      Option.match({
        onNone: () => ({}),
        onSome: (_) => ({ items: _ }),
      }),
    );
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { items: list },
    sideEffect: true,
  });
};

export const moveBodyUrlEncoded = ({
  method,
  name,
}: MakeEndpointProps<typeof BodyService.method.bodyUrlEncodedMove>) => {
  const list = makeListCollection({
    inputPrimaryKeys: ['exampleId'],
    itemSchema: BodyUrlEncodedListItemEntity,
    method,
  });

  const endpointFn = async (props: EndpointProps<typeof BodyService.method.bodyUrlEncodedMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { bodyId, position, targetBodyId } = create(BodyUrlEncodedMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.bodyId.toString() === bodyId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.bodyId.toString() === targetBodyId.toString());

      const [before, after] = Array.splitAt(rest, index + offset);

      return [...before, ...move, ...after];
    }).pipe(
      Option.match({
        onNone: () => ({}),
        onSome: (_) => ({ items: _ }),
      }),
    );
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { items: list },
    sideEffect: true,
  });
};

export const moveBodyUrlEncodedDelta = ({
  method,
  name,
}: MakeEndpointProps<typeof BodyService.method.bodyUrlEncodedDeltaMove>) => {
  const list = makeListCollection({
    inputPrimaryKeys: ['exampleId', 'originId'],
    itemSchema: BodyUrlEncodedDeltaListItemEntity,
    method,
  });

  const endpointFn = async (props: EndpointProps<typeof BodyService.method.bodyUrlEncodedDeltaMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { bodyId, position, targetBodyId } = create(BodyUrlEncodedDeltaMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.bodyId.toString() === bodyId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.bodyId.toString() === targetBodyId.toString());

      const [before, after] = Array.splitAt(rest, index + offset);

      return [...before, ...move, ...after];
    }).pipe(
      Option.match({
        onNone: () => ({}),
        onSome: (_) => ({ items: _ }),
      }),
    );
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { items: list },
    sideEffect: true,
  });
};

export const moveQuery = ({ method, name }: MakeEndpointProps<typeof RequestService.method.queryMove>) => {
  const list = makeListCollection({ inputPrimaryKeys: ['exampleId'], itemSchema: QueryListItemEntity, method });

  const endpointFn = async (props: EndpointProps<typeof RequestService.method.queryMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { position, queryId, targetQueryId } = create(QueryMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.queryId.toString() === queryId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.queryId.toString() === targetQueryId.toString());

      const [before, after] = Array.splitAt(rest, index + offset);

      return [...before, ...move, ...after];
    }).pipe(
      Option.match({
        onNone: () => ({}),
        onSome: (_) => ({ items: _ }),
      }),
    );
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { items: list },
    sideEffect: true,
  });
};

export const moveQueryDelta = ({ method, name }: MakeEndpointProps<typeof RequestService.method.queryDeltaMove>) => {
  const list = makeListCollection({
    inputPrimaryKeys: ['exampleId', 'originId'],
    itemSchema: QueryDeltaListItemEntity,
    method,
  });

  const endpointFn = async (props: EndpointProps<typeof RequestService.method.queryDeltaMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { position, queryId, targetQueryId } = create(QueryDeltaMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.queryId.toString() === queryId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.queryId.toString() === targetQueryId.toString());

      const [before, after] = Array.splitAt(rest, index + offset);

      return [...before, ...move, ...after];
    }).pipe(
      Option.match({
        onNone: () => ({}),
        onSome: (_) => ({ items: _ }),
      }),
    );
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { items: list },
    sideEffect: true,
  });
};

export const moveHeader = ({ method, name }: MakeEndpointProps<typeof RequestService.method.headerMove>) => {
  const list = makeListCollection({ inputPrimaryKeys: ['exampleId'], itemSchema: HeaderListItemEntity, method });

  const endpointFn = async (props: EndpointProps<typeof RequestService.method.headerMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { headerId, position, targetHeaderId } = create(HeaderMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.headerId.toString() === headerId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.headerId.toString() === targetHeaderId.toString());

      const [before, after] = Array.splitAt(rest, index + offset);

      return [...before, ...move, ...after];
    }).pipe(
      Option.match({
        onNone: () => ({}),
        onSome: (_) => ({ items: _ }),
      }),
    );
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { items: list },
    sideEffect: true,
  });
};
