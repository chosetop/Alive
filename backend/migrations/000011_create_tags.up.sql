-- Global tags and the join table from entries.
--
-- Tags are global across worlds. Entries can hold many tags, so the relation
-- is many-to-many rather than a column on entries.

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE tags (
    id          BIGSERIAL   PRIMARY KEY,
    name        CITEXT      NOT NULL,
    slug        VARCHAR(160) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tags_name_key UNIQUE (name),
    CONSTRAINT tags_slug_key UNIQUE (slug),
    CONSTRAINT tags_name_length_check
        CHECK (length(name) BETWEEN 1 AND 64),
    CONSTRAINT tags_slug_length_check
        CHECK (length(slug) BETWEEN 1 AND 160),
    CONSTRAINT tags_slug_format_check
        CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE TRIGGER tags_set_updated_at
    BEFORE UPDATE ON tags
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE entry_tags (
    entry_id   BIGINT      NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    tag_id     BIGINT      NOT NULL REFERENCES tags(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (entry_id, tag_id)
);

CREATE INDEX idx_entry_tags_tag_entry ON entry_tags (tag_id, entry_id);
