DROP INDEX IF EXISTS idx_entries_world_public_display_order;

DROP TRIGGER entries_set_updated_at ON entries;
CREATE TRIGGER entries_set_updated_at
  BEFORE UPDATE ON entries
  FOR EACH ROW
  EXECUTE FUNCTION set_updated_at();

ALTER TABLE entries DROP COLUMN display_order;
