DROP INDEX sessions_absolute_expires_at_idx;
ALTER TABLE sessions DROP COLUMN absolute_expires_at;
