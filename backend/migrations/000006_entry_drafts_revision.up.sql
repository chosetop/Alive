ALTER TABLE entries ALTER COLUMN title SET DEFAULT '';
ALTER TABLE entries ALTER COLUMN slug SET DEFAULT '';
ALTER TABLE entries ADD COLUMN revision BIGINT NOT NULL DEFAULT 1;

DROP INDEX uk_entries_slug;
ALTER TABLE entries DROP CONSTRAINT entries_slug_format_check;
ALTER TABLE entries ADD CONSTRAINT entries_slug_format_check
  CHECK (slug = '' OR slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$');
ALTER TABLE entries ADD CONSTRAINT entries_published_title_check
  CHECK (status <> 'published' OR title <> '');
ALTER TABLE entries ADD CONSTRAINT entries_published_slug_check
  CHECK (status <> 'published' OR slug <> '');
ALTER TABLE entries ADD CONSTRAINT entries_published_content_check
  CHECK (status <> 'published' OR btrim(content_md) <> '');
CREATE UNIQUE INDEX uk_entries_slug
  ON entries (slug)
  WHERE deleted_at IS NULL AND slug <> '';
