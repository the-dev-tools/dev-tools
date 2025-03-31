import "@typespec/protobuf";

namespace TypeSpec.Protobuf.WellKnown;

model JsonValue is Extern<"google/protobuf/struct.proto", "google.protobuf.Value">;
