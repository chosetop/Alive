-- Queries for the owner account and its sessions.
--
-- These return database facts and nothing more. Whether a session has expired
-- is a decision, so it is not encoded here: GetSessionByHash returns an expired
-- row and lets the service layer rule on it. A SQL-level expiry filter would
-- make "expired" and "no such session" indistinguishable to the caller.

-- name: CreateUser :one
-- password_hash is written but not returned. A hash has no use above the
-- repository except during a login comparison, which GetUserByUsername serves.
INSERT INTO users (
    username,
    password_hash,
    role,
    display_name
) VALUES (
    $1, $2, $3, $4
)
RETURNING
    id,
    username,
    role,
    display_name,
    created_at,
    updated_at;

-- name: GetUserByUsername :one
-- The login lookup, and the only query that exposes password_hash.
--
-- No lower() on either side: username is citext, so the comparison is already
-- case-insensitive. Wrapping the column in lower() would also make the query
-- unable to use the unique index.
SELECT
    id,
    username,
    password_hash,
    role,
    display_name,
    created_at,
    updated_at
FROM users
WHERE username = $1;

-- name: GetUserByID :one
SELECT
    id,
    username,
    role,
    display_name,
    created_at,
    updated_at
FROM users
WHERE id = $1;

-- name: CreateSession :one
-- token_hash is supplied by the caller, already hashed. The plaintext token
-- never reaches the database.
INSERT INTO sessions (
    user_id,
    token_hash,
    expires_at,
    user_agent,
    ip
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING
    id,
    user_id,
    expires_at,
    created_at;

-- name: GetSessionByHash :one
-- The authentication lookup. One round trip returns the session and its user,
-- because this runs on every protected request and a second query would double
-- that cost.
--
-- Deliberately no "AND expires_at > now()". The row comes back whatever its
-- expiry, and the service decides. That keeps two different outcomes apart:
-- an unknown token and a known but expired one.
SELECT
    s.id,
    s.user_id,
    s.token_hash,
    s.expires_at,
    s.created_at,
    s.user_agent,
    s.ip,
    u.id           AS user_id_ref,
    u.username     AS user_username,
    u.role         AS user_role,
    u.display_name AS user_display_name,
    u.created_at   AS user_created_at,
    u.updated_at   AS user_updated_at
FROM sessions s
INNER JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: TouchSession :exec
-- Sliding renewal. The service calls this only when a session is past the
-- halfway point of its lifetime, so an active session is not one write per
-- request.
UPDATE sessions
SET expires_at = $2
WHERE id = $1;

-- name: DeleteSession :exec
-- Logout. Deleting a missing row is not an error here; the caller's goal is
-- that the session no longer exists, and it does not.
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteSessionByHash :exec
-- Logout. The request carries only the cookie, so deleting by hash ends the
-- session in one round trip instead of a lookup followed by a delete by id.
-- DeleteSession stays for callers that already hold the id.
DELETE FROM sessions
WHERE token_hash = $1;

-- name: DeleteExpiredSessions :execrows
-- Periodic cleanup, driven by a CLI command. Returns the number of rows removed
-- so the command can report it.
DELETE FROM sessions
WHERE expires_at <= now();
