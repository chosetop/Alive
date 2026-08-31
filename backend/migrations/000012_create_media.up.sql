CREATE TABLE media (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL UNIQUE,
    mime_type VARCHAR(128) NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size > 0),
    width INTEGER,
    height INTEGER,
    duration_seconds INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE entry_media (
    entry_id BIGINT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    media_id BIGINT NOT NULL REFERENCES media(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entry_id, media_id)
);

CREATE INDEX idx_entry_media_media ON entry_media (media_id);
