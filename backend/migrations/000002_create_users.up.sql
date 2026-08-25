-- Owner account and its login sessions.
--
-- One account exists today. The table carries no role logic: see the note on
-- the role column below.

CREATE TABLE users (
    id            BIGSERIAL   PRIMARY KEY,
    -- citext, installed by 000001, makes the column case-insensitive. "Alice"
    -- and "alice" are then one account, and the unique constraint enforces that
    -- in the database. A varchar column would need every query to remember
    -- lower(), and one missed call would create a second "same" account.
    username      CITEXT      NOT NULL,
    -- No length limit. An Argon2id PHC string is around 100 characters, but it
    -- grows when the cost parameters are raised, and a fixed width would turn
    -- that routine change into a migration.
    password_hash TEXT        NOT NULL,
    -- Recorded, never read. Authentication asks one question, "is this session
    -- valid", and does not consult this column. It exists so that adding roles
    -- later does not require backfilling rows. Adding code that branches on it
    -- means building role-based access control, which is a separate decision.
    role          VARCHAR(16) NOT NULL DEFAULT 'owner',
    -- Public display name, unused by login.
    display_name  VARCHAR(64),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_username_key UNIQUE (username),
    CONSTRAINT users_username_length_check
        CHECK (length(username) BETWEEN 3 AND 64)
);

-- set_updated_at() comes from 000001. Keeping the timestamp in the database
-- means a row changed by psql or by a CLI command is stamped correctly too.
CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE sessions (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    -- SHA-256 of the token, 32 raw bytes. bytea stores those 32 bytes; hex in a
    -- text column would take 64 and add an encoding step to every comparison.
    -- The plaintext token exists only in the client's cookie.
    token_hash BYTEA       NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Observation only. Authentication does not compare either value: pinning a
    -- session to an IP drops the login when a phone changes cell tower, and
    -- pinning to a user agent drops it when the browser updates itself.
    user_agent TEXT,
    -- inet is a native type and holds IPv4 and IPv6 alike.
    ip         INET,

    -- Sessions belong to their user. Cascade is right here because a session is
    -- an attachment to the account, not independent data. Content tables will
    -- use RESTRICT instead, since a post must not disappear silently.
    CONSTRAINT sessions_user_id_fkey
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON DELETE CASCADE,

    -- The unique constraint creates its own index, which serves the
    -- authentication lookup by token_hash. A separate index on the same column
    -- would be a duplicate: extra writes on every insert, no extra reads.
    CONSTRAINT sessions_token_hash_key UNIQUE (token_hash)
);

-- Serves the periodic delete of expired rows.
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- Serves "end every session for this user" and the cascade delete.
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- No soft delete on sessions. Logging out deletes the row. A session is a
-- credential, not a record worth keeping; a retained dead credential only adds
-- a condition that every future query has to remember to filter on.
