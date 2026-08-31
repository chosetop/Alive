package media

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

type Repository struct{ q *sqlcgen.Queries }

func NewRepository(pool *postgres.Pool) *Repository { return &Repository{q: sqlcgen.New(pool)} }
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
