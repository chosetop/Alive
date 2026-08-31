package contentworld_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/contentworld/contentworldtest"
)

func TestServiceListMethodsReturnFixedWorldOrder(t *testing.T) {
	store := contentworldtest.NewStore()
	store.Settings[contentworld.Saying] = contentworld.Setting{
		World:       contentworld.Saying,
		Status:      contentworld.Open,
		NavLabel:    "片语",
		SortOrder:   20,
		DefaultView: contentworld.ViewWall,
		Revision:    1,
		UpdatedAt:   store.Settings[contentworld.Saying].UpdatedAt,
	}
	store.ListOpenReturn = []contentworld.Setting{
		store.Settings[contentworld.Saying],
		store.Settings[contentworld.Journal],
	}
	store.ListAllReturn = []contentworld.Setting{
		store.Settings[contentworld.Video],
		store.Settings[contentworld.Journal],
		store.Settings[contentworld.Saying],
	}
	service := contentworld.NewService(store)

	openWorlds, err := service.ListOpen(context.Background())
	if err != nil {
		t.Fatalf("ListOpen() error = %v", err)
	}
	if got := []contentworld.Key{openWorlds[0].World, openWorlds[1].World}; got[0] != contentworld.Journal || got[1] != contentworld.Saying {
		t.Fatalf("ListOpen() order = %v, want [journal saying]", got)
	}

	adminWorlds, err := service.ListAdmin(context.Background())
	if err != nil {
		t.Fatalf("ListAdmin() error = %v", err)
	}
	if got := []contentworld.Key{adminWorlds[0].World, adminWorlds[1].World, adminWorlds[2].World}; got[0] != contentworld.Journal || got[1] != contentworld.Saying || got[2] != contentworld.Video {
		t.Fatalf("ListAdmin() order = %v, want [journal saying video]", got)
	}
}

func TestServiceUpdateAcceptsValidLifecycleChange(t *testing.T) {
	store := contentworldtest.NewStore()
	service := contentworld.NewService(store)

	status := contentworld.Open
	label := "片语"
	view := contentworld.ViewWall
	updated, err := service.Update(context.Background(), contentworld.Saying, contentworld.UpdateInput{
		ExpectedRevision: 1,
		Status:           &status,
		NavLabel:         &label,
		DefaultView:      &view,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != contentworld.Open || updated.DefaultView != contentworld.ViewWall || updated.Revision != 2 {
		t.Fatalf("Update() = %+v, want open wall revision 2", updated)
	}
}

func TestServiceUpdateRejectsUnknownWorld(t *testing.T) {
	store := contentworldtest.NewStore()
	service := contentworld.NewService(store)

	status := contentworld.Open
	_, err := service.Update(context.Background(), contentworld.Key("book"), contentworld.UpdateInput{
		ExpectedRevision: 1,
		Status:           &status,
	})
	if !errors.Is(err, contentworld.ErrWorldNotFound) {
		t.Fatalf("Update() error = %v, want ErrWorldNotFound", err)
	}
	if store.UpdateCalls != 0 {
		t.Fatalf("store Update calls = %d, want 0", store.UpdateCalls)
	}
}

func TestServiceUpdateRejectsInvalidStatusBeforeStore(t *testing.T) {
	store := contentworldtest.NewStore()
	service := contentworld.NewService(store)
	status := contentworld.Status("published")

	_, err := service.Update(context.Background(), contentworld.Saying, contentworld.UpdateInput{
		ExpectedRevision: 1,
		Status:           &status,
	})
	if !errors.Is(err, contentworld.ErrInvalidStatus) {
		t.Fatalf("Update() error = %v, want ErrInvalidStatus", err)
	}
	if store.UpdateCalls != 0 {
		t.Fatalf("store Update calls = %d, want 0", store.UpdateCalls)
	}
}

func TestServiceUpdateRejectsInvalidLabelsBeforeStore(t *testing.T) {
	tests := []string{"", strings.Repeat("片", 65)}

	for _, label := range tests {
		t.Run(label, func(t *testing.T) {
			store := contentworldtest.NewStore()
			service := contentworld.NewService(store)

			_, err := service.Update(context.Background(), contentworld.Saying, contentworld.UpdateInput{
				ExpectedRevision: 1,
				NavLabel:         &label,
			})
			if !errors.Is(err, contentworld.ErrInvalidNavLabel) {
				t.Fatalf("Update() error = %v, want ErrInvalidNavLabel", err)
			}
			if store.UpdateCalls != 0 {
				t.Fatalf("store Update calls = %d, want 0", store.UpdateCalls)
			}
		})
	}
}

func TestServiceUpdateRejectsUnsupportedViewModeForWorld(t *testing.T) {
	store := contentworldtest.NewStore()
	service := contentworld.NewService(store)
	view := contentworld.ViewWall

	_, err := service.Update(context.Background(), contentworld.Journal, contentworld.UpdateInput{
		ExpectedRevision: 1,
		DefaultView:      &view,
	})
	if !errors.Is(err, contentworld.ErrInvalidViewMode) {
		t.Fatalf("Update() error = %v, want ErrInvalidViewMode", err)
	}
	if store.UpdateCalls != 0 {
		t.Fatalf("store Update calls = %d, want 0", store.UpdateCalls)
	}
}

func TestServiceAllowsPublicPublishDependsOnLifecycle(t *testing.T) {
	store := contentworldtest.NewStore()
	service := contentworld.NewService(store)

	store.Settings[contentworld.Saying] = contentworld.Setting{
		World:       contentworld.Saying,
		Status:      contentworld.Hidden,
		NavLabel:    "片语",
		SortOrder:   20,
		DefaultView: contentworld.ViewStream,
		Revision:    1,
		UpdatedAt:   store.Settings[contentworld.Saying].UpdatedAt,
	}

	tests := []struct {
		world contentworld.Key
		want  bool
	}{
		{world: contentworld.Journal, want: true},
		{world: contentworld.Saying, want: true},
		{world: contentworld.Video, want: false},
	}

	for _, tc := range tests {
		t.Run(string(tc.world), func(t *testing.T) {
			got, err := service.AllowsPublicPublish(context.Background(), tc.world)
			if err != nil {
				t.Fatalf("AllowsPublicPublish() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("AllowsPublicPublish() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestServiceAllowsPublicPublishRejectsUnknownWorld(t *testing.T) {
	service := contentworld.NewService(contentworldtest.NewStore())

	_, err := service.AllowsPublicPublish(context.Background(), contentworld.Key("book"))
	if !errors.Is(err, contentworld.ErrWorldNotFound) {
		t.Fatalf("AllowsPublicPublish() error = %v, want ErrWorldNotFound", err)
	}
}
