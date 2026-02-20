--
-- BetterAuth
--

-- name: AuthGetUser :one
SELECT
  id,
  name,
  email,
  email_verified,
  image,
  created_at,
  updated_at
FROM
  auth_user
WHERE
  id = ?
LIMIT
  1;

-- name: AuthGetUserByEmail :one
SELECT
  id,
  name,
  email,
  email_verified,
  image,
  created_at,
  updated_at
FROM
  auth_user
WHERE
  email = ?
LIMIT
  1;

-- name: AuthCreateUser :exec
INSERT INTO
  auth_user (id, name, email, email_verified, image, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: AuthUpdateUser :exec
UPDATE auth_user
SET
  name = ?,
  email = ?,
  email_verified = ?,
  image = ?,
  updated_at = ?
WHERE
  id = ?;

-- name: AuthDeleteUser :exec
DELETE FROM auth_user
WHERE
  id = ?;

-- name: AuthCountUsers :one
SELECT
  COUNT(*)
FROM
  auth_user;

--
-- Sessions
--

-- name: AuthGetSession :one
SELECT
  id,
  user_id,
  token,
  expires_at,
  ip_address,
  user_agent,
  created_at,
  updated_at
FROM
  auth_session
WHERE
  id = ?
LIMIT
  1;

-- name: AuthGetSessionByToken :one
SELECT
  id,
  user_id,
  token,
  expires_at,
  ip_address,
  user_agent,
  created_at,
  updated_at
FROM
  auth_session
WHERE
  token = ?
LIMIT
  1;

-- name: AuthListSessionsByUser :many
SELECT
  id,
  user_id,
  token,
  expires_at,
  ip_address,
  user_agent,
  created_at,
  updated_at
FROM
  auth_session
WHERE
  user_id = ?;

-- name: AuthCreateSession :exec
INSERT INTO
  auth_session (
    id,
    user_id,
    token,
    expires_at,
    ip_address,
    user_agent,
    created_at,
    updated_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: AuthUpdateSession :exec
UPDATE auth_session
SET
  expires_at = ?,
  ip_address = ?,
  user_agent = ?,
  updated_at = ?
WHERE
  id = ?;

-- name: AuthDeleteSession :exec
DELETE FROM auth_session
WHERE
  id = ?;

-- name: AuthDeleteSessionByToken :exec
DELETE FROM auth_session
WHERE
  token = ?;

-- name: AuthDeleteSessionsByUser :exec
DELETE FROM auth_session
WHERE
  user_id = ?;

-- name: AuthDeleteExpiredSessions :exec
DELETE FROM auth_session
WHERE
  expires_at < ?;

--
-- Accounts
--

-- name: AuthGetAccount :one
SELECT
  id,
  user_id,
  account_id,
  provider_id,
  access_token,
  refresh_token,
  access_token_expires_at,
  refresh_token_expires_at,
  scope,
  id_token,
  password,
  created_at,
  updated_at
FROM
  auth_account
WHERE
  id = ?
LIMIT
  1;

-- name: AuthGetAccountByProvider :one
SELECT
  id,
  user_id,
  account_id,
  provider_id,
  access_token,
  refresh_token,
  access_token_expires_at,
  refresh_token_expires_at,
  scope,
  id_token,
  password,
  created_at,
  updated_at
FROM
  auth_account
WHERE
  provider_id = ?
  AND account_id = ?
LIMIT
  1;

-- name: AuthListAccountsByUser :many
SELECT
  id,
  user_id,
  account_id,
  provider_id,
  access_token,
  refresh_token,
  access_token_expires_at,
  refresh_token_expires_at,
  scope,
  id_token,
  password,
  created_at,
  updated_at
FROM
  auth_account
WHERE
  user_id = ?;

-- name: AuthCreateAccount :exec
INSERT INTO
  auth_account (
    id,
    user_id,
    account_id,
    provider_id,
    access_token,
    refresh_token,
    access_token_expires_at,
    refresh_token_expires_at,
    scope,
    id_token,
    password,
    created_at,
    updated_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: AuthUpdateAccount :exec
UPDATE auth_account
SET
  access_token = ?,
  refresh_token = ?,
  access_token_expires_at = ?,
  refresh_token_expires_at = ?,
  scope = ?,
  id_token = ?,
  password = ?,
  updated_at = ?
WHERE
  id = ?;

-- name: AuthDeleteAccount :exec
DELETE FROM auth_account
WHERE
  id = ?;

-- name: AuthDeleteAccountsByUser :exec
DELETE FROM auth_account
WHERE
  user_id = ?;

--
-- Verifications
--

-- name: AuthGetVerification :one
SELECT
  id,
  identifier,
  value,
  expires_at,
  created_at,
  updated_at
FROM
  auth_verification
WHERE
  id = ?
LIMIT
  1;

-- name: AuthGetVerificationByIdentifier :one
SELECT
  id,
  identifier,
  value,
  expires_at,
  created_at,
  updated_at
FROM
  auth_verification
WHERE
  identifier = ?
LIMIT
  1;

-- name: AuthCreateVerification :exec
INSERT INTO
  auth_verification (id, identifier, value, expires_at, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: AuthDeleteVerification :exec
DELETE FROM auth_verification
WHERE
  id = ?;

-- name: AuthDeleteExpiredVerifications :exec
DELETE FROM auth_verification
WHERE
  expires_at < ?;

--
-- JWKS
--

-- name: AuthCreateJwks :exec
INSERT INTO
  auth_jwks (id, public_key, private_key, created_at, expires_at)
VALUES
  (?, ?, ?, ?, ?);

-- name: AuthListJwks :many
SELECT
  id,
  public_key,
  private_key,
  created_at,
  expires_at
FROM
  auth_jwks
ORDER BY
  created_at DESC;

-- name: AuthDeleteJwks :exec
DELETE FROM auth_jwks
WHERE
  id = ?;
