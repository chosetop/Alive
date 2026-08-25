-- Reverses 000001_init_extensions.up.sql.
--
-- Every migration ships a down file. A migration that cannot be undone is
-- found out during an incident, which is the worst time to find out.

DROP FUNCTION IF EXISTS set_updated_at();

-- The extension is left in place on purpose. Dropping it would fail, or
-- silently cascade, if anything created outside this chain came to depend on
-- it. Removing an extension is a manual decision, not an automatic rollback.
