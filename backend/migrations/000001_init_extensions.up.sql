-- Baseline migration. Creates no table.
--
-- It exists so the migration chain has a first link and so `migrate up` on a
-- fresh database produces the schema_migrations bookkeeping table. Business
-- tables arrive in later migrations.

-- citext gives case-insensitive text columns. Needed later for the owner
-- username, so a login does not depend on capitalisation.
CREATE EXTENSION IF NOT EXISTS citext;

-- Sets updated_at to now() on every UPDATE.
--
-- Defined once here rather than repeated per table, and enforced by the
-- database rather than by application code: a row changed by a manual psql
-- statement or a future CLI command gets a correct timestamp either way.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
