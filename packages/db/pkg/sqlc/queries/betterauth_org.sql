--
-- BetterAuth Organizations
--

-- name: AuthGetOrganization :one
SELECT
  id,
  name,
  slug,
  logo,
  metadata,
  created_at
FROM
  auth_organization
WHERE
  id = ?
LIMIT
  1;

-- name: AuthGetOrganizationBySlug :one
SELECT
  id,
  name,
  slug,
  logo,
  metadata,
  created_at
FROM
  auth_organization
WHERE
  slug = ?
LIMIT
  1;

-- name: AuthCreateOrganization :exec
INSERT INTO
  auth_organization (id, name, slug, logo, metadata, created_at)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: AuthUpdateOrganization :exec
UPDATE auth_organization
SET
  name = ?,
  slug = ?,
  logo = ?,
  metadata = ?
WHERE
  id = ?;

-- name: AuthDeleteOrganization :exec
DELETE FROM auth_organization
WHERE
  id = ?;

-- name: AuthCountOrganizations :one
SELECT
  COUNT(*)
FROM
  auth_organization;

--
-- Members
--

-- name: AuthGetMember :one
SELECT
  id,
  user_id,
  organization_id,
  role,
  created_at
FROM
  auth_member
WHERE
  id = ?
LIMIT
  1;

-- name: AuthGetMemberByUserAndOrg :one
SELECT
  id,
  user_id,
  organization_id,
  role,
  created_at
FROM
  auth_member
WHERE
  user_id = ?
  AND organization_id = ?
LIMIT
  1;

-- name: AuthListMembersByOrganization :many
SELECT
  id,
  user_id,
  organization_id,
  role,
  created_at
FROM
  auth_member
WHERE
  organization_id = ?;

-- name: AuthListMembersByUser :many
SELECT
  id,
  user_id,
  organization_id,
  role,
  created_at
FROM
  auth_member
WHERE
  user_id = ?;

-- name: AuthCreateMember :exec
INSERT INTO
  auth_member (id, user_id, organization_id, role, created_at)
VALUES
  (?, ?, ?, ?, ?);

-- name: AuthUpdateMember :exec
UPDATE auth_member
SET
  role = ?
WHERE
  id = ?;

-- name: AuthDeleteMember :exec
DELETE FROM auth_member
WHERE
  id = ?;

-- name: AuthDeleteMembersByOrganization :exec
DELETE FROM auth_member
WHERE
  organization_id = ?;

-- name: AuthDeleteMembersByUser :exec
DELETE FROM auth_member
WHERE
  user_id = ?;

--
-- Invitations
--

-- name: AuthGetInvitation :one
SELECT
  id,
  email,
  inviter_id,
  organization_id,
  role,
  status,
  created_at,
  expires_at
FROM
  auth_invitation
WHERE
  id = ?
LIMIT
  1;

-- name: AuthListInvitationsByOrganization :many
SELECT
  id,
  email,
  inviter_id,
  organization_id,
  role,
  status,
  created_at,
  expires_at
FROM
  auth_invitation
WHERE
  organization_id = ?;

-- name: AuthListInvitationsByEmail :many
SELECT
  id,
  email,
  inviter_id,
  organization_id,
  role,
  status,
  created_at,
  expires_at
FROM
  auth_invitation
WHERE
  email = ?;

-- name: AuthCreateInvitation :exec
INSERT INTO
  auth_invitation (id, email, inviter_id, organization_id, role, status, created_at, expires_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: AuthUpdateInvitation :exec
UPDATE auth_invitation
SET
  status = ?
WHERE
  id = ?;

-- name: AuthDeleteInvitation :exec
DELETE FROM auth_invitation
WHERE
  id = ?;

-- name: AuthDeleteInvitationsByOrganization :exec
DELETE FROM auth_invitation
WHERE
  organization_id = ?;
