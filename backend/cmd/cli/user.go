package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/postgres"
)

// commandTimeout bounds one CLI command.
//
// Generous because creating a user includes an Argon2id hash of roughly 200 ms
// and an operator typing a password before that. It exists so a command against
// an unreachable database fails rather than hanging.
const commandTimeout = 2 * time.Minute

// withService opens a pool, builds the auth service and runs fn.
//
// The pool is closed on every path. A CLI process is short-lived, so a leaked
// connection would be released by exit anyway, but "the process exits soon" is
// not a resource policy.
func withService(fn func(context.Context, *auth.Service) error) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	pool, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()

	// Logging to stderr at warn level: a CLI run should print its result on
	// stdout, not an info log for every step. Errors still surface.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	service := auth.NewService(
		auth.NewRepository(pool),
		auth.WithSessionLifetime(cfg.Session.Lifetime),
		auth.WithLogger(logger),
	)

	return fn(ctx, service)
}

// newPrompter builds a prompter over the real terminal.
func newPrompter(in *os.File, out io.Writer) *prompter {
	fd := int(in.Fd())
	return &prompter{
		in:           in,
		out:          out,
		isTerminal:   func() bool { return term.IsTerminal(fd) },
		readPassword: func() ([]byte, error) { return term.ReadPassword(fd) },
	}
}

// createUser creates one account.
//
// The password is read interactively without echo, or from a pipe when
// --password-stdin is given. It is never a flag value: argv reaches shell
// history and every ps on the machine.
func createUser(args []string) error {
	fs := flag.NewFlagSet("user:create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	username := fs.String("username", "", "account name, 3 to 64 characters")
	displayName := fs.String("display-name", "", "name shown on the site (optional)")
	role := fs.String("role", "", `stored on the account, never used for access control (default "owner")`)
	passwordStdin := fs.Bool("password-stdin", false, "read the password from stdin instead of prompting")

	// Rejected explicitly so the failure names the reason. Without this the flag
	// package would report "flag provided but not defined", which reads like an
	// oversight rather than a decision.
	fs.String("password", "", "REJECTED: passing a password on the command line puts it in shell history")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if isFlagSet(fs, "password") {
		return errors.New(
			"--password is not accepted: a password in argv is recorded in shell " +
				"history and visible in ps to every user on this machine; " +
				"omit it to be prompted, or use --password-stdin")
	}

	prompt := newPrompter(os.Stdin, os.Stdout)

	name := *username
	if name == "" {
		// Prompted rather than required, so the command is usable without
		// remembering the flag name.
		name, err := prompt.line("username: ")
		if err != nil {
			return fmt.Errorf("read username: %w", err)
		}
		if name == "" {
			return errors.New("username is required")
		}
		*username = name
	}

	// Validated before the password is asked for. Being told the username is too
	// short after typing a password twice is a bad trade of the operator's time.
	if err := auth.ValidateUsername(*username); err != nil {
		return fmt.Errorf("username %q: %w (%d to %d characters)",
			*username, err, auth.MinUsernameLength, auth.MaxUsernameLength)
	}

	var password string
	var err error
	if *passwordStdin {
		password, err = prompt.passwordFromStdin()
	} else {
		password, err = prompt.password()
	}
	if err != nil {
		return err
	}

	if err := auth.ValidatePassword(password); err != nil {
		return fmt.Errorf("password: %w (%d to %d bytes)",
			err, auth.MinPasswordLength, auth.MaxPasswordLength)
	}

	return withService(func(ctx context.Context, service *auth.Service) error {
		user, err := service.CreateUser(ctx, auth.CreateUserInput{
			Username:    *username,
			Password:    password,
			DisplayName: *displayName,
			Role:        *role,
		})
		if err != nil {
			if errors.Is(err, auth.ErrUsernameTaken) {
				return fmt.Errorf("username %q already exists", *username)
			}
			return err
		}

		fmt.Printf("created user %d\n", user.ID)
		fmt.Printf("  username      %s\n", user.Username)
		fmt.Printf("  role          %s\n", user.Role)
		if user.DisplayName != "" {
			fmt.Printf("  display name  %s\n", user.DisplayName)
		}

		return nil
	})
}

// pruneSessions deletes every expired session.
//
// Expired sessions are already refused at authentication, so this reclaims rows
// rather than closing a hole. Safe to run at any time, and safe to run twice.
func pruneSessions(args []string) error {
	fs := flag.NewFlagSet("session:prune", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	return withService(func(ctx context.Context, service *auth.Service) error {
		removed, err := service.PruneExpiredSessions(ctx)
		if err != nil {
			return err
		}

		// Reported even when zero: a command that prints nothing leaves the operator
		// unsure whether it ran.
		fmt.Printf("removed %d expired session(s)\n", removed)
		return nil
	})
}

// isFlagSet reports whether the operator passed a flag, as opposed to it holding
// its default.
func isFlagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
