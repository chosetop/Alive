-- The public list orders by the date a reader sees, which is happened_at when the
-- entry records one and published_at otherwise.
--
-- Before this, the query ordered by happened_at DESC NULLS LAST while the
-- frontend labelled each entry with COALESCE(happened_at, published_at). Sorting
-- by one expression and grouping by another put an entry dated 2026 after the
-- 2024 block, so the year headings read 2026, 2025, 2024, 2026.
--
-- COALESCE is safe as a sort key here because entries_published_at_check
-- guarantees published_at IS NOT NULL for every published row, so the expression
-- is never NULL on the rows this index covers. That removes the NULLS LAST
-- special case as well.
--
-- An expression index, not a column index: the planner only uses an index for
-- ORDER BY when the index expression matches the sort expression textually, so
-- idx_entries_timeline on (happened_at DESC) cannot serve this ordering. Without
-- this index the list degrades to a sort of every published row.
--
-- The predicate mirrors the query's filter exactly (deleted_at, status,
-- visibility), which keeps the index to the rows the public feed reads.
CREATE INDEX idx_entries_public_timeline
    ON entries (COALESCE(happened_at, published_at) DESC, id DESC)
    WHERE deleted_at IS NULL
      AND status = 'published'
      AND visibility = 'public';
