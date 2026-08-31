package taxonomy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

func TestNormalizeTagNameTrimsWhitespace(t *testing.T) {
	if got := taxonomy.NormalizeTagName("  京都  "); got != "京都" {
		t.Fatalf("NormalizeTagName = %q, want 京都", got)
	}
}

func TestValidateTagNameCountsRunes(t *testing.T) {
	if err := taxonomy.ValidateTagName(strings.Repeat("字", 64)); err != nil {
		t.Fatalf("ValidateTagName rejected a 64-rune name: %v", err)
	}
	if err := taxonomy.ValidateTagName(strings.Repeat("字", 65)); !errors.Is(err, taxonomy.ErrInvalidTagName) {
		t.Fatalf("ValidateTagName = %v, want ErrInvalidTagName", err)
	}
}

func TestNormalizeTagSlugUsesSlugRules(t *testing.T) {
	if got := taxonomy.NormalizeTagSlug("  Mixed Case / 2026  "); got != "mixed-case-2026" {
		t.Fatalf("NormalizeTagSlug = %q, want mixed-case-2026", got)
	}
}

func TestValidateTagSlugRejectsEmptyResult(t *testing.T) {
	if err := taxonomy.ValidateTagSlug("   "); !errors.Is(err, taxonomy.ErrInvalidTagSlug) {
		t.Fatalf("ValidateTagSlug = %v, want ErrInvalidTagSlug", err)
	}
}
