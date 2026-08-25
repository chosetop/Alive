package auth_test

import (
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/auth"
)

const lifetime = 7 * 24 * time.Hour

func TestSessionIsExpired(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "expiry in the future",
			expiresAt: now.Add(time.Hour),
			want:      false,
		},
		{
			name:      "expiry in the past",
			expiresAt: now.Add(-time.Hour),
			want:      true,
		},
		{
			// Treated as expired. At exactly the boundary the lifetime is over,
			// and the strict reading is the safe one for a credential.
			name:      "expiry exactly now",
			expiresAt: now,
			want:      true,
		},
		{
			name:      "expiry one nanosecond away",
			expiresAt: now.Add(time.Nanosecond),
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := auth.Session{ExpiresAt: tc.expiresAt}
			if got := s.IsExpired(now); got != tc.want {
				t.Errorf("IsExpired() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSessionNeedsRenewal(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "fresh session, full lifetime left",
			expiresAt: now.Add(lifetime),
			want:      false,
		},
		{
			name:      "just past halfway",
			expiresAt: now.Add(lifetime/2 - time.Minute),
			want:      true,
		},
		{
			// Not yet: the check is "less than half remains", so the exact
			// halfway point still waits one more request.
			name:      "exactly halfway",
			expiresAt: now.Add(lifetime / 2),
			want:      false,
		},
		{
			name:      "almost expired",
			expiresAt: now.Add(time.Minute),
			want:      true,
		},
		{
			// An expired session is replaced, not renewed. Returning true here
			// would let a dead session be revived by using it.
			name:      "already expired",
			expiresAt: now.Add(-time.Minute),
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := auth.Session{ExpiresAt: tc.expiresAt}
			if got := s.NeedsRenewal(now, lifetime); got != tc.want {
				t.Errorf("NeedsRenewal() = %v, want %v", got, tc.want)
			}
		})
	}
}
