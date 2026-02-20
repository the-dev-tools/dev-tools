import { create, fromJson, type JsonValue, toJson } from '@bufbuild/protobuf';
import { type Value, ValueSchema } from '@bufbuild/protobuf/wkt';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-node';
import * as BA from 'better-auth/adapters';
import { Match, pipe, Record } from 'effect';
import id128 from 'id128';
import {
  AuthAdapterService,
  Connector,
  Direction,
  Operator,
  WhereSchema,
} from '@the-dev-tools/spec/buf/api/private/auth_adapter/v1/auth_adapter_pb';

// eslint-disable-next-line import-x/no-named-as-default-member
const { Ulid } = id128;

/** Unwrap a protobuf Value to a plain JSON value. */
const unwrapValue = (v: Value): JsonValue => toJson(ValueSchema, v);

/** Unwrap a protobuf Value (struct) to a plain JS object. */
const unwrapObject = (v: Value): Record<string, unknown> => unwrapValue(v) as Record<string, unknown>;

/** Unwrap a map<string, Value> to a plain JS object. */
const unwrapMap = (m: Record<string, Value>): Record<string, unknown> =>
  Record.map(m, (_) => unwrapValue(_));

export const makeTransport = (socketPath: string) =>
  createConnectTransport({
    baseUrl: 'http://the-dev-tools:0',
    httpVersion: '1.1',
    nodeOptions: { socketPath },
    useHttpGet: true,
  });

export interface CustomAdapterConfig {
  debugLogs?: BA.DBAdapterDebugLogOption;
  socketPath: string;
}

export const adapter = (config: CustomAdapterConfig) => {
  const transport = makeTransport(config.socketPath);
  const client = createClient(AuthAdapterService, transport);

  return BA.createAdapterFactory({
    adapter: (_) => ({
      create: <T>(_: Parameters<BA.CustomAdapter['create']>[0]) =>
        client
          .create({
            data: Record.map(_.data, (_) => fromJson(ValueSchema, _ as JsonValue)),
            model: _.model,
            ...(_.select && { select: _.select }),
          })
          .then((_) => unwrapMap(_.data) as T),

      update: <T>(_: Parameters<BA.CustomAdapter['update']>[0]) =>
        client
          .update({
            model: _.model,
            update: fromJson(ValueSchema, _.update as JsonValue),
            where: mapWhere(_.where),
          })
          .then((_) => (_.data ? unwrapObject(_.data) : null) as null | T),

      updateMany: (_) =>
        client
          .updateMany({
            model: _.model,
            update: Record.map(_.update, (_) => fromJson(ValueSchema, _ as JsonValue)),
            where: mapWhere(_.where),
          })
          .then((_) => _.count),

      findOne: <T>(_: Parameters<BA.CustomAdapter['findOne']>[0]) =>
        client
          .find({
            model: _.model,
            where: mapWhere(_.where),
            ...(_.select && { select: _.select }),
          })
          .then((_) => (_.data ? unwrapObject(_.data) : null) as null | T),

      findMany: <T>(_: Parameters<BA.CustomAdapter['findMany']>[0]) =>
        client
          .findMany({
            limit: _.limit,
            model: _.model,
            ...(_.where && { where: mapWhere(_.where) }),
            ...(_.offset && { offset: _.offset }),
            ...(_.sortBy && {
              sortBy: {
                direction: pipe(
                  Match.value(_.sortBy.direction),
                  Match.when('asc', () => Direction.ASCENDING),
                  Match.when('desc', () => Direction.DESCENDING),
                  Match.exhaustive,
                ),
                field: _.sortBy.field,
              },
            }),
          })
          .then((_) => _.items.map(unwrapObject) as T[]),

      delete: (_) =>
        client
          .delete({
            model: _.model,
            where: mapWhere(_.where),
          })
          .then(() => undefined),

      deleteMany: (_) =>
        client
          .deleteMany({
            model: _.model,
            where: mapWhere(_.where),
          })
          .then((_) => _.count),

      count: (_) =>
        client
          .count({
            model: _.model,
            ...(_.where && { where: mapWhere(_.where) }),
          })
          .then((_) => _.count),

      createSchema: ({ file = 'schema.json', tables }) =>
        Promise.resolve({ code: JSON.stringify(tables, undefined, 2), path: file }),
    }),
    config: {
      adapterId: '@the-dev-tools/auth-adapter',
      adapterName: 'DevTools Auth Adapter',

      customIdGenerator: () => Ulid.generate().toCanonical(),

      debugLogs: config.debugLogs,
      supportsArrays: true,
      supportsBooleans: true,
      supportsDates: false,
      supportsJSON: true,
      supportsNumericIds: false,
      supportsUUIDs: false,
      transaction: false,
      usePlural: false,
    },
  });
};

const mapWhere = (_: Required<BA.Where>[]) =>
  _.map((_) =>
    create(WhereSchema, {
      field: _.field,
      value: fromJson(ValueSchema, _.value as JsonValue),
      ...(_.connector && {
        connector: pipe(
          Match.value(_.connector),
          Match.when('AND', () => Connector.AND),
          Match.when('OR', () => Connector.OR),
          Match.exhaustive,
        ),
      }),
      ...(_.operator && {
        operator: pipe(
          Match.value(_.operator),
          Match.when('eq', () => Operator.EQUAL),
          Match.when('ne', () => Operator.NOT_EQUAL),
          Match.when('lt', () => Operator.LESS_THAN),
          Match.when('lte', () => Operator.LESS_OR_EQUAL),
          Match.when('gt', () => Operator.GREATER_THAN),
          Match.when('gte', () => Operator.GREATER_OR_EQUAL),
          Match.when('in', () => Operator.IN),
          Match.when('not_in', () => Operator.NOT_IN),
          Match.when('contains', () => Operator.CONTAINS),
          Match.when('starts_with', () => Operator.STARTS_WITH),
          Match.when('ends_with', () => Operator.ENDS_WITH),
          Match.exhaustive,
        ),
      }),
    }),
  );
