package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// errNoPasswordSource reports that a password was needed and stdin was not a
// terminal.
var errNoPasswordSource = errors.New(
	"stdin is not a terminal, so a password cannot be read without echo; " +
		"pass --password-stdin to read it from the pipe instead")

// prompter reads operator input.
//
// The password source is a field rather than a direct call to term.ReadPassword
// so that tests can drive every path here. Terminal handling itself is not
// re-tested: it belongs to golang.org/x/term.
type prompter struct {
	in  io.Reader
	out io.Writer

	// isTerminal reports whether in is an interactive terminal.
	isTerminal func() bool

	// readPassword reads one line with echo disabled. Called only when
	// isTerminal returns true.
	readPassword func() ([]byte, error)
}

// line prints a prompt and reads one line, trimmed.
//
// Echoed, so it is for names, not secrets.
func (p *prompter) line(prompt string) (string, error) {
	fmt.Fprint(p.out, prompt)

	reader := bufio.NewReader(p.in)
	text, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	// A line ending in EOF rather than \n is still a line; only an empty read
	// with EOF means there was nothing there.
	if text == "" && err == io.EOF {
		return "", io.EOF
	}

	return strings.TrimSpace(text), nil
}

// password reads a password twice and returns it only when both match.
//
// Never taken from a command-line argument. argv is visible in shell history and
// in ps output for every user on the machine, and a password that has been
// written to either is no longer a secret.
//
// The two readings are compared with != rather than a constant-time compare.
// Constant time protects a stored secret from an attacker measuring a remote
// server; both of these strings were typed by the same person at this keyboard a
// second ago, and using subtle here would imply a threat that does not exist.
func (p *prompter) password() (string, error) {
	if !p.isTerminal() {
		return "", errNoPasswordSource
	}

	fmt.Fprint(p.out, "password: ")
	first, err := p.readPassword()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	// The terminal echoed nothing, including the newline the operator typed.
	fmt.Fprintln(p.out)

	fmt.Fprint(p.out, "confirm password: ")
	second, err := p.readPassword()
	if err != nil {
		return "", fmt.Errorf("read confirmation: %w", err)
	}
	fmt.Fprintln(p.out)

	if string(first) != string(second) {
		// Nothing is echoed about what differed. The operator retries the command.
		return "", errors.New("the two passwords do not match")
	}

	return string(first), nil
}

// passwordFromStdin reads a password from a pipe, for non-interactive use.
//
// Explicit by flag, never a fallback. Silently accepting piped input would mean
// a mistyped command in a script creates an account whose password came from
// whatever happened to be on stdin.
//
// Only the first line is read, and a trailing newline is stripped: `echo secret
// | ...` appends one, and treating it as part of the password would create an
// account nobody can log into.
func (p *prompter) passwordFromStdin() (string, error) {
	reader := bufio.NewReader(p.in)
	text, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}

	// Only the line ending is removed. Spaces inside or at the edge of a password
	// are legitimate characters, so TrimSpace would silently change the secret.
	password := strings.TrimRight(text, "\r\n")
	if password == "" {
		return "", errors.New("no password on stdin")
	}

	return password, nil
}
