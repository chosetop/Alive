-- Bound every session to a fixed maximum age from login, while preserving
-- the existing sliding expires_at within that boundary.
ALTER TABLE sessions
    ADD COLUMN absolute_expires_at TIMESTAMPTZ;

UPDATE sessions
SET absolute_expires_at = created_at + INTERVAL '30 days';

ALTER TABLE sessions
    ALTER COLUMN absolute_expires_at SET NOT NULL;

CREATE INDEX sessions_absolute_expires_at_idx ON sessions (absolute_expires_at);
