DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM entries
    WHERE deleted_at IS NULL
      AND slug <> ''
    GROUP BY slug
    HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'cannot roll back content worlds while live entries have duplicate slugs across worlds';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM categories
    GROUP BY slug
    HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'cannot roll back content worlds while categories have duplicate slugs across worlds';
  END IF;
END;
$$;

DROP TRIGGER categories_prevent_world_change ON categories;
DROP TRIGGER entries_prevent_world_change ON entries;
DROP TRIGGER site_worlds_set_updated_at ON site_worlds;

DROP INDEX idx_entries_world_category_public;
DROP INDEX idx_entries_world_public_timeline;
DROP INDEX idx_entries_world;
DROP INDEX uk_entries_world_slug;

ALTER TABLE entries DROP CONSTRAINT entries_category_world_fkey;
ALTER TABLE entries DROP CONSTRAINT entries_world_check;
ALTER TABLE entries DROP COLUMN kind;
ALTER TABLE entries DROP COLUMN world;

CREATE UNIQUE INDEX uk_entries_slug
  ON entries (slug)
  WHERE deleted_at IS NULL AND slug <> '';

ALTER TABLE categories DROP CONSTRAINT categories_id_world_key;
ALTER TABLE categories DROP CONSTRAINT categories_world_slug_key;
ALTER TABLE categories DROP CONSTRAINT categories_world_check;
ALTER TABLE categories ADD CONSTRAINT categories_slug_key UNIQUE (slug);
ALTER TABLE categories DROP COLUMN world;

ALTER TABLE entries ADD CONSTRAINT entries_category_id_fkey
  FOREIGN KEY (category_id)
  REFERENCES categories (id)
  ON DELETE SET NULL;

DROP TABLE site_worlds;
DROP FUNCTION prevent_world_change();
