using DevTools;

namespace API.CollectionItem.Body;

enum BodyKind {
  FormArray,
  UrlEncodedArray,
  Raw,
}

@DataClient.entity
@parent(Example.Example)
model BodyForm {
  @key bodyId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model BodyFormMoveRequest {
  ...ParentKeyOf<BodyForm>;
  ...KeyOf<BodyForm>;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

model BodyFormMoveResponse {}

model BodyFormDeltaMoveRequest {
  ...ParentKeyOf<BodyForm>;
  ...KeyOf<BodyForm>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

model BodyFormDeltaMoveResponse {}

@DataClient.entity
@parent(Example.Example)
model BodyUrlEncoded {
  @key bodyId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model BodyUrlEncodedMoveRequest {
  ...ParentKeyOf<BodyUrlEncoded>;
  ...KeyOf<BodyUrlEncoded>;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

model BodyUrlEncodedMoveResponse {}

model BodyUrlEncodedDeltaMoveRequest {
  ...ParentKeyOf<BodyUrlEncoded>;
  ...KeyOf<BodyUrlEncoded>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

model BodyUrlEncodedDeltaMoveResponse {}

@DataClient.entity
model BodyRaw {
  ...KeyOf<Example.Example>;
  data: bytes;
}

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
  @DataClient.endpoint("request.js:moveBodyForm")
  BodyFormMove(...BodyFormMoveRequest): BodyFormMoveResponse;
  @DataClient.endpoint("request.js:moveBodyFormDelta")
  BodyFormDeltaMove(...BodyFormDeltaMoveRequest): BodyFormDeltaMoveResponse;
  @DataClient.endpoint("request.js:moveBodyUrlEncoded")
  BodyUrlEncodedMove(...BodyUrlEncodedMoveRequest): BodyUrlEncodedMoveResponse;
  @DataClient.endpoint("request.js:moveBodyUrlEncodedDelta")
  BodyUrlEncodedDeltaMove(...BodyUrlEncodedDeltaMoveRequest): BodyUrlEncodedDeltaMoveResponse;
}
