package musichttp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/httpx"
	"github.com/p30huiwei/alive/backend/internal/music"
)

type fakeStore struct {
	music.Store
	cover  music.OptionalID
	public bool
	author int64
	err    error
}

func (s *fakeStore) Catalog(_ context.Context, a int64, p bool) (music.Catalog, error) {
	s.author = a
	s.public = p
	return music.Catalog{Tracks: []music.Track{}, Playlists: []music.Playlist{}}, s.err
}
func (s *fakeStore) SaveTrack(_ context.Context, a, id int64, in music.TrackInput) (music.Track, error) {
	s.cover = in.CoverAssetID
	return music.Track{ID: id}, s.err
}
func testRouter(store *fakeStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(music.NewService(store, nil, "", time.Minute), func(c *gin.Context) (int64, bool) { return 7, c.GetHeader("X-Test-Owner") == "yes" })
	h.Register(r.Group("/api/v1"), func(c *gin.Context) {
		if c.GetHeader("X-Test-Owner") != "yes" {
			httpx.Error(c, apperr.Unauthorized("authentication required"))
		}
	})
	return r
}
func TestAuthAndPublicCatalog(t *testing.T) {
	s := &fakeStore{}
	r := testRouter(s)
	for _, path := range []string{"/api/v1/admin/music", "/api/v1/music"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		want := 200
		if strings.Contains(path, "/admin/") {
			want = 401
		}
		if w.Code != want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if !s.public || s.author != 0 {
		t.Fatal("wrong public catalog scope")
	}
	req := httptest.NewRequest("POST", "/api/v1/admin/music/uploads/presign", strings.NewReader(`{"filename":"a.mp3","mime_type":"audio/mpeg","size_bytes":10}`))
	req.Header.Set("X-Test-Owner", "yes")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestCoverPatchAndErrorMapping(t *testing.T) {
	s := &fakeStore{}
	r := testRouter(s)
	for _, tc := range []struct {
		body  string
		set   bool
		value int64
		code  int
	}{
		{`{"title":"Song","revision":1}`, false, 0, 200},
		{`{"title":"Song","revision":1,"cover_asset_id":null}`, true, 0, 200},
		{`{"title":"Song","revision":1,"cover_asset_id":22}`, true, 22, 200},
		{`{"title":"Song","revision":1,"cover_asset_id":"22"}`, false, 0, 400},
		{`{"title":"Song"}`, false, 0, 400},
	} {
		req := httptest.NewRequest("PUT", "/api/v1/admin/music/tracks/1", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Owner", "yes")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.code == 200 {
			if s.cover.Set != tc.set {
				t.Fatal("cover omitted/null conflated")
			}
			if tc.value > 0 && (s.cover.Value == nil || *s.cover.Value != tc.value) {
				t.Fatal("wrong cover")
			}
		}
	}
	for _, tc := range []struct {
		err  error
		code int
	}{{music.ErrConflict, 409}, {music.ErrNotFound, 404}} {
		s.err = tc.err
		req := httptest.NewRequest("GET", "/api/v1/music", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatal(w.Code)
		}
	}
}
