-- Reverses 000003_create_entries.up.sql.
--
-- DROP TABLE takes the table's own indexes, constraints and trigger with it, so
-- none of those need a statement of their own. Nothing references entries yet:
-- entry_tags and entry_media arrive in later migrations and will be dropped by
-- their own down files first.

DROP TABLE IF EXISTS entries;
