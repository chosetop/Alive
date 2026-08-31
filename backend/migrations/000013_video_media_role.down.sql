DROP INDEX IF EXISTS uk_entry_media_primary_video;
DROP INDEX IF EXISTS uk_entry_media_cover;
ALTER TABLE entry_media DROP CONSTRAINT IF EXISTS entry_media_role_check;
ALTER TABLE entry_media DROP COLUMN IF EXISTS role;
