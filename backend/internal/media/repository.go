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
	row, err := r.q.CreateMedia(ctx, sqlcgen.CreateMediaParams{AuthorID: m.AuthorID, ObjectKey: m.ObjectKey, MimeType: m.MimeType, ByteSize: m.ByteSize})
	if err != nil {
		return Media{}, err
	}
	return Media{ID: row.ID, AuthorID: row.AuthorID, ObjectKey: row.ObjectKey, MimeType: row.MimeType, ByteSize: row.ByteSize, CreatedAt: row.CreatedAt}, nil
}
func (r *Repository) Get(ctx context.Context, id int64) (Media, error) {
	row, err := r.q.GetMediaByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Media{}, errors.New("media: not found")
	}
	if err != nil {
		return Media{}, err
	}
	return Media{ID: row.ID, AuthorID: row.AuthorID, ObjectKey: row.ObjectKey, MimeType: row.MimeType, ByteSize: row.ByteSize, CreatedAt: row.CreatedAt}, nil
}
