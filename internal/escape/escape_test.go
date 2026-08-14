package escape

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestShellSingleQuote(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", "'hello'"},
		{"empty", "", "''"},
		{"space", "hello world", "'hello world'"},
		{"single quote", "it's", `'it'\''s'`},
		{"double quote", `say "hi"`, `'say "hi"'`},
		{"dollar", "$HOME", "'$HOME'"},
		{"backtick", "`whoami`", "'`whoami`'"},
		{"command sub", "$(rm -rf /)", "'$(rm -rf /)'"},
		{"newline", "line1\nline2", "'line1\nline2'"},
		{"only quotes", "'''", `''\'''\'''\'''`},
		{"leading trailing quote", "'x'", `''\''x'\'''`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ShellSingleQuote(c.in)
			if got != c.want {
				t.Errorf("ShellSingleQuote(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestShellSingleQuoteRoundTrip is the adversarial guard the PRD calls out
// explicitly: every escaped value, when actually eval'd by a real POSIX
// shell as `export NAME=<escaped>`, must reproduce the original value
// byte-for-byte and must never execute embedded commands.
func TestShellSingleQuoteRoundTrip(t *testing.T) {
	adversarial := []string{
		"",
		"plain",
		"hello world",
		"it's a test",
		`say "hi"`,
		"$HOME",
		"`whoami`",
		"$(whoami)",
		"${PATH}",
		"line1\nline2\nline3",
		"'''",
		"a'b'c'd",
		"; rm -rf /",
		"&& echo pwned",
		"| cat /etc/passwd",
		"\\'; echo pwned; echo '",
		"back\\slash",
		"tab\there",
	}

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	for _, val := range adversarial {
		val := val
		t.Run(val, func(t *testing.T) {
			escaped := ShellSingleQuote(val)
			script := fmt.Sprintf("export SSM_ENV_TEST_VAR=%s\nprintf '%%s' \"$SSM_ENV_TEST_VAR\"", escaped)
			out, err := exec.Command("sh", "-c", script).Output()
			if err != nil {
				t.Fatalf("sh -c failed: %v", err)
			}
			if string(out) != val {
				t.Errorf("round trip mismatch: got %q, want %q (escaped form: %s)", string(out), val, escaped)
			}
		})
	}
}
