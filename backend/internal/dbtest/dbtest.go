// Package dbtest holds what every integration test needs to reach the test
// database safely.
//
// It exists because three test packages need the same two things, and because
// the first version of them was wrong in a way that only showed up once a
// second table existed.
//
// The wrong version: each package truncated `users CASCADE` before and after its
// tests. `go test ./...` runs packages in parallel, so one package's truncate
// deleted the rows another package was in the middle of using. With only users
// and sessions the window was narrow enough that it rarely bit. Once entries
// arrived, a test that created an author and then inserted several entries had a
// window wide enough to lose the author mid-test, and the insert failed on the
// foreign key. Measured: five failures in eight runs.
//
// The fix is not to serialise the packages. It is to stop each test assuming it
// owns the whole table. A test names its rows with a unique prefix and removes
// only those, so two packages running at once cannot see each other's data.
//
// Under internal/, so nothing outside this module can depend on it.
package dbtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/postgres"
)

// requiredSuffix is what a database name must end in before these helpers will
// delete anything in it.
const requiredSuffix = "_test"

// DSN returns the test database DSN, skipping the test when none is configured.
//
// Skip rather than fail: `go test ./...` has to work on a machine with no
// PostgreSQL. The integration run is the one that sets TEST_DATABASE_URL.
func DSN(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping database integration test")
	}
	requireTestDatabase(t, dsn)
	return dsn
}

// requireTestDatabase refuses a database that is not obviously a test database.
//
// These helpers delete rows. Pointed at a development database they would delete
// real content, and silently: the tests would still pass, so nothing would report
// the loss.
//
// Fatal, not Skip. Skipping would let a wrong DSN pass as a green run, which is
// the failure this check exists to prevent. A guard here as well as in the
// Makefile, because the Makefile only governs one command, and anyone exporting
// the variable by hand is exactly the case where a mistake is most likely.
func requireTestDatabase(t *testing.T, dsn string) {
	t.Helper()

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL is not a valid URL: %v", err)
	}

	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		t.Fatalf("TEST_DATABASE_URL names no database: %q", u.Redacted())
	}
	if !strings.HasSuffix(name, requiredSuffix) {
		t.Fatalf(
			"refusing to run against database %q: these tests delete rows, which "+
				"would destroy real content. The database name must end in %q. "+
				"Run `make test-db-create` once, then `make test-integration`.",
			name, requiredSuffix)
	}
}

// Pool opens a connection pool to the test database and closes it on cleanup.
func Pool(t *testing.T) *postgres.Pool {
	t.Helper()

	pool, err := postgres.New(context.Background(), config.DatabaseConfig{
		DSN:            DSN(t),
		MaxOpenConns:   4,
		MaxIdleConns:   1,
		ConnectTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to the test database: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// counter makes every generated name unique within a process.
var counter atomic.Uint64

// Username returns a username unique to this test, prefixed so that cleanup can
// find it.
//
// Unique per call, not per test: a test that creates two accounts needs two
// names, and one of them being a duplicate would fail on the unique constraint
// rather than on what the test is about.
//
// The prefix carries the process id, so two `go test` invocations against the
// same database do not collide either.
func Username(t *testing.T, label string) string {
	t.Helper()

	// citext makes the column case-insensitive, so a name differing only in case
	// is the same name. Lowercased here to keep that from surprising a caller.
	return strings.ToLower(fmt.Sprintf("%s%s%d", Prefix(), label, counter.Add(1)))
}

// Prefix is the marker every row created by this process carries.
//
// Cleanup deletes by this prefix, which is what lets two packages run at once:
// each process removes only what it made.
//
// Lowercase letters and digits only, so it is also valid inside a slug.
func Prefix() string {
	return fmt.Sprintf("t%dx", os.Getpid())
}

// Slug returns a slug unique to this process, in the format the column accepts.
//
// Entry slugs are unique across the whole table, so two processes using the
// literal "kyoto-spring" would collide on the unique index. The prefix keeps them
// apart, and the result still matches the CHECK constraint: lowercase letters and
// digits in groups joined by single hyphens.
func Slug(t *testing.T, label string) string {
	t.Helper()
	return fmt.Sprintf("%s-%s-%d", Prefix(), label, counter.Add(1))
}

// CleanupUsers deletes the accounts this process created, and everything that
// cascades from them, before and after the calling test.
//
// Scoped by prefix rather than TRUNCATE. A truncate would take rows belonging to
// a package running in parallel, which is the bug this package was written to
// remove. Deleting before as well as after means a previous crashed run leaves
// nothing behind that a later assertion could count.
func CleanupUsers(t *testing.T, pool *postgres.Pool) {
	t.Helper()

	clean := func() {
		ctx := context.Background()
		pattern := Prefix() + "%"

		// Order matters, and it is not the same as the cascade order.
		//
		// sessions cascades from users, so it needs no statement. entries does NOT:
		// author_id is ON DELETE RESTRICT, deliberately, so that deleting an account
		// cannot silently take written content with it. That means the entries have
		// to go first or the delete below fails on the foreign key.
		//
		// The pattern comes from the process id rather than from test input, and is
		// passed as a parameter regardless.
		if _, err := pool.Exec(ctx,
			`DELETE FROM entries
			 WHERE author_id IN (SELECT id FROM users WHERE username LIKE $1)`,
			pattern); err != nil {
			t.Fatalf("clean up test entries: %v", err)
		}

		if _, err := pool.Exec(ctx,
			"DELETE FROM users WHERE username LIKE $1", pattern); err != nil {
			t.Fatalf("clean up test users: %v", err)
		}
	}

	clean()
	t.Cleanup(clean)
}

// CleanupCategories deletes the categories this process created, before and after
// the calling test.
//
// Separate from CleanupUsers rather than folded into it, because the two are not
// ordered with respect to each other: a category has no owner, so a test that
// needs categories may need no users at all.
//
// Scoped by the slug prefix, for the same reason as everything else here: two
// packages run at once, and a truncate would take the other one's rows.
//
// No entries statement, and none is needed. entries.category_id is ON DELETE SET
// NULL, deliberately: deleting a category uncategorises its entries rather than
// refusing or removing them. That is the opposite of author_id, which is RESTRICT,
// and it is why this can delete categories without touching entries first.
func CleanupCategories(t *testing.T, pool *postgres.Pool) {
	t.Helper()

	clean := func() {
		pattern := Prefix() + "%"
		if _, err := pool.Exec(context.Background(),
			"DELETE FROM categories WHERE slug LIKE $1", pattern); err != nil {
			t.Fatalf("clean up test categories: %v", err)
		}
	}

	clean()
	t.Cleanup(clean)
}
