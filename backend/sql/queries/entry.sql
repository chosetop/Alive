-- Queries for entries.
--
-- The visibility rule is in SQL, not in Go. Every public read carries
-- "deleted_at IS NULL AND status = 'published' AND visibility = 'public'" as
-- part of the statement, so a caller cannot reach an unpublished row by
-- forgetting a filter. The three conditions also match the partial index
-- idx_entries_public_feed exactly, so the planner uses it for these queries.
--
-- The admin queries, which do see drafts, are separate statements rather than a
-- flag on these. A boolean that switches the visibility filter on and off is one
-- wrong argument away from publishing every draft. They are named Admin* and
-- appear at the end of this file; no query serves both audiences.
--
-- GetLinkEntryBySlug is the one public read that accepts 'unlisted' as well as
-- 'public'. It is a separate statement for the reason above, not a parameter on
-- GetPublicEntryBySlug: an entry reachable by link must stay out of the lists,
-- and a shared query with a flag would make that depend on the argument.
--
-- The reads LEFT JOIN categories to carry the category name and slug, so one
-- response needs one query. The writes cannot: RETURNING sees only the row being
-- written, so they hand back category_id alone and a caller that needs the name
-- reads the entry back. That asymmetry is in the generated types too, which is
-- why the domain has two conversion paths.

-- name: CreateEntry :one
-- Insert one entry.
--
-- published_at is a parameter rather than a now() call, because "first
-- published" is a decision the service makes: it is set when an entry is created
-- as published, and never rewritten afterwards. Passing now() here would move it
-- on every write.
--
-- word_count likewise arrives computed. Counting words in SQL would tie the
-- definition of a word to Postgres' text functions, and the domain owns that
-- rule.
-- category_id is nullable and NULL means uncategorised, which is a normal state
-- rather than missing data. The foreign key refuses an id no category holds, so a
-- typo here is a constraint violation the repository translates, not a row
-- pointing at nothing.
INSERT INTO entries (
    author_id,
    category_id,
    type,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING
    id,
    revision,
    author_id,
    category_id,
    type,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: GetPublicEntryBySlug :one
-- The strictly public detail read, addressed by slug.
--
-- Returns content_md: the detail endpoints are the ones that need the body.
--
-- Kept alongside GetLinkEntryBySlug rather than replaced by it. This one is what
-- a sitemap generator or a feed builder should ask, because those enumerate what
-- is meant to be found, and an unlisted entry is not.
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.type,
    e.title,
    e.slug,
    e.summary,
    e.content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c ON c.id = e.category_id
WHERE e.slug = sqlc.arg(slug)
  AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND e.visibility = 'public';

-- name: GetLinkEntryBySlug :one
-- The detail read behind a shared link: public or unlisted.
--
-- This is what GET /entries/:slug uses, so that an unlisted entry can be opened
-- by anyone holding its URL while staying out of every list, count and sitemap.
-- The lists above are unchanged and still say visibility = 'public'.
--
-- Note what unlisted does and does not buy. A slug is human readable and
-- guessable, so this is "not advertised", not access control: it keeps an entry
-- off the front page and out of search engines, and that is all. An entry that
-- must not be readable by a stranger is 'private', which no public statement
-- accepts.
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.type,
    e.title,
    e.slug,
    e.summary,
    e.content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c ON c.id = e.category_id
WHERE e.slug = sqlc.arg(slug)
  AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND e.visibility IN ('public', 'unlisted');

-- name: ListPublicEntries :many
-- The public list, newest happening first.
--
-- No content_md in the column list. A list of twenty entries carrying twenty
-- Markdown bodies is a response tens of times larger than the page needs, and
-- nothing on a list view renders the body.
--
-- Ordered by happened_at, not created_at: the timeline records when things
-- happened, not when they were typed. Entries with no happened_at fall back to
-- published_at rather than sorting to the end, because that is the date the
-- reader is shown for them, and a list sorted by one date while labelled with
-- another puts a 2026 entry below the 2024 block.
--
-- COALESCE cannot be NULL on these rows: entries_published_at_check requires
-- published_at on every published entry, which is why there is no NULLS LAST.
-- id DESC breaks ties, which is what stops a row appearing on two pages when
-- several share a date.
--
-- idx_entries_public_timeline (000005) indexes this exact expression. Changing
-- the ORDER BY without changing that index turns the list into a full sort.
-- The category filter is a nullable id: NULL means every category. The caller
-- passes an id, not a slug, because the service resolves the slug first — that
-- way an unknown category is a 404 saying the URL is wrong, rather than an empty
-- list saying the category has nothing in it.
--
-- One thing this shape cannot express is "only the uncategorised ones", since
-- that would need a third state alongside "this category" and "all". No endpoint
-- asks for it: uncategorised is not a navigable page, having neither a name nor a
-- slug. Adding it later means a separate statement, not another argument.
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.type,
    e.title,
    e.slug,
    e.summary,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c ON c.id = e.category_id
WHERE e.deleted_at IS NULL
  AND e.status = 'published'
  AND e.visibility = 'public'
  AND (sqlc.narg(category_id)::bigint IS NULL
       OR e.category_id = sqlc.narg(category_id)::bigint)
ORDER BY COALESCE(e.happened_at, e.published_at) DESC, e.id DESC
LIMIT sqlc.arg(limit_) OFFSET sqlc.arg(offset_);

-- name: CountPublicEntries :one
-- The total for the pagination block, under the same filter as the list.
--
-- A separate statement rather than a window function on the list query. With
-- count(*) OVER () the total arrives only when at least one row does, so the
-- last page plus one would report a total of zero.
--
-- No join here: the count needs no category name, only the same filter.
SELECT count(*)
FROM entries
WHERE deleted_at IS NULL
  AND status = 'published'
  AND visibility = 'public'
  AND (sqlc.narg(category_id)::bigint IS NULL
       OR category_id = sqlc.narg(category_id)::bigint);

-- name: UpdateEntry :one
-- Apply a partial update to one live entry.
--
-- Every field is a pair: a boolean saying whether the caller asked for this
-- field, and the value to write. The boolean is not redundant with a NULL value,
-- and this is why COALESCE is not used here: summary, cover_url and happened_at
-- are nullable, so "clear this field" and "leave this field alone" both arrive as
-- NULL under COALESCE and become the same statement. With the flag, clearing a
-- summary is set_summary = true and summary = NULL, which is distinct from
-- set_summary = false.
--
-- status is absent on purpose. Publishing carries the published_at rule, and
-- PublishEntry below owns it; allowing status here would put that rule in two
-- places, and the CHECK constraint would reject the write anyway once status
-- became 'published' with a NULL published_at.
--
-- word_count travels with content_md rather than being its own parameter: it is
-- derived from the body, so accepting it separately would let the two disagree.
--
-- updated_at is left to the entries_set_updated_at trigger from 000001.
UPDATE entries
SET
    revision = revision + 1,
    type = CASE WHEN sqlc.arg(set_type)::boolean
                THEN sqlc.arg(type)::varchar ELSE type END,
    title = CASE WHEN sqlc.arg(set_title)::boolean
                 THEN sqlc.arg(title)::varchar ELSE title END,
    slug = CASE WHEN sqlc.arg(set_slug)::boolean
                THEN sqlc.arg(slug)::varchar ELSE slug END,
    summary = CASE WHEN sqlc.arg(set_summary)::boolean
                   THEN sqlc.narg(summary)::text ELSE summary END,
    content_md = CASE WHEN sqlc.arg(set_content_md)::boolean
                      THEN sqlc.arg(content_md)::text ELSE content_md END,
    word_count = CASE WHEN sqlc.arg(set_content_md)::boolean
                      THEN sqlc.arg(word_count)::int ELSE word_count END,
    cover_url = CASE WHEN sqlc.arg(set_cover_url)::boolean
                     THEN sqlc.narg(cover_url)::text ELSE cover_url END,
    visibility = CASE WHEN sqlc.arg(set_visibility)::boolean
                      THEN sqlc.arg(visibility)::varchar ELSE visibility END,
    meta = CASE WHEN sqlc.arg(set_meta)::boolean
                THEN sqlc.arg(meta)::jsonb ELSE meta END,
    happened_at = CASE WHEN sqlc.arg(set_happened_at)::boolean
                       THEN sqlc.narg(happened_at)::timestamptz ELSE happened_at END,
    -- Clearing this one is what makes an entry uncategorised again: set_category_id
    -- true with a NULL value. Under COALESCE that would be indistinguishable from
    -- leaving the category as it is, which is the whole reason for the flags.
    category_id = CASE WHEN sqlc.arg(set_category_id)::boolean
                       THEN sqlc.narg(category_id)::bigint ELSE category_id END
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision)
  AND deleted_at IS NULL
RETURNING
    id,
    revision,
    author_id,
    category_id,
    type,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: SoftDeleteEntry :one
-- Mark one entry deleted, returning its id so the caller can tell a hit from a
-- miss.
--
-- deleted_at IS NULL in the WHERE makes this report no row on a second call
-- rather than moving the timestamp. The first delete is the one that happened,
-- and a repeat must not rewrite when.
--
-- The row stays. Content is not regenerable, and every read filters on
-- deleted_at, so the row is unreachable without being gone. Note that the slug
-- is released: uk_entries_slug is partial on deleted_at IS NULL, so a later
-- entry may take it.
UPDATE entries
SET deleted_at = sqlc.arg(deleted_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id;

-- name: PublishEntry :one
-- Publish one entry, stamping published_at only if it has never been set.
--
-- Both changes are in one statement because entries_published_at_check refuses a
-- row with status 'published' and a NULL published_at. Two statements would leave
-- the row invalid between them, and the constraint would reject the first.
--
-- COALESCE is the whole rule: an entry published, withdrawn and published again
-- keeps its original date, so fixing a typo years later does not move a
-- three-year-old entry to the top of the feed.
UPDATE entries
SET
    revision = revision + 1,
    status = 'published',
    published_at = COALESCE(published_at, sqlc.arg(published_at))
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision)
  AND deleted_at IS NULL
RETURNING
    id,
    revision,
    author_id,
    category_id,
    type,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: UnpublishEntry :one
-- Return one entry to draft.
--
-- published_at is deliberately untouched: it records the first publication, which
-- is a fact that withdrawing does not undo. Clearing it would make a
-- re-publication look like a first one and move the entry to the top of the feed.
UPDATE entries
SET
    revision = revision + 1,
    status = 'draft'
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision)
  AND deleted_at IS NULL
RETURNING
    id,
    revision,
    author_id,
    category_id,
    type,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: ArchiveEntry :one
-- Retire one entry: off the site, but not deleted.
--
-- Distinct from both of the above. A draft is unfinished and an archived entry is
-- finished and withdrawn, and the difference matters in the admin list: drafts are
-- a work queue, archived entries are not. Distinct from a soft delete too, since
-- this one is still listed and still editable.
--
-- published_at survives here for the same reason it survives unpublishing.
UPDATE entries
SET
    revision = revision + 1,
    status = 'archived'
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision)
  AND deleted_at IS NULL
RETURNING
    id,
    revision,
    author_id,
    category_id,
    type,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: EntrySlugExists :one
-- Whether a live entry already holds this slug.
--
-- Used to answer a conflict before attempting the insert, so the caller gets a
-- clear 409 rather than a constraint violation. It does not replace the unique
-- index: two concurrent creates can both see false here, and the index is what
-- settles it. The repository translates that violation to the same conflict.
--
-- Matches uk_entries_slug: deleted rows do not hold their slug.
SELECT EXISTS (
    SELECT 1
    FROM entries
    WHERE slug = $1
      AND deleted_at IS NULL
);

-- name: EntrySlugExistsExcluding :one
-- Whether a live entry other than this one holds the slug.
--
-- The exclusion is what makes a no-op slug change work: submitting an entry's own
-- slug back must not be refused as a conflict with itself.
SELECT EXISTS (
    SELECT 1
    FROM entries
    WHERE slug = sqlc.arg(slug)
      AND id <> sqlc.arg(excluded_id)
      AND deleted_at IS NULL
);

-- The admin reads.
--
-- Separate statements from the public ones above, not the same queries with the
-- filter parameterised. These see every status and every visibility, and the only
-- thing keeping a draft off the front page is that the public queries cannot
-- express this. A shared query with a boolean would put that guarantee in the
-- hands of whoever passes the argument.
--
-- Soft deleted rows are excluded here too. There is no endpoint that shows them
-- and no restore, so a deleted entry is out of reach from every route.

-- name: ListAdminEntries :many
-- The admin list: every status, ordered by last edit.
--
-- updated_at DESC, not happened_at: this list is a work queue, so what was
-- touched last belongs at the top. The public list answers a different question
-- and sorts differently. Matches idx_entries_admin.
--
-- The status filter is a nullable argument: NULL means every status. That is not
-- the same hazard as parameterising the visibility filter, because no draft is
-- being kept from anyone here. Every row this statement can return is already
-- visible to the caller.
--
-- No content_md, for the same reason the public list omits it.
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.type,
    e.title,
    e.slug,
    e.summary,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c ON c.id = e.category_id
WHERE e.deleted_at IS NULL
  AND (sqlc.narg(status)::varchar IS NULL OR e.status = sqlc.narg(status)::varchar)
  AND (
    sqlc.narg(search)::text IS NULL
    OR e.title ILIKE '%' || sqlc.narg(search)::text || '%'
    OR e.slug ILIKE '%' || sqlc.narg(search)::text || '%'
    OR COALESCE(e.summary, '') ILIKE '%' || sqlc.narg(search)::text || '%'
  )
ORDER BY e.updated_at DESC, e.id DESC
LIMIT sqlc.arg(limit_) OFFSET sqlc.arg(offset_);

-- name: CountAdminEntries :one
-- The total for the admin list, under the same filter.
--
-- The search predicate is duplicated from ListAdminEntries rather than shared,
-- because sqlc generates from literal SQL and has no include mechanism. The two
-- must stay identical: a total computed under a different filter than the page
-- would report a pagination control the page cannot honour.
SELECT count(*)
FROM entries
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)::varchar)
  AND (
    sqlc.narg(search)::text IS NULL
    OR title ILIKE '%' || sqlc.narg(search)::text || '%'
    OR slug ILIKE '%' || sqlc.narg(search)::text || '%'
    OR COALESCE(summary, '') ILIKE '%' || sqlc.narg(search)::text || '%'
  );

-- name: GetAdminEntryByID :one
-- The admin detail read, addressed by id rather than slug.
--
-- By id because a draft is edited before its slug is settled, and because a slug
-- may change during editing while the thing being edited does not.
SELECT
    e.id,
    e.revision,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.type,
    e.title,
    e.slug,
    e.summary,
    e.content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c ON c.id = e.category_id
WHERE e.id = sqlc.arg(id)
  AND e.deleted_at IS NULL;
