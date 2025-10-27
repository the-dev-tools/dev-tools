namespace Api.Import;

enum ImportMissingDataKind {
  Domain,
}

model ImportDomainData {
  enabled: boolean;
  domain: string;
  variable: string;
}

model ImportRequest {
  workspaceId: Id;
  name: string;
  data: bytes;
  textData: string;
  domainData?: ImportDomainData[];
}

model ImportResponse {
  missingData: ImportMissingDataKind;
  domains?: string[];
}

op Import(...ImportRequest): ImportResponse;
