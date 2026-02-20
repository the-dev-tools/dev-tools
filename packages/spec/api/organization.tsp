using DevTools;

namespace Api.Organization;

enum OrganizationRole {
  Owner,
  Admin,
  Member,
}

enum InvitationStatus {
  Pending,
  Accepted,
  Rejected,
  Canceled,
}

@TanStackDB.collection(#{ canCreate: false, canDelete: false })
model Organization {
  @primaryKey organizationId: Id;
  name: string;
  slug?: string;
  logo?: string;
  metadata?: string;
  @visibility(Lifecycle.Read) createdAt?: int64;
}

@TanStackDB.collection(#{ isReadOnly: true })
model OrganizationMember {
  @primaryKey memberId: Id;
  @foreignKey organizationId: Id;
  @foreignKey userId: Id;
  role: OrganizationRole;
  @visibility(Lifecycle.Read) createdAt?: int64;
}

@TanStackDB.collection(#{ isReadOnly: true })
model OrganizationInvitation {
  @primaryKey invitationId: Id;
  @foreignKey organizationId: Id;
  @foreignKey inviterId: Id;
  email: string;
  role: OrganizationRole;
  status: InvitationStatus;
  @visibility(Lifecycle.Read) createdAt?: int64;
  expiresAt: int64;
}
