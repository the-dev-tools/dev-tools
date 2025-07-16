import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "../delta.tsp";
import "./example.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.body.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/item/body/v1;bodyv1",
  },
})
namespace API.CollectionItem.Body;

enum BodyKind {
  BODY_KIND_UNSPECIFIED: 0,
  BODY_KIND_FORM_ARRAY: 1,
  BODY_KIND_URL_ENCODED_ARRAY: 2,
  BODY_KIND_RAW: 3,
}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model BodyForm {
  @key bodyId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
model BodyFormMoveRequest {
  ...ParentKeyOf<BodyForm>;
  ...KeyOf<BodyForm>;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

model BodyFormMoveResponse {}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model BodyUrlEncoded {
  @key bodyId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
model BodyUrlEncodedMoveRequest {
  ...ParentKeyOf<BodyUrlEncoded>;
  ...KeyOf<BodyUrlEncoded>;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

model BodyUrlEncodedMoveResponse {}

@autoFields
@entity
model BodyRaw {
  ...KeyOf<Example.Example>;
  data: bytes;
}

@Protobuf.service
interface BodyService
  extends BlankInterface,
    // Form Array
    Resource.List.Interface<BodyForm>,
    Resource.Create.Interface<BodyForm>,
    Resource.Update.Interface<BodyForm>,
    Resource.Delete.Interface<BodyForm>,
    Delta.Interface<BodyForm>,
    // URL Encoded Array
    Resource.List.Interface<BodyUrlEncoded>,
    Resource.Create.Interface<BodyUrlEncoded>,
    Resource.Update.Interface<BodyUrlEncoded>,
    Resource.Delete.Interface<BodyUrlEncoded>,
    Delta.Interface<BodyUrlEncoded>,
    // Raw
    Resource.Get.Interface<BodyRaw>,
    Resource.Update.Interface<BodyRaw> {
  @endpoint("request.js:moveBodyForm")
  BodyFormMove(...BodyFormMoveRequest): BodyFormMoveResponse;
  @endpoint("request.js:moveBodyUrlEncoded")
  BodyUrlEncodedMove(...BodyUrlEncodedMoveRequest): BodyUrlEncodedMoveResponse;
}
