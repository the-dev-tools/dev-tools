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
model Item<TResource extends Model> {
  ...TResource;
  @visibility(Resource.Mutation.None) origin?: TResource;
  @visibility(Resource.Mutation.None) source?: SourceKind;
}

@autoFields
@move(Delta, TResource)
@friendlyName("{name}DeltaListRequest", TResource)
model ListRequest<TResource extends Model> {
  ...ParentKeyOf<TResource>;
  originId: Resource.Id;
}

@move(Delta, TResource)
@friendlyName("{name}DeltaResetRequest", TResource)
model ResetRequest<TResource extends Model> {
  ...KeyOf<TResource>;
}

interface Interface<
  TResource extends Model,
  TDelta extends Model = Item<TResource>,
  TListRequest extends Model = ListRequest<TResource>
>
  extends Resource.List.Interface<TDelta, TMove = TResource, TRequest = TListRequest>,
    Resource.Create.Interface<TDelta, TMove = TResource, TListRequest = TListRequest>,
    Resource.Update.Interface<TDelta, TMove = TResource>,
    Resource.Delete.Interface<TDelta, TMove = TResource> {
  @move(Delta, TResource)
  @useFriendlyName
  @friendlyName("{name}Reset", TDelta)
  Reset(...ResetRequest<TResource>): WellKnown.Empty;
}
