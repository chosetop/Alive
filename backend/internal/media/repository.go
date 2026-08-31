package media

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

type Repository struct {
	q  *sqlcgen.Queries
	db *postgres.Pool
}

func NewRepository(pool *postgres.Pool) *Repository {
	return &Repository{q: sqlcgen.New(pool), db: pool}
}
func (r *Repository) Create(ctx context.Context, m Media) (Media, error) {
	row, err := r.q.CreateMedia(ctx, sqlcgen.CreateMediaParams{AuthorID: m.AuthorID, ObjectKey: m.ObjectKey, Url: m.URL, MimeType: m.MimeType, ByteSize: m.ByteSize})
	if err != nil {
		return Media{}, err
	}
	return Media{ID: row.ID, AuthorID: row.AuthorID, ObjectKey: row.ObjectKey, URL: row.Url, MimeType: row.MimeType, ByteSize: row.ByteSize, CreatedAt: row.CreatedAt}, nil
}
func (r *Repository) Get(ctx context.Context, id int64) (Media, error) {
	row, err := r.q.GetMediaByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Media{}, errors.New("media: not found")
	}
	if err != nil {
		return Media{}, err
	}
	return Media{ID: row.ID, AuthorID: row.AuthorID, ObjectKey: row.ObjectKey, URL: row.Url, MimeType: row.MimeType, ByteSize: row.ByteSize, CreatedAt: row.CreatedAt}, nil
}

func (r *Repository) ListForEntry(ctx context.Context, entryID int64) ([]Media, error) {
	rows, err := r.q.ListMediaForEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	out := make([]Media, 0, len(rows))
	for _, row := range rows {
		out = append(out, Media{ID: row.ID, AuthorID: row.AuthorID, ObjectKey: row.ObjectKey, URL: row.Url, MimeType: row.MimeType, ByteSize: row.ByteSize, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (r *Repository) EntryOwnedByAuthor(ctx context.Context, entryID, authorID int64) (bool, error) {
	return r.q.EntryOwnedByAuthor(ctx, sqlcgen.EntryOwnedByAuthorParams{ID: entryID, AuthorID: authorID})
}

func (r *Repository) SetPrimaryVideo(ctx context.Context, entryID, authorID, expectedRevision, mediaID int64) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var mime string
	if err = tx.QueryRow(ctx, `SELECT mime_type FROM media WHERE id=$1 AND author_id=$2`, mediaID, authorID).Scan(&mime); err != nil {
		return 0, err
	}
	if mime != "video/mp4" {
		return 0, errors.New("media: primary video must be mp4")
	}
	var revision int64
	if err = tx.QueryRow(ctx, `UPDATE entries SET revision=revision+1, updated_at=now() WHERE id=$1 AND author_id=$2 AND revision=$3 AND deleted_at IS NULL RETURNING revision`, entryID, authorID, expectedRevision).Scan(&revision); err != nil {
		return 0, errors.New("media: entry revision conflict")
	}
	if _, err = tx.Exec(ctx, `DELETE FROM entry_media WHERE entry_id=$1 AND role='primary_video'`, entryID); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO entry_media(entry_id,media_id,role) VALUES($1,$2,'primary_video') ON CONFLICT (entry_id,media_id) DO UPDATE SET role='primary_video'`, entryID, mediaID); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return revision, nil
}
