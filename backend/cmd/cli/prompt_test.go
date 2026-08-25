package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// fixedPrompter builds a prompter whose password source is scripted.
//
// term.ReadPassword itself is not re-tested here; it belongs to
// golang.org/x/term. What is tested is everything this package decides: that a
// password is read twice, that a mismatch is refused, and that a non-terminal
// stdin does not silently fall back to an echoing read.
func fixedPrompter(in string, terminal bool, replies ...string) (*prompter, *bytes.Buffer) {
	out := &bytes.Buffer{}
	remaining := replies

	return &prompter{
		in:         strings.NewReader(in),
		out:        out,
		isTerminal: func() bool { return terminal },
		readPassword: func() ([]byte, error) {
			if len(remaining) == 0 {
				return nil, errors.New("no more scripted replies")
			}
			reply := remaining[0]
			remaining = remaining[1:]
			return []byte(reply), nil
		},
	}, out
}

func TestPasswordRequiresTwoMatchingEntries(t *testing.T) {
	p, out := fixedPrompter("", true, "correct-horse-battery", "correct-horse-battery")

	got, err := p.password()
	if err != nil {
		t.Fatalf("password: %v", err)
	}
	if got != "correct-horse-battery" {
		t.Errorf("password = %q, want the entered value", got)
	}

	// The prompts are written, the password never is.
	printed := out.String()
	if !strings.Contains(printed, "password:") {
		t.Error("no password prompt was printed")
	}
	if !strings.Contains(printed, "confirm password:") {
		t.Error("no confirmation prompt was printed")
	}
	if strings.Contains(printed, "correct-horse-battery") {
		t.Errorf("the password was echoed to the terminal: %q", printed)
	}
}

func TestPasswordRejectsMismatch(t *testing.T) {
	p, out := fixedPrompter("", true, "first-value", "second-value")

	_, err := p.password()
	if err == nil {
		t.Fatal("password() = nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), "do not match") {
		t.Errorf("error = %q, want it to say the passwords do not match", err)
	}
	// Neither value is echoed, not even to show what differed.
	for _, secret := range []string{"first-value", "second-value"} {
		if strings.Contains(out.String(), secret) {
			t.Errorf("output carries %q", secret)
		}
	}
}

// TestPasswordRefusesNonTerminal is the important one. Falling back to a plain
// read here would echo the password to the screen and into the scrollback of
// whatever terminal the operator was using.
func TestPasswordRefusesNonTerminal(t *testing.T) {
	p, _ := fixedPrompter("piped-secret\n", false)

	_, err := p.password()
	if !errors.Is(err, errNoPasswordSource) {
		t.Fatalf("error = %v, want errNoPasswordSource", err)
	}
	// The message has to name the way out, or the operator is stuck.
	if !strings.Contains(err.Error(), "--password-stdin") {
		t.Errorf("error = %q, want it to mention --password-stdin", err)
	}
}

func TestPasswordFromStdin(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "trailing newline is stripped", input: "secret-value\n", want: "secret-value"},
		{name: "CRLF is stripped", input: "secret-value\r\n", want: "secret-value"},
		{name: "no line ending at all", input: "secret-value", want: "secret-value"},
		{
			// TrimSpace would silently alter the password. A space is a legitimate
			// character, and an account whose password was quietly trimmed cannot be
			// logged into with the password its owner typed.
			name:  "inner and edge spaces are kept",
			input: " two words \n",
			want:  " two words ",
		},
		{
			name:  "only the first line is read",
			input: "secret-value\nignored\n",
			want:  "secret-value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := fixedPrompter(tc.input, false)

			got, err := p.passwordFromStdin()
			if err != nil {
				t.Fatalf("passwordFromStdin: %v", err)
			}
			if got != tc.want {
				t.Errorf("password = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPasswordFromStdinRejectsEmpty(t *testing.T) {
	for _, input := range []string{"", "\n"} {
		p, _ := fixedPrompter(input, false)

		if _, err := p.passwordFromStdin(); err == nil {
			t.Errorf("input %q: got nil error, want a refusal", input)
		}
	}
}

func TestLineReadsAndTrims(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain line", input: "owner\n", want: "owner"},
		{name: "surrounding spaces", input: "  owner  \n", want: "owner"},
		{name: "no line ending", input: "owner", want: "owner"},
		{name: "empty line", input: "\n", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, out := fixedPrompter(tc.input, true)

			got, err := p.line("username: ")
			if err != nil {
				t.Fatalf("line: %v", err)
			}
			if got != tc.want {
				t.Errorf("line = %q, want %q", got, tc.want)
			}
			if !strings.Contains(out.String(), "username: ") {
				t.Error("the prompt was not printed")
			}
		})
	}
}
