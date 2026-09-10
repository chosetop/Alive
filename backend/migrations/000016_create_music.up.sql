CREATE TABLE music_assets (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT NOT NULL REFERENCES users(id),
    object_key TEXT NOT NULL UNIQUE,
    url TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('audio', 'image')),
    mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, author_id)
);
CREATE TABLE music_tracks (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT NOT NULL REFERENCES users(id),
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    artist TEXT NOT NULL DEFAULT '' CHECK (char_length(artist) <= 200),
    audio_asset_id BIGINT NOT NULL,
    cover_asset_id BIGINT,
    duration DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (duration >= 0 AND duration <= 86400),
    revision BIGINT NOT NULL DEFAULT 1,
    UNIQUE (id, author_id),
    FOREIGN KEY (audio_asset_id, author_id) REFERENCES music_assets(id, author_id),
    FOREIGN KEY (cover_asset_id, author_id) REFERENCES music_assets(id, author_id)
);
CREATE TABLE music_playlists (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    cover_asset_id BIGINT,
    is_public BOOLEAN NOT NULL DEFAULT false,
    is_default BOOLEAN NOT NULL DEFAULT false CHECK (NOT is_default OR is_public),
    revision BIGINT NOT NULL DEFAULT 1,
    UNIQUE (id, author_id),
    FOREIGN KEY (cover_asset_id, author_id) REFERENCES music_assets(id, author_id)
);
CREATE UNIQUE INDEX music_one_default ON music_playlists ((true)) WHERE is_default;
CREATE INDEX music_tracks_author ON music_tracks (author_id, id);
CREATE INDEX music_playlists_author ON music_playlists (author_id, id);
CREATE TABLE music_playlist_tracks (
    playlist_id BIGINT NOT NULL,
    track_id BIGINT NOT NULL,
    author_id BIGINT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (playlist_id, track_id),
    UNIQUE (playlist_id, position),
    FOREIGN KEY (playlist_id, author_id) REFERENCES music_playlists(id, author_id) ON DELETE CASCADE,
    FOREIGN KEY (track_id, author_id) REFERENCES music_tracks(id, author_id) ON DELETE CASCADE
);
CREATE INDEX music_playlist_tracks_track ON music_playlist_tracks (track_id);
