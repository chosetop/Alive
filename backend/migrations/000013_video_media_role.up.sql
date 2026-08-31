ALTER TABLE entry_media ADD COLUMN role VARCHAR(32) NOT NULL DEFAULT 'body_image';
ALTER TABLE entry_media ADD CONSTRAINT entry_media_role_check CHECK (role IN ('body_image', 'cover', 'primary_video'));
CREATE UNIQUE INDEX uk_entry_media_primary_video ON entry_media (entry_id) WHERE role = 'primary_video';
CREATE UNIQUE INDEX uk_entry_media_cover ON entry_media (entry_id) WHERE role = 'cover';
