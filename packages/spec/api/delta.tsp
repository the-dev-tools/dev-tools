using DevTools;
using Reflection;

namespace API.Delta;

enum SourceKind {
  Origin,
  Mixed,
  Delta,
}

@DataClient.entity
@copyParent(TResource)
@templateOf(TResource)
model Delta<TResource extends Model> {
  ...TResource;
  @visibility(Resource.Mutation.None) origin?: TResource;
  @visibility(Resource.Mutation.None) source?: SourceKind;
}

@templateOf(TResource)
model DeltaListRequest<TResource extends Model> {
  ...Resource.List.ListRequest<TResource>;
  @normalKey originId: Resource.Id;
}

@templateOf(TResource)
model DeltaCreateRequest<TResource extends Model> {
  ...Resource.Create.CreateRequest<TResource>;
  @normalKey originId: Resource.Id;
}

@templateOf(TResource)
model DeltaResetRequest<TResource extends Model> {
  ...KeyOf<TResource>;
}

@templateOf(TResource)
model DeltaResetResponse<TResource extends Model> {}

@templateOf(TDelta)
interface Interface<TResource extends Model, TDelta extends Model = Delta<TResource>>
  extends Resource.List.Interface<TDelta>,
    Resource.Create.Interface<TDelta>,
    Resource.Delete.Interface<TDelta> {
  @DataClient.endpoint(
    "delta.js:deltaUpdate",
    {
      schema: DataClient.Schema<TDelta>,
    }
  )
  Update is Resource.Update.Operation<TDelta>;

  @DataClient.endpoint(
    "delta.js:deltaReset",
    {
      schema: DataClient.Schema<TDelta>,
    }
  )
  Reset(...DeltaResetRequest<TResource>): DeltaResetResponse<TResource>;
}
