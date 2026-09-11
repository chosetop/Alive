ALTER TABLE entries DROP CONSTRAINT entries_published_content_check;

ALTER TABLE entries ADD CONSTRAINT entries_published_content_check
  CHECK (status <> 'published' OR btrim(content_md) <> '');
