package taxonomy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/dbtest"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

func newTestTagRepository(t *testing.T) (*taxonomy.TagRepository, *postgres.Pool, int64) {
	t.Helper()

	pool := dbtest.Pool(t)
	dbtest.CleanupUsers(t, pool)
	dbtest.CleanupTags(t, pool)

	owner, err := auth.NewRepository(pool).CreateUser(context.Background(), auth.CreateUserParams{
		Username:     dbtest.Username(t, "tagowner"),
		PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}

	return taxonomy.NewTagRepository(pool), pool, owner.ID
}

func storedTag(t *testing.T, repo *taxonomy.TagRepository, name, slug string) taxonomy.Tag {
	t.Helper()

	created, err := repo.Create(context.Background(), taxonomy.CreateTagParams{Name: name, Slug: slug})
	if err != nil {
		t.Fatalf("create tag %q: %v", name, err)
	}
	return created
}

func TestRepositoryCreateStoresEveryField(t *testing.T) {
	repo, _, _ := newTestTagRepository(t)

	created, err := repo.Create(context.Background(), taxonomy.CreateTagParams{
		Name: "Travel",
		Slug: dbtest.Slug(t, "travel"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created tag carries no id")
	}
	if created.Name != "Travel" {
		t.Fatalf("name = %q, want Travel", created.Name)
	}
	if created.Slug == "" {
		t.Fatal("slug is empty")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("timestamps were not filled")
	}
}

func TestRepositoryRejectsDuplicateNameCaseInsensitive(t *testing.T) {
	repo, _, _ := newTestTagRepository(t)
	ctx := context.Background()

	if _, err := repo.Create(ctx, taxonomy.CreateTagParams{
		Name: "Travel",
		Slug: dbtest.Slug(t, "travel"),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := repo.Create(ctx, taxonomy.CreateTagParams{
		Name: "travel",
		Slug: dbtest.Slug(t, "journey"),
	})
	if !errors.Is(err, taxonomy.ErrTagNameTaken) {
		t.Fatalf("Create = %v, want ErrTagNameTaken", err)
	}
}

func TestRepositoryRejectsDuplicateSlug(t *testing.T) {
	repo, _, _ := newTestTagRepository(t)
	ctx := context.Background()
	slug := dbtest.Slug(t, "travel")

	if _, err := repo.Create(ctx, taxonomy.CreateTagParams{
		Name: "Travel",
		Slug: slug,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := repo.Create(ctx, taxonomy.CreateTagParams{
		Name: "Journey",
		Slug: slug,
	})
	if !errors.Is(err, taxonomy.ErrTagSlugTaken) {
		t.Fatalf("Create = %v, want ErrTagSlugTaken", err)
	}
}

func TestRepositoryListOrdersPrefixMatchesFirst(t *testing.T) {
	repo, _, _ := newTestTagRepository(t)
	ctx := context.Background()

	yodel := storedTag(t, repo, "Yodel", dbtest.Slug(t, "yodel"))
	_ = storedTag(t, repo, "Kiyoto", dbtest.Slug(t, "kiyoto"))
	storedTag(t, repo, "Aroma", dbtest.Slug(t, "aroma"))

	tags, err := repo.List(ctx, "yo", 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) < 2 {
		t.Fatalf("List returned %d tags, want at least 2", len(tags))
	}
	if tags[0].ID != yodel.ID {
		t.Fatalf("first tag id = %d, want %d", tags[0].ID, yodel.ID)
	}
}

func TestRepositoryUsageCountsExcludeSoftDeletedEntries(t *testing.T) {
	repo, pool, ownerID := newTestTagRepository(t)
	ctx := context.Background()

	tag := storedTag(t, repo, "Travel", dbtest.Slug(t, "travel"))
	entries := entry.NewRepository(pool)

	live, err := entries.Create(ctx, entry.CreateParams{
		AuthorID:   ownerID,
		World:      contentworld.Journal,
		Title:      "京都",
		Slug:       dbtest.Slug(t, "live"),
		ContentMD:  "hello",
		Status:     entry.StatusDraft,
		Visibility: entry.VisibilityPublic,
		Meta:       entry.DefaultMeta(),
	})
	if err != nil {
		t.Fatalf("create live entry: %v", err)
	}
	deleted, err := entries.Create(ctx, entry.CreateParams{
		AuthorID:   ownerID,
		World:      contentworld.Journal,
		Title:      "京都",
		Slug:       dbtest.Slug(t, "deleted"),
		ContentMD:  "hello",
		Status:     entry.StatusDraft,
		Visibility: entry.VisibilityPublic,
		Meta:       entry.DefaultMeta(),
	})
	if err != nil {
		t.Fatalf("create deleted entry: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO entry_tags (entry_id, tag_id) VALUES ($1, $2), ($3, $2)", live.ID, tag.ID, deleted.ID); err != nil {
		t.Fatalf("seed entry_tags: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE entries SET deleted_at = $1 WHERE id = $2", time.Now(), deleted.ID); err != nil {
		t.Fatalf("soft delete entry: %v", err)
	}

	tags, err := repo.List(ctx, "", 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, candidate := range tags {
		if candidate.ID == tag.ID {
			if candidate.UsageCount != 1 {
				t.Fatalf("usage_count = %d, want 1", candidate.UsageCount)
			}
			return
		}
	}
	t.Fatalf("tag %d not found in list", tag.ID)
}

func TestRepositoryDeleteDistinguishesUnusedAndUsedTags(t *testing.T) {
	repo, pool, ownerID := newTestTagRepository(t)
	ctx := context.Background()

	used := storedTag(t, repo, "Used", dbtest.Slug(t, "used"))
	unused := storedTag(t, repo, "Unused", dbtest.Slug(t, "unused"))

	ownedEntry, err := entry.NewRepository(pool).Create(ctx, entry.CreateParams{
		AuthorID:   ownerID,
		World:      contentworld.Journal,
		Title:      "京都",
		Slug:       dbtest.Slug(t, "entry"),
		ContentMD:  "hello",
		Status:     entry.StatusDraft,
		Visibility: entry.VisibilityPublic,
		Meta:       entry.DefaultMeta(),
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO entry_tags (entry_id, tag_id) VALUES ($1, $2)", ownedEntry.ID, used.ID); err != nil {
		t.Fatalf("seed entry_tags: %v", err)
	}

	deleted, err := repo.Delete(ctx, unused.ID)
	if err != nil {
		t.Fatalf("Delete unused: %v", err)
	}
	if !deleted {
		t.Fatal("unused tag was not deleted")
	}

	if deleted, err := repo.Delete(ctx, used.ID); err == nil {
		t.Fatalf("Delete used returned deleted=%v, want tag-in-use error", deleted)
	} else if !errors.Is(err, taxonomy.ErrTagInUse) {
		t.Fatalf("Delete used = %v, want ErrTagInUse", err)
	}
}
