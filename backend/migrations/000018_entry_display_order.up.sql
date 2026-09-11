ALTER TABLE entries ADD COLUMN display_order BIGINT NOT NULL DEFAULT 0;

DROP TRIGGER entries_set_updated_at ON entries;

WITH ranked AS (
  SELECT
    id,
    row_number() OVER (
      PARTITION BY world
      ORDER BY COALESCE(happened_at, published_at) ASC NULLS FIRST, id ASC
    ) AS position
  FROM entries
  WHERE deleted_at IS NULL AND status = 'published'
)
UPDATE entries
SET display_order = ranked.position
FROM ranked
WHERE entries.id = ranked.id;

CREATE TRIGGER entries_set_updated_at
  BEFORE UPDATE ON entries
  FOR EACH ROW
  WHEN ((to_jsonb(OLD) - 'display_order') IS DISTINCT FROM (to_jsonb(NEW) - 'display_order'))
  EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_entries_world_public_display_order
  ON entries (world, display_order DESC, id DESC)
  WHERE deleted_at IS NULL
    AND status = 'published'
    AND visibility = 'public';
