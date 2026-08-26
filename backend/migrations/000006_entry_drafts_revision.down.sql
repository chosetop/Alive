DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM entries
    WHERE deleted_at IS NULL AND (title = '' OR slug = '')
  ) THEN
    RAISE EXCEPTION 'cannot roll back entry drafts revision while live entries have empty title or slug';
  END IF;
END $$;

DROP INDEX uk_entries_slug;
ALTER TABLE entries DROP CONSTRAINT entries_published_content_check;
ALTER TABLE entries DROP CONSTRAINT entries_published_slug_check;
ALTER TABLE entries DROP CONSTRAINT entries_published_title_check;
ALTER TABLE entries DROP CONSTRAINT entries_slug_format_check;
ALTER TABLE entries DROP COLUMN revision;
ALTER TABLE entries ADD CONSTRAINT entries_slug_format_check
  CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$');
ALTER TABLE entries ALTER COLUMN title DROP DEFAULT;
ALTER TABLE entries ALTER COLUMN slug DROP DEFAULT;
CREATE UNIQUE INDEX uk_entries_slug
  ON entries (slug)
  WHERE deleted_at IS NULL;
