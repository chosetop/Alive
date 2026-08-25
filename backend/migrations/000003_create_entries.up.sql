-- The core content table. One row is one thing the owner recorded.
--
-- Every record type shares this table and is told apart by the type column, so
-- "list everything by date" stays one query. A table per type would make that
-- an N-way UNION that grows whenever a type is added.
--
-- No category_id column here. It would reference a categories table that does
-- not exist yet, so it could carry no foreign key, and until 000004 arrives
-- nothing would stop a row pointing at a category id that never existed. The
-- column and the table it points at are created together in that migration.

CREATE TABLE entries (
    id           BIGSERIAL   PRIMARY KEY,

    -- RESTRICT, not CASCADE. Deleting an account must not take written content
    -- with it. Sessions cascade because a session is an attachment to the
    -- account; an entry is the point of the whole system.
    author_id    BIGINT      NOT NULL,

    -- The type discriminator. The CHECK below fixes the six accepted values.
    type         VARCHAR(32) NOT NULL DEFAULT 'journal',

    title        VARCHAR(255) NOT NULL,

    -- The public URL segment. Supplied by the client, never derived from the
    -- title here: deriving it from Chinese text yields either a percent-encoded
    -- URL nobody can read, or a pinyin dependency inside the backend for what is
    -- a presentation concern.
    slug         VARCHAR(255) NOT NULL,

    summary      TEXT,

    -- The Markdown source, and the only copy of the text. Rendering happens in
    -- the frontend, so no derived HTML is stored to fall out of step with it.
    content_md   TEXT        NOT NULL DEFAULT '',

    cover_url    TEXT,

    status       VARCHAR(16) NOT NULL DEFAULT 'draft',
    visibility   VARCHAR(16) NOT NULL DEFAULT 'public',

    -- Type-specific attributes. A book row carries an author and a rating, a
    -- travel row carries a place; forcing those into columns would give every
    -- row a wide band of NULLs. Shape is owned by the domain that owns the type.
    meta         JSONB       NOT NULL DEFAULT '{}',

    -- Computed on write so the list endpoint never has to read content_md.
    word_count   INT         NOT NULL DEFAULT 0,

    -- When the thing happened, as opposed to when it was typed. Backfilling a
    -- trip from three years ago sets created_at to today and happened_at to
    -- then. The public timeline sorts on this column, otherwise it would be a
    -- record of writing habits rather than of a life.
    happened_at  TIMESTAMPTZ,

    -- First publication, and it stays put. Re-editing a published entry must not
    -- move it, or fixing a typo in a three-year-old entry would push it back to
    -- the top of the feed.
    published_at TIMESTAMPTZ,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Soft delete. Content is not regenerable, so a delete marks the row and
    -- every read filters on it.
    deleted_at   TIMESTAMPTZ,

    CONSTRAINT entries_author_id_fkey
        FOREIGN KEY (author_id)
        REFERENCES users (id)
        ON DELETE RESTRICT,

    -- The six accepted types, fixed in the database.
    --
    -- A seventh type needs a migration, and that cost is deliberate: it forces a
    -- new type through one explicit decision. Without the constraint a typo like
    -- 'joural' inserts happily and that row then vanishes from every query that
    -- filters by type, reporting no error anywhere.
    CONSTRAINT entries_type_check
        CHECK (type IN ('journal', 'book', 'movie', 'music', 'travel', 'photo')),

    -- status answers "is this finished", visibility answers "who may see it".
    -- Two columns rather than one enum: combined, they would produce
    -- draft / published / published_private / published_unlisted and keep
    -- growing. Apart, each is a small closed set.
    CONSTRAINT entries_status_check
        CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT entries_visibility_check
        CHECK (visibility IN ('public', 'private', 'unlisted')),

    -- A published entry always has a publication time. Without this the feed
    -- could hold rows that sort by a NULL date.
    CONSTRAINT entries_published_at_check
        CHECK (status <> 'published' OR published_at IS NOT NULL),

    -- The slug format the client must supply: lowercase letters, digits and
    -- single hyphens between them. No leading, trailing or doubled hyphen.
    --
    -- Checked here as well as in the domain. The domain check produces a usable
    -- message; this one holds for rows written by psql or by a later CLI command.
    CONSTRAINT entries_slug_format_check
        CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),

    CONSTRAINT entries_word_count_check
        CHECK (word_count >= 0)
);

-- set_updated_at() comes from 000001.
CREATE TRIGGER entries_set_updated_at
    BEFORE UPDATE ON entries
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Indexes.
--
-- These are partial indexes: the WHERE clause is part of the definition. Every
-- public read carries "deleted_at IS NULL AND status = 'published'", so folding
-- that into the index keeps it to the rows that are actually queried, and lets
-- the planner use it for exactly those queries.

-- The public feed: published and public, newest publication first.
CREATE INDEX idx_entries_public_feed
    ON entries (published_at DESC)
    WHERE deleted_at IS NULL
      AND status = 'published'
      AND visibility = 'public';

-- The life timeline, ordered by when things happened.
CREATE INDEX idx_entries_timeline
    ON entries (happened_at DESC)
    WHERE deleted_at IS NULL AND status = 'published';

-- Per-type pages: the bookshelf, the film list.
CREATE INDEX idx_entries_type
    ON entries (type, published_at DESC)
    WHERE deleted_at IS NULL AND status = 'published';

-- The admin list, which sees every status and sorts by last edit.
CREATE INDEX idx_entries_admin
    ON entries (updated_at DESC)
    WHERE deleted_at IS NULL;

-- Queries that reach inside meta.
CREATE INDEX idx_entries_meta ON entries USING GIN (meta);

-- The category index from the design belongs with category_id and arrives in
-- 000004 alongside it.

-- Slug uniqueness, ignoring deleted rows.
--
-- A partial unique index, not a table-level UNIQUE. With a plain unique
-- constraint, deleting /hello-world would reserve that slug forever, because a
-- row nobody can see would still hold it.
CREATE UNIQUE INDEX uk_entries_slug
    ON entries (slug)
    WHERE deleted_at IS NULL;
