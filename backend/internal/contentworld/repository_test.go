package contentworld

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/p30huiwei/alive/backend/internal/dbtest"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

func newTestRepository(t *testing.T) (*Repository, pgx.Tx) {
	t.Helper()

	pool := dbtest.Pool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
	})

	return &Repository{q: sqlcgen.New(tx)}, tx
}

func TestRepositoryListOpenWorldsReturnsOnlyOpenRowsInOrder(t *testing.T) {
	repo, tx := newTestRepository(t)
	ctx := context.Background()

	if _, err := tx.Exec(ctx, `UPDATE site_worlds SET status = 'open' WHERE world IN ('journal', 'saying')`); err != nil {
		t.Fatalf("open worlds: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE site_worlds SET status = 'hidden' WHERE world = 'video'`); err != nil {
		t.Fatalf("hide video: %v", err)
	}

	worlds, err := repo.ListOpen(ctx)
	if err != nil {
		t.Fatalf("ListOpen() error = %v", err)
	}
	if len(worlds) != 2 {
		t.Fatalf("ListOpen() len = %d, want 2", len(worlds))
	}
	if worlds[0].World != Journal || worlds[1].World != Saying {
		t.Fatalf("ListOpen() order = [%s %s], want [journal saying]", worlds[0].World, worlds[1].World)
	}
}

func TestRepositoryGetWorldMapsUnknownKeyToNotFound(t *testing.T) {
	repo, _ := newTestRepository(t)

	_, err := repo.Get(context.Background(), Key("book"))
	if !errors.Is(err, ErrWorldNotFound) {
		t.Fatalf("Get() error = %v, want ErrWorldNotFound", err)
	}
}

func TestRepositoryUpdateWorldReturnsVersionConflictOnStaleRevision(t *testing.T) {
	repo, _ := newTestRepository(t)
	status := Open

	_, err := repo.Update(context.Background(), Saying, UpdateInput{
		ExpectedRevision: 999,
		Status:           &status,
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Update() error = %v, want ErrVersionConflict", err)
	}
}

func TestRepositoryUpdateWorldPersistsLifecycleFields(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()
	status := Open
	label := "短句"
	view := ViewWall

	updated, err := repo.Update(ctx, Saying, UpdateInput{
		ExpectedRevision: 1,
		Status:           &status,
		NavLabel:         &label,
		DefaultView:      &view,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != Open || updated.NavLabel != "短句" || updated.DefaultView != ViewWall || updated.Revision != 2 {
		t.Fatalf("Update() = %+v, want open/短句/wall/revision 2", updated)
	}
}
