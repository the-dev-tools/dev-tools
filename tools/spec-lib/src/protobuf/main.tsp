import "../../dist/src/protobuf";

namespace DevTools.Protobuf;

scalar sint32 extends int32;
scalar sint64 extends int64;
scalar sfixed32 extends int32;
scalar sfixed64 extends int64;
scalar fixed32 extends uint32;
scalar fixed64 extends uint64;

alias keyType =
  | int32
  | int64
  | uint32
  | uint64
  | sint32
  | sint64
  | fixed32
  | fixed64
  | sfixed32
  | sfixed64
  | boolean
  | string;

@Private.map(Key, Value)
scalar Map<Key extends keyType, Value>;

@Private.external(Path, Name)
model External<Path extends valueof string, Name extends valueof string> {}

enum StreamMode {
  Duplex,
  In,
  Out,
  None,
}

extern dec stream(target: Reflection.Operation, mode: StreamMode);

namespace Private {
  extern dec map(target: Reflection.Scalar, key: keyType, value);
  extern dec external(target: Reflection.Model, path: valueof string, name: valueof string);
}

namespace WellKnown {
  model Any is External<"google/protobuf/any.proto", "google.protobuf.Any">;
  model Empty is External<"google/protobuf/empty.proto", "google.protobuf.Empty">;
  model Json is External<"google/protobuf/struct.proto", "google.protobuf.Value">;
  model Timestamp is External<"google/protobuf/timestamp.proto", "google.protobuf.Timestamp">;
}

namespace Validate {
  const path = "buf/validate/validate.proto";

  namespace Field {
    model Required is External<path, "(buf.validate.field).required">;
  }
}
