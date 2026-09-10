package music

import (
	"context"
	"slices"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/dbtest"
)

func TestCatalogTransactionsAndOwnership(t *testing.T) {
	p := dbtest.Pool(t)
	ctx := context.Background()
	r := NewRepository(p)
	var owner, other int64
	for i, out := range []*int64{&owner, &other} {
		if err := p.QueryRow(ctx, `INSERT INTO users(username,password_hash) VALUES($1,'test') RETURNING id`, dbtest.Username(t, []string{"music", "foreign"}[i])).Scan(out); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, table := range []string{"music_playlist_tracks", "music_playlists", "music_tracks", "music_assets"} {
			_, _ = p.Exec(ctx, `DELETE FROM `+table+` WHERE author_id=ANY($1::bigint[])`, []int64{owner, other})
		}
		_, _ = p.Exec(ctx, `DELETE FROM users WHERE id=ANY($1::bigint[])`, []int64{owner, other})
	})
	asset, err := r.SaveAsset(ctx, Asset{AuthorID: owner, ObjectKey: dbtest.Username(t, "object"), URL: "https://cdn.test/audio.mp3", Kind: "audio", MIMEType: "audio/mpeg", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	track, err := r.SaveTrack(ctx, owner, 0, TrackInput{Title: "One", AudioAssetID: &asset.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.SaveTrack(ctx, other, 0, TrackInput{Title: "stolen", AudioAssetID: &asset.ID}); err != ErrInvalid {
		t.Fatalf("ownership: %v", err)
	}
	hidden, err := r.SavePlaylist(ctx, owner, 0, PlaylistInput{Name: "Private", TrackIDs: []int64{track.ID}})
	if err != nil {
		t.Fatal(err)
	}
	// The public catalog is global: unrelated fixtures may already be public.
	// Assert visibility of this test's resources without requiring an empty database.
	pub, err := r.Catalog(ctx, 0, true)
	if err != nil || slices.ContainsFunc(pub.Tracks, func(item Track) bool { return item.ID == track.ID }) || slices.ContainsFunc(pub.Playlists, func(item Playlist) bool { return item.ID == hidden.ID }) {
		t.Fatalf("private leaked %+v %v", pub, err)
	}
	list, err := r.SavePlaylist(ctx, owner, 0, PlaylistInput{Name: "Public", IsPublic: true, IsDefault: true, TrackIDs: []int64{track.ID}})
	if err != nil {
		t.Fatal(err)
	}
	pub, err = r.Catalog(ctx, 0, true)
	if err != nil || !slices.ContainsFunc(pub.Tracks, func(item Track) bool { return item.ID == track.ID }) || !slices.ContainsFunc(pub.Playlists, func(item Playlist) bool { return item.ID == list.ID }) || slices.ContainsFunc(pub.Playlists, func(item Playlist) bool { return item.ID == hidden.ID }) {
		t.Fatalf("public %+v %v", pub, err)
	}
	// A foreign track error must roll back the attempted default reassignment.
	if _, err = r.SavePlaylist(ctx, owner, hidden.ID, PlaylistInput{Name: "Bad", Revision: hidden.Revision, IsPublic: true, IsDefault: true, TrackIDs: []int64{9223372036854775807}}); err != ErrInvalid {
		t.Fatal(err)
	}
	catalog, err := r.Catalog(ctx, owner, false)
	if err != nil || !catalog.Playlists[0].IsDefault || catalog.Playlists[0].ID != list.ID {
		t.Fatalf("default lost %+v %v", catalog, err)
	}
	if _, err = r.SavePlaylist(ctx, owner, list.ID, PlaylistInput{Name: "Stale", Revision: 999, IsPublic: true}); err != ErrConflict {
		t.Fatal(err)
	}
	if err = r.Delete(ctx, other, track.ID, track.Revision, false); err != ErrNotFound {
		t.Fatal(err)
	}
	if err = r.Delete(ctx, owner, track.ID, 99, false); err != ErrConflict {
		t.Fatal(err)
	}
	if err = r.Delete(ctx, owner, track.ID, track.Revision, false); err != nil {
		t.Fatal(err)
	}
	catalog, err = r.Catalog(ctx, owner, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range catalog.Playlists {
		if len(list.TrackIDs) != 0 || list.Revision != 2 {
			t.Fatalf("dangling or stale playlist %+v", list)
		}
	}
	pub, err = r.Catalog(ctx, 0, true)
	if err != nil || slices.ContainsFunc(pub.Playlists, func(item Playlist) bool { return item.ID == list.ID || item.ID == hidden.ID }) || slices.ContainsFunc(pub.Tracks, func(item Track) bool { return item.ID == track.ID }) {
		t.Fatalf("empty public %+v %v", pub, err)
	}
}
