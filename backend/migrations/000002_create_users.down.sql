-- Reverses 000002_create_users.up.sql.
--
-- Reverse dependency order: sessions references users, so sessions goes first.
-- Dropping users first would fail on the foreign key.
--
-- DROP TABLE removes the table's own indexes, constraints and triggers, so the
-- indexes and the users trigger need no separate statement.

DROP TABLE IF EXISTS sessions;

DROP TABLE IF EXISTS users;
