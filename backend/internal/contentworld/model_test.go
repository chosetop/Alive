package contentworld_test

import (
	"slices"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
)

func TestRegistryDefinesSupportedWorlds(t *testing.T) {
	want := []contentworld.Key{
		contentworld.Journal,
		contentworld.Saying,
		contentworld.Video,
	}

	if got := contentworld.Keys(); !slices.Equal(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
}

func TestLookupReturnsDefinitionForEachSupportedWorld(t *testing.T) {
	tests := []struct {
		name            string
		key             contentworld.Key
		label           string
		sortOrder       int
		views           []contentworld.ViewMode
		categoryEnabled bool
		mediaCapability contentworld.MediaCapability
	}{
		{
			name:            "journal",
			key:             contentworld.Journal,
			label:           "日志",
			sortOrder:       10,
			views:           []contentworld.ViewMode{contentworld.ViewNone},
			categoryEnabled: true,
			mediaCapability: contentworld.MediaMarkdownImages,
		},
		{
			name:            "saying",
			key:             contentworld.Saying,
			label:           "片语",
			sortOrder:       20,
			views:           []contentworld.ViewMode{contentworld.ViewStream, contentworld.ViewWall, contentworld.ViewFocus},
			categoryEnabled: true,
			mediaCapability: contentworld.MediaNone,
		},
		{
			name:            "video",
			key:             contentworld.Video,
			label:           "影像",
			sortOrder:       30,
			views:           []contentworld.ViewMode{contentworld.ViewNone},
			categoryEnabled: true,
			mediaCapability: contentworld.MediaPrimaryVideo,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := contentworld.Lookup(tc.key)
			if !ok {
				t.Fatalf("Lookup(%q) returned ok=false", tc.key)
			}
			if got.Key != tc.key {
				t.Fatalf("definition key = %q, want %q", got.Key, tc.key)
			}
			if got.DefaultLabel != tc.label {
				t.Fatalf("definition label = %q, want %q", got.DefaultLabel, tc.label)
			}
			if got.SortOrder != tc.sortOrder {
				t.Fatalf("definition sort_order = %d, want %d", got.SortOrder, tc.sortOrder)
			}
			if !slices.Equal(got.AllowedViews, tc.views) {
				t.Fatalf("definition views = %v, want %v", got.AllowedViews, tc.views)
			}
			if got.CategoryEnabled != tc.categoryEnabled {
				t.Fatalf("definition category_enabled = %v, want %v", got.CategoryEnabled, tc.categoryEnabled)
			}
			if got.MediaCapability != tc.mediaCapability {
				t.Fatalf("definition media_capability = %q, want %q", got.MediaCapability, tc.mediaCapability)
			}
		})
	}
}

func TestLookupRejectsUnknownWorld(t *testing.T) {
	if _, ok := contentworld.Lookup(contentworld.Key("book")); ok {
		t.Fatal("Lookup(book) returned ok=true, want false")
	}
}
