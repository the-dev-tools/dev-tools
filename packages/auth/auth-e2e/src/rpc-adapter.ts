import { create, fromJson, toJson } from '@bufbuild/protobuf';
import type { JsonValue } from '@bufbuild/protobuf';
import { ValueSchema } from '@bufbuild/protobuf/wkt';
import { createClient, type Transport } from '@connectrpc/connect';
import { createAdapter } from 'better-auth/adapters';

import {
  AuthAdapterService,
  Connector,
  Direction,
  Operator,
  SortBySchema,
  WhereSchema,
} from '@the-dev-tools/spec/buf/api/auth_adapter/v1/auth_adapter_pb';

const OPERATOR_MAP: Record<string, Operator> = {
  eq: Operator.EQUAL,
  ne: Operator.NOT_EQUAL,
  lt: Operator.LESS_THAN,
  lte: Operator.LESS_OR_EQUAL,
  gt: Operator.GREATER_THAN,
  gte: Operator.GREATER_OR_EQUAL,
  in: Operator.IN,
  contains: Operator.CONTAINS,
  starts_with: Operator.STARTS_WITH,
  ends_with: Operator.ENDS_WITH,
};

/**
 * Converts JavaScript values to proto-serializable JSON, turning Date objects
 * into Unix timestamps (seconds) as expected by the Go authadapter.
 */
function processForProto(value: unknown): JsonValue {
  if (value instanceof Date) {
    return Math.floor(value.getTime() / 1000);
  }
  if (Array.isArray(value)) {
    return value.map((v: unknown) => processForProto(v)) as JsonValue[];
  }
  if (value !== null && typeof value === 'object') {
    const result: Record<string, JsonValue> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      result[k] = processForProto(v);
    }
    return result;
  }
  return value as JsonValue;
}

interface BetterAuthWhere {
  field: string;
  operator: string;
  value: unknown;
  connector: string;
}

function toProtoWhere(w: BetterAuthWhere) {
  return create(WhereSchema, {
    field: w.field,
    operator: OPERATOR_MAP[w.operator] ?? Operator.EQUAL,
    // Use processForProto so Date values in where conditions (e.g. expiresAt < now)
    // are converted to Unix seconds before passing to Go.
    value: fromJson(ValueSchema, processForProto(w.value)),
    connector: w.connector === 'OR' ? Connector.OR : Connector.AND,
  });
}

export function createRpcAdapter(transport: Transport) {
  const client = createClient(AuthAdapterService, transport);

  return createAdapter({
    config: {
      adapterId: 'rpc-adapter',
      // Go adapter generates IDs via parseOrGenerateID (ULID).
      // BetterAuth must NOT pre-generate IDs since UUIDs would fail ULID parsing.
      disableIdGeneration: true,
      // Receive Date objects from BetterAuth; convert with customTransformInput.
      supportsDates: true,
      supportsBooleans: true,
      supportsJSON: false,
      // Convert Date objects to Unix seconds before calling the Go RPC.
      customTransformInput: ({ data, fieldAttributes }) => {
        if (fieldAttributes.type === 'date' && data instanceof Date) {
          return Math.floor(data.getTime() / 1000);
        }
        return data as JsonValue;
      },
      // Convert Unix seconds returned by Go back to Date objects for BetterAuth.
      customTransformOutput: ({ data, fieldAttributes }) => {
        if (fieldAttributes.type === 'date' && typeof data === 'number') {
          return new Date(data * 1000);
        }
        return data;
      },
    },

    adapter: () => ({
      async create({ model, data, select }) {
        const resp = await client.create({
          model,
          // customTransformInput has already converted Date → Unix seconds,
          // so data is safe to pass directly as a JSON value.
          data: fromJson(ValueSchema, data as JsonValue),
          select: select ?? [],
        });
        // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
        return toJson(ValueSchema, resp.data!) as never;
      },

      async findOne({ model, where, select }) {
        const resp = await client.find({
          model,
          where: where.map(toProtoWhere),
          select: select ?? [],
        });
        if (resp.data == null) return null as never;
        return toJson(ValueSchema, resp.data) as never;
      },

      async findMany({ model, where, limit, sortBy, offset }) {
        const resp = await client.findMany({
          model,
          where: where?.map(toProtoWhere) ?? [],
          limit: limit ?? 0,
          offset: offset ?? undefined,
          sortBy: sortBy
            ? create(SortBySchema, {
                field: sortBy.field,
                direction:
                  sortBy.direction === 'asc'
                    ? Direction.ASCENDING
                    : Direction.DESCENDING,
              })
            : undefined,
        });
        return resp.items.map((v) => toJson(ValueSchema, v)) as never;
      },

      async update({ model, where, update }) {
        const resp = await client.update({
          model,
          where: where.map(toProtoWhere),
          update: fromJson(ValueSchema, update as JsonValue),
        });
        if (resp.data == null) return null as never;
        return toJson(ValueSchema, resp.data) as never;
      },

      async updateMany({ model, where, update }) {
        const updateProto: Record<string, ReturnType<typeof fromJson<typeof ValueSchema>>> = {};
        for (const [key, val] of Object.entries(update)) {
          updateProto[key] = fromJson(ValueSchema, processForProto(val));
        }
        const resp = await client.updateMany({
          model,
          where: where.map(toProtoWhere),
          update: updateProto,
        });
        return resp.count;
      },

      async delete({ model, where }) {
        await client.delete({ model, where: where.map(toProtoWhere) });
      },

      async deleteMany({ model, where }) {
        const resp = await client.deleteMany({
          model,
          where: where.map(toProtoWhere),
        });
        return resp.count;
      },

      async count({ model, where }) {
        const resp = await client.count({
          model,
          where: where?.map(toProtoWhere) ?? [],
        });
        return resp.count;
      },
    }),
  });
}
