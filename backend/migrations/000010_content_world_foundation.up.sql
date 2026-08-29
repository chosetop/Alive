ALTER TABLE entries DROP CONSTRAINT entries_type_check;
ALTER TABLE entries RENAME COLUMN type TO world;
UPDATE entries SET world = 'journal';
ALTER TABLE entries ALTER COLUMN world DROP DEFAULT;
ALTER TABLE entries ADD CONSTRAINT entries_world_check
  CHECK (world IN ('journal', 'saying', 'video'));
ALTER TABLE entries ADD COLUMN kind VARCHAR(32) NOT NULL DEFAULT '';

DROP INDEX uk_entries_slug;
CREATE UNIQUE INDEX uk_entries_world_slug
  ON entries (world, slug)
  WHERE deleted_at IS NULL AND slug <> '';
DROP INDEX idx_entries_type;
CREATE INDEX idx_entries_world
  ON entries (world, published_at DESC)
  WHERE deleted_at IS NULL AND status = 'published';

DROP INDEX idx_entries_public_feed;
DROP INDEX idx_entries_timeline;
DROP INDEX idx_entries_public_timeline;
DROP INDEX idx_entries_category;
CREATE INDEX idx_entries_world_public_timeline
  ON entries (world, COALESCE(happened_at, published_at) DESC, id DESC)
  WHERE deleted_at IS NULL
    AND status = 'published'
    AND visibility = 'public';
CREATE INDEX idx_entries_world_category_public
  ON entries (world, category_id, COALESCE(happened_at, published_at) DESC, id DESC)
  WHERE deleted_at IS NULL
    AND status = 'published'
    AND visibility = 'public';

ALTER TABLE categories ADD COLUMN world VARCHAR(32) NOT NULL DEFAULT 'journal';
ALTER TABLE categories ALTER COLUMN world DROP DEFAULT;
ALTER TABLE categories ADD CONSTRAINT categories_world_check
  CHECK (world IN ('journal', 'saying', 'video'));
ALTER TABLE categories DROP CONSTRAINT categories_slug_key;
ALTER TABLE categories ADD CONSTRAINT categories_world_slug_key UNIQUE (world, slug);
ALTER TABLE categories ADD CONSTRAINT categories_id_world_key UNIQUE (id, world);

ALTER TABLE entries DROP CONSTRAINT entries_category_id_fkey;
ALTER TABLE entries ADD CONSTRAINT entries_category_world_fkey
  FOREIGN KEY (category_id, world) REFERENCES categories (id, world)
  ON DELETE SET NULL (category_id);

CREATE OR REPLACE FUNCTION prevent_world_change()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.world IS DISTINCT FROM OLD.world THEN
    RAISE EXCEPTION 'world is immutable';
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER entries_prevent_world_change
  BEFORE UPDATE ON entries
  FOR EACH ROW
  EXECUTE FUNCTION prevent_world_change();

CREATE TRIGGER categories_prevent_world_change
  BEFORE UPDATE ON categories
  FOR EACH ROW
  EXECUTE FUNCTION prevent_world_change();

CREATE TABLE site_worlds (
  world VARCHAR(32) PRIMARY KEY,
  status VARCHAR(16) NOT NULL,
  nav_label VARCHAR(64) NOT NULL,
  sort_order INT NOT NULL,
  default_view VARCHAR(32) NOT NULL DEFAULT '',
  revision BIGINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT site_worlds_world_check
    CHECK (world IN ('journal', 'saying', 'video')),
  CONSTRAINT site_worlds_status_check
    CHECK (status IN ('unopened', 'open', 'hidden')),
  CONSTRAINT site_worlds_default_view_check
    CHECK (default_view IN ('', 'stream', 'wall', 'focus')),
  CONSTRAINT site_worlds_nav_label_length_check
    CHECK (length(nav_label) BETWEEN 1 AND 64)
);

CREATE TRIGGER site_worlds_set_updated_at
  BEFORE UPDATE ON site_worlds
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO site_worlds (world, status, nav_label, sort_order, default_view)
VALUES
  ('journal', 'open', '日志', 10, 'stream'),
  ('saying', 'unopened', '片语', 20, 'stream'),
  ('video', 'unopened', '影像', 30, 'wall');

