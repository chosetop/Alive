-- Reverses 000004_create_categories.up.sql.
--
-- Order matters: the column on entries goes first. Dropping the table while
-- entries still references it would fail on the foreign key, and DROP TABLE
-- ... CASCADE would take the constraint with it but leave a category_id column
-- full of ids pointing at nothing.
--
-- DROP COLUMN removes idx_entries_category and entries_category_id_fkey along
-- with it, so neither needs its own statement.
ALTER TABLE entries DROP COLUMN IF EXISTS category_id;

DROP TABLE IF EXISTS categories;
