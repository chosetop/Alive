package music

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/p30huiwei/alive/backend/internal/postgres"
)

type Repository struct{ pool *postgres.Pool }

func NewRepository(pool *postgres.Pool) *Repository { return &Repository{pool: pool} }
func dbErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var e *pgconn.PgError
	if errors.As(err, &e) {
		switch e.Code {
		case "23503", "23514":
			return ErrInvalid
		case "23505":
			return ErrConflict
		}
	}
	return err
}
func (r *Repository) SaveAsset(ctx context.Context, a Asset) (Asset, error) {
	// Registration is idempotent so retrying after a lost response cannot strand an upload.
	err := r.pool.QueryRow(ctx, `INSERT INTO music_assets(author_id,object_key,url,kind,mime_type,size_bytes) VALUES($1,$2,$3,$4,$5,$6)
 ON CONFLICT(object_key) DO UPDATE SET object_key=EXCLUDED.object_key WHERE music_assets.author_id=EXCLUDED.author_id
 RETURNING id`, a.AuthorID, a.ObjectKey, a.URL, a.Kind, a.MIMEType, a.SizeBytes).Scan(&a.ID)
	return a, dbErr(err)
}
func (r *Repository) Catalog(ctx context.Context, author int64, public bool) (Catalog, error) {
	result := Catalog{Tracks: []Track{}, Playlists: []Playlist{}}
	// Both lists must describe the same snapshot, including during a publish/delete.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT p.id,p.name,COALESCE(a.url,''),p.is_public,p.is_default,p.revision,
 ARRAY(SELECT pt.track_id FROM music_playlist_tracks pt WHERE pt.playlist_id=p.id ORDER BY pt.position)
 FROM music_playlists p LEFT JOIN music_assets a ON a.id=p.cover_asset_id
 WHERE ($1 AND p.is_public AND EXISTS(SELECT 1 FROM music_playlist_tracks pt WHERE pt.playlist_id=p.id)) OR (NOT $1 AND p.author_id=$2)
 ORDER BY p.is_default DESC,p.id`, public, author)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var p Playlist
		if err = rows.Scan(&p.ID, &p.Name, &p.CoverURL, &p.IsPublic, &p.IsDefault, &p.Revision, &p.TrackIDs); err != nil {
			rows.Close()
			return result, err
		}
		result.Playlists = append(result.Playlists, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = tx.Query(ctx, `SELECT t.id,t.title,t.artist,a.url,COALESCE(c.url,''),t.duration,t.revision FROM music_tracks t
 JOIN music_assets a ON a.id=t.audio_asset_id LEFT JOIN music_assets c ON c.id=t.cover_asset_id
 WHERE (NOT $1 AND t.author_id=$2) OR ($1 AND EXISTS(SELECT 1 FROM music_playlist_tracks pt JOIN music_playlists p ON p.id=pt.playlist_id WHERE pt.track_id=t.id AND p.is_public)) ORDER BY t.id DESC`, public, author)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var t Track
		if err = rows.Scan(&t.ID, &t.Title, &t.Artist, &t.AudioURL, &t.CoverURL, &t.Duration, &t.Revision); err != nil {
			rows.Close()
			return result, err
		}
		result.Tracks = append(result.Tracks, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

// Music mutations share one transaction lock: default reassignment, track removal
// and playlist replacement must not interleave into an inconsistent public catalog.
func (r *Repository) begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(716230901)`)
	if err != nil {
		tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}
func checkAsset(ctx context.Context, tx pgx.Tx, id *int64, author int64, kind string) error {
	if id == nil {
		return nil
	}
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM music_assets WHERE id=$1 AND author_id=$2 AND kind=$3)`, *id, author, kind).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalid
	}
	return nil
}
func (r *Repository) SaveTrack(ctx context.Context, author, id int64, in TrackInput) (Track, error) {
	var out Track
	tx, err := r.begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = checkAsset(ctx, tx, in.AudioAssetID, author, "audio"); err != nil {
		return out, err
	}
	if err = checkAsset(ctx, tx, in.CoverAssetID.Value, author, "image"); err != nil {
		return out, err
	}
	if id == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO music_tracks(author_id,title,artist,audio_asset_id,cover_asset_id,duration) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, author, in.Title, in.Artist, in.AudioAssetID, in.CoverAssetID.Value, in.Duration).Scan(&id)
	} else {
		var rev int64
		err = tx.QueryRow(ctx, `SELECT revision FROM music_tracks WHERE id=$1 AND author_id=$2 FOR UPDATE`, id, author).Scan(&rev)
		if err != nil {
			return out, dbErr(err)
		}
		if rev != in.Revision {
			return out, ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE music_tracks SET title=$3,artist=$4,audio_asset_id=COALESCE($5,audio_asset_id),cover_asset_id=CASE WHEN $6 THEN $7 ELSE cover_asset_id END,duration=$8,revision=revision+1 WHERE id=$1 AND author_id=$2`, id, author, in.Title, in.Artist, in.AudioAssetID, in.CoverAssetID.Set, in.CoverAssetID.Value, in.Duration)
	}
	if err != nil {
		return out, dbErr(err)
	}
	err = tx.QueryRow(ctx, `SELECT t.id,t.title,t.artist,a.url,COALESCE(c.url,''),t.duration,t.revision FROM music_tracks t JOIN music_assets a ON a.id=t.audio_asset_id LEFT JOIN music_assets c ON c.id=t.cover_asset_id WHERE t.id=$1`, id).Scan(&out.ID, &out.Title, &out.Artist, &out.AudioURL, &out.CoverURL, &out.Duration, &out.Revision)
	if err != nil {
		return out, dbErr(err)
	}
	return out, tx.Commit(ctx)
}
func (r *Repository) SavePlaylist(ctx context.Context, author, id int64, in PlaylistInput) (Playlist, error) {
	var out Playlist
	tx, err := r.begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = checkAsset(ctx, tx, in.CoverAssetID.Value, author, "image"); err != nil {
		return out, err
	}
	var count int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM music_tracks WHERE author_id=$1 AND id=ANY($2::bigint[])`, author, in.TrackIDs).Scan(&count)
	if err != nil {
		return out, err
	}
	if count != len(in.TrackIDs) {
		return out, ErrInvalid
	}
	if id != 0 {
		var rev int64
		err = tx.QueryRow(ctx, `SELECT revision FROM music_playlists WHERE id=$1 AND author_id=$2 FOR UPDATE`, id, author).Scan(&rev)
		if err != nil {
			return out, dbErr(err)
		}
		if rev != in.Revision {
			return out, ErrConflict
		}
	}
	if in.IsDefault {
		_, err = tx.Exec(ctx, `UPDATE music_playlists SET is_default=false,revision=revision+1 WHERE is_default AND id<>$1`, id)
		if err != nil {
			return out, err
		}
	}
	if id == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO music_playlists(author_id,name,cover_asset_id,is_public,is_default) VALUES($1,$2,$3,$4,$5) RETURNING id`, author, in.Name, in.CoverAssetID.Value, in.IsPublic, in.IsDefault).Scan(&id)
	} else {
		_, err = tx.Exec(ctx, `UPDATE music_playlists SET name=$3,cover_asset_id=CASE WHEN $4 THEN $5 ELSE cover_asset_id END,is_public=$6,is_default=$7,revision=revision+1 WHERE id=$1 AND author_id=$2`, id, author, in.Name, in.CoverAssetID.Set, in.CoverAssetID.Value, in.IsPublic, in.IsDefault)
	}
	if err != nil {
		return out, dbErr(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM music_playlist_tracks WHERE playlist_id=$1`, id); err != nil {
		return out, err
	}
	for i, track := range in.TrackIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO music_playlist_tracks(playlist_id,track_id,author_id,position) VALUES($1,$2,$3,$4)`, id, track, author, i); err != nil {
			return out, dbErr(err)
		}
	}
	err = tx.QueryRow(ctx, `SELECT p.id,p.name,COALESCE(a.url,''),p.is_public,p.is_default,p.revision FROM music_playlists p LEFT JOIN music_assets a ON a.id=p.cover_asset_id WHERE p.id=$1`, id).Scan(&out.ID, &out.Name, &out.CoverURL, &out.IsPublic, &out.IsDefault, &out.Revision)
	out.TrackIDs = append([]int64{}, in.TrackIDs...)
	if err != nil {
		return out, dbErr(err)
	}
	return out, tx.Commit(ctx)
}
func (r *Repository) Delete(ctx context.Context, author, id, revision int64, playlist bool) error {
	tx, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	table := "music_tracks"
	if playlist {
		table = "music_playlists"
	}
	var rev int64
	err = tx.QueryRow(ctx, `SELECT revision FROM `+table+` WHERE id=$1 AND author_id=$2 FOR UPDATE`, id, author).Scan(&rev)
	if err != nil {
		return dbErr(err)
	}
	if rev != revision {
		return ErrConflict
	}
	if !playlist {
		_, err = tx.Exec(ctx, `UPDATE music_playlists SET revision=revision+1 WHERE id IN(SELECT playlist_id FROM music_playlist_tracks WHERE track_id=$1)`, id)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM `+table+` WHERE id=$1 AND author_id=$2`, id, author)
	if err != nil {
		return dbErr(err)
	}
	return tx.Commit(ctx)
}
