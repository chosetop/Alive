// Command cli is the operator entry point for tasks that must not be reachable
// over HTTP.
//
// It currently holds one command, config:check, which loads and validates
// configuration and reports the result. That command needs no business logic,
// so it can exist now; the commands this binary is meant for (creating the
// owner account, removing orphaned objects from storage) belong to the stages
// that build those features and are deliberately absent.
//
// Usage:
//
//	go run ./cmd/cli config:check
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/p30huiwei/alive/backend/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no command given")
	}

	switch args[0] {
	case "config:check":
		return checkConfig()
	case "user:create":
		return createUser(args[1:])
	case "session:prune":
		return pruneSessions(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// checkConfig reports whether the current environment produces a valid
// configuration. It prints no secret: the DSN is described, never echoed.
func checkConfig() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println("configuration is valid")
	fmt.Printf("  env             %s\n", cfg.Env)
	fmt.Printf("  listen          %s\n", cfg.Server.Addr())
	fmt.Printf("  database dsn    set (%d chars, not printed)\n", len(cfg.Database.DSN))
	fmt.Printf("  db max conns    %d\n", cfg.Database.MaxOpenConns)
	fmt.Printf("  cors origins    %v\n", cfg.CORS.AllowedOrigins)
	fmt.Printf("  session ttl     %s\n", cfg.Session.Lifetime)
	fmt.Printf("  session cookie  %s (path %s, secure %t)\n",
		cfg.Session.CookieName, cfg.Session.CookiePath, cfg.Session.CookieSecure)

	if cfg.RateLimit.Enabled {
		fmt.Printf("  login limit     %d per %s per ip\n",
			cfg.RateLimit.LoginAttempts, cfg.RateLimit.LoginWindow)
	} else {
		// Printed as a warning, not a value. A disabled limiter is the kind of
		// setting that gets switched off for one afternoon and stays off.
		fmt.Printf("  login limit     DISABLED\n")
	}

	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `alive cli

Commands:
  config:check    Load and validate configuration, then report it
  user:create     Create an account, prompting for the password
  session:prune   Delete every expired session
  help            Show this message

user:create flags:
  --username      Account name, 3 to 64 characters. Prompted if omitted.
  --display-name  Name shown on the site. Optional.
  --role          Stored on the account, never used for access control.
  --password-stdin
                  Read the password from stdin rather than prompting.

  The password is never taken from a command-line flag. argv is written to
  shell history and is visible in ps to every user on this machine.

Examples:
  go run ./cmd/cli user:create --username owner
  go run ./cmd/cli session:prune
`)
}
