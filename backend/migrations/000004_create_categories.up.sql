-- Categories, and the column on entries that points at them.
--
-- One migration for both, which is why 000003 left the column out. A
-- category_id created before this table could carry no foreign key, so nothing
-- would have stopped a row referencing a category that never existed, and the
-- column would have looked usable while guaranteeing nothing.
--
-- A category answers "what is this record, essentially". An entry has at most
-- one, so this is N:1 and the id lives on entries. Tags answer a different
-- question, "what is this about", are many per entry, and arrive later as their
-- own table plus a join table. Merging the two would make the navigation
-- structure grow without bound.

CREATE TABLE categories (
    id          BIGSERIAL   PRIMARY KEY,

    -- The display name. Free text, so Chinese names need no transliteration.
    name        VARCHAR(64) NOT NULL,

    -- The URL segment, supplied rather than derived, for the same reason as the
    -- entry slug: deriving it from Chinese text yields either a percent-encoded
    -- URL nobody can read or a pinyin dependency in the backend.
    slug        VARCHAR(64) NOT NULL,

    -- Used on the category page, and as its meta description.
    description TEXT,

    -- Manual ordering. Categories are a navigation structure, and the order that
    -- suits a reader is not alphabetical and not by creation date.
    sort_order  INT         NOT NULL DEFAULT 0,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A plain table-level UNIQUE, unlike uk_entries_slug.
    --
    -- Categories are deleted physically, not marked, so there is no deleted_at
    -- to exclude and no row nobody can see holding a slug hostage.
    CONSTRAINT categories_slug_key UNIQUE (slug),

    CONSTRAINT categories_name_length_check
        CHECK (length(name) BETWEEN 1 AND 64),

    -- The same slug format as entries: lowercase letters and digits in groups
    -- joined by single hyphens. No leading, trailing or doubled hyphen.
    CONSTRAINT categories_slug_format_check
        CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

-- set_updated_at() comes from 000001.
CREATE TRIGGER categories_set_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Serves the category list, which is ordered for display rather than by id.
CREATE INDEX idx_categories_sort ON categories (sort_order, id);

-- The column 000003 documented and deferred.
--
-- Nullable, and NULL is a legitimate state meaning "uncategorised", not missing
-- data. Requiring a category would mean inventing one before the first entry
-- could be written.
ALTER TABLE entries
    ADD COLUMN category_id BIGINT;

-- ON DELETE SET NULL, so deleting a category leaves its entries uncategorised
-- rather than refusing the delete or removing the content.
--
-- The cost is that the delete is silent: afterwards nothing in the database
-- records which category those rows used to point at. That is accepted here
-- because a category holds only a name and a slug, so recreating one is cheap;
-- it is not accepted for entries, whose text is not regenerable and which are
-- therefore soft deleted instead.
ALTER TABLE entries
    ADD CONSTRAINT entries_category_id_fkey
        FOREIGN KEY (category_id)
        REFERENCES categories (id)
        ON DELETE SET NULL;

-- The sixth partial index from the design, which needed this column to exist.
--
-- Same WHERE clause as idx_entries_type, matching what the category page
-- queries: published rows, newest publication first, within one category.
CREATE INDEX idx_entries_category
    ON entries (category_id, published_at DESC)
    WHERE deleted_at IS NULL AND status = 'published';
