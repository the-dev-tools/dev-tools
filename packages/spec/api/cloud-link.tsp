using DevTools;

namespace Api.CloudLink;

enum CloudLinkKind {
  Repo,
  FlowFile,
}

model CloudLinkFlow {
  flowId: Id;
  name: string;
}

@doc("A Files row that mirrors a repo (folder) or a flow file from Stresseur Cloud. Linked rows stay out of the Files tree.")
model CloudLink {
  fileId: Id;
  kind: CloudLinkKind;
  apiUrl: string;
  cloudWorkspaceSlug: string;
  repoSlug: string;

  @doc("Repo-relative path of the flow file. Empty for a repo link.")
  path: string;

  @doc("Commit the local copy is based on")
  baseSha: string;

  @doc("Linked local clone (repo links)")
  localDir: string;

  @doc("Edited since it was fetched or last saved")
  dirty: boolean;

  @doc("Flows imported from this flow file")
  flows: CloudLinkFlow[];
}

model CloudLinkListRequest {
  workspaceId: Id;
}

model CloudLinkListResponse {
  items: CloudLink[];
}

op CloudLinkList(...CloudLinkListRequest): CloudLinkListResponse;

model CloudLinkUpsertRequest {
  workspaceId: Id;
  apiUrl: string;
  cloudWorkspaceSlug: string;
  repoSlug: string;

  @doc("Repo display name, used for the repo folder")
  repoName: string;

  path: string;
  sha: string;
  yamlText: string;

  @doc("Replace even when the local copy has unsaved edits")
  force?: boolean;
}

model CloudLinkUpsertResponse {
  link: CloudLink;
}

@doc("Imports a flow file under its repo folder, replacing the previous import of that path. Fails with FAILED_PRECONDITION when the local copy is dirty and force is not set.")
op CloudLinkUpsert(...CloudLinkUpsertRequest): CloudLinkUpsertResponse;

model CloudLinkExportRequest {
  fileId: Id;
}

model CloudLinkExportResponse {
  path: string;
  baseSha: string;
  yamlText: string;
}

op CloudLinkExport(...CloudLinkExportRequest): CloudLinkExportResponse;

model CloudLinkMarkSavedRequest {
  fileId: Id;

  @doc("New base commit, when known")
  baseSha?: string;
}

@doc("Clears dirty after the flow file was written to the clone or proposed as a change.")
op CloudLinkMarkSaved(...CloudLinkMarkSavedRequest): {};

model CloudLinkSetLocalDirRequest {
  workspaceId: Id;
  apiUrl: string;
  cloudWorkspaceSlug: string;
  repoSlug: string;
  repoName: string;

  @doc("Empty to unlink the clone")
  localDir: string;
}

model CloudLinkSetLocalDirResponse {
  link: CloudLink;
}

op CloudLinkSetLocalDir(...CloudLinkSetLocalDirRequest): CloudLinkSetLocalDirResponse;

model CloudLinkUnlinkRequest {
  fileId: Id;
}

@doc("Removes the link and its imported rows. Unlinking a repo removes its flow files too.")
op CloudLinkUnlink(...CloudLinkUnlinkRequest): {};
