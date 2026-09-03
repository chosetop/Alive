ALTER TABLE entries DROP CONSTRAINT entries_published_title_check;

ALTER TABLE entries ADD CONSTRAINT entries_published_title_check
  CHECK (status <> 'published' OR world = 'saying' OR title <> '');
