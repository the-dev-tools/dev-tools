import "@typespec/protobuf";

import "./lib.tsp";
import "./resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Reflection;

@package({
  name: "delta.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/delta/v1;deltav1",
  },
})
namespace API.Delta;

enum SourceKind {
  SOURCE_KIND_UNSPECIFIED: 0,
  SOURCE_KIND_ORIGIN: 1,
  SOURCE_KIND_MIXED: 2,
  SOURCE_KIND_DELTA: 3,
}

@autoFields
@entity
@move(Delta, TResource)
@useFriendlyName
@friendlyName("{name}Delta", TResource)
@parentResourceOf(TResource)
model Item<TResource extends Model> {
  ...TResource;
  @visibility(Resource.Mutation.None) origin?: TResource;
  @visibility(Resource.Mutation.None) source?: SourceKind;
}

@autoFields
@move(Delta, TResource)
@friendlyName("{name}DeltaListRequest", TResource)
model ListRequest<TResource extends Model> {
  ...Resource.List.Request<TResource>;
  @normalKey originId: Resource.Id;
}

@autoFields
@move(Delta, TResource)
@friendlyName("{name}DeltaCreateRequest", TResource)
model CreateRequest<TResource extends Model> {
  ...Resource.Create.Request<TResource>;
  @normalKey originId: Resource.Id;
}

@autoFields
@move(Delta, TResource)
@friendlyName("{name}DeltaUpdateRequest", TResource)
model UpdateRequest<
  TResource extends Model,
  TDelta extends Model = Item<TResource>
> {
  ...ParentKeyOf<TResource>;
  ...Resource.Update.Request<TDelta>;
}

@autoFields
@move(Delta, TResource)
@friendlyName("{name}DeltaDeleteRequest", TResource)
model DeleteRequest<
  TResource extends Model,
  TDelta extends Model = Item<TResource>
> {
  ...ParentKeyOf<TResource>;
  ...Resource.Delete.Request<TDelta>;
}

@move(Delta, TResource)
@friendlyName("{name}DeltaResetRequest", TResource)
model ResetRequest<TResource extends Model> {
  ...ParentKeyOf<TResource>;
  ...KeyOf<TResource>;
}

@move(Delta, TResource)
@friendlyName("{name}DeltaResetResponse", TResource)
model ResetResponse<TResource extends Model> {}

interface Interface<
  TResource extends Model,
  TDelta extends Model = Item<TResource>,
  TListRequest extends Model = ListRequest<TResource>,
  TCreateRequest extends Model = CreateRequest<TResource>,
  TUpdateRequest extends Model = UpdateRequest<TResource, TDelta>,
  TDeleteRequest extends Model = DeleteRequest<TResource, TDelta>
>
  extends Resource.List.Interface<TDelta, TMove = TResource, TRequest = TListRequest>,
    Resource.Create.Interface<TDelta, TMove = TResource, TRequest = TCreateRequest, TListRequest = TListRequest>,
    Resource.Delete.Interface<TDelta, TMove = TResource, TRequest = TDeleteRequest> {
  @move(Delta, TResource)
  @useFriendlyName
  @friendlyName("{name}Update", TDelta)
  @endpoint(
    "delta.js:deltaUpdate",
    {
      schema: Schema<TDelta>,
    }
  )
  Update is Resource.Update.Operation<TDelta, TMove = TResource, TRequest = TUpdateRequest>;

  @move(Delta, TResource)
  @useFriendlyName
  @friendlyName("{name}Reset", TDelta)
  @endpoint(
    "delta.js:deltaReset",
    {
      schema: Schema<TDelta>,
    }
  )
  Reset(...ResetRequest<TResource>): ResetResponse<TResource>;
}
