package entry

import (
	"context"
	"fmt"
)

// ReplaceTags atomically replaces an entry's tag ids and advances its revision.
// It is intentionally repository-level until the editor write path is wired to
// the same CAS contract as the other entry mutations.
func (r *Repository) ReplaceTags(ctx context.Context, id, expectedRevision int64, tagIDs []int64) (int64, error) {
	if len(tagIDs) > 20 {
		return 0, fmt.Errorf("entry: at most 20 tags")
	}
	seen := make(map[int64]struct{}, len(tagIDs))
	for _, tagID := range tagIDs {
		if tagID <= 0 {
			return 0, fmt.Errorf("entry: invalid tag id")
		}
		if _, ok := seen[tagID]; ok {
			return 0, fmt.Errorf("entry: duplicate tag id")
		}
		seen[tagID] = struct{}{}
	}
	if r.db == nil {
		return 0, fmt.Errorf("entry: tag replacement requires database")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var revision int64
	if err := tx.QueryRow(ctx, `UPDATE entries SET revision = revision + 1 WHERE id = $1 AND revision = $2 AND deleted_at IS NULL RETURNING revision`, id, expectedRevision).Scan(&revision); err != nil {
		return 0, ErrVersionConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM entry_tags WHERE entry_id = $1`, id); err != nil {
		return 0, err
	}
	for _, tagID := range tagIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO entry_tags (entry_id, tag_id) VALUES ($1, $2)`, id, tagID); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return revision, nil
}
