using DevTools;

namespace API.Tag;

enum TagColor {
  Slate,
  Green,
  Amber,
  Sky,
  Purple,
  Rose,
  Blue,
  Fuchsia,
}

@DataClient.entity
@parent(Workspace.Workspace)
model Tag {
  @key tagId: Resource.Id;
  name: string;
  color: TagColor;
}

interface TagService extends Resource.CRUD<Tag> {}
