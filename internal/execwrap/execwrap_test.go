package execwrap

import "testing"

func TestRun_NoCommand(t *testing.T) {
	if err := Run(nil, nil); err == nil {
		t.Fatal("expected error for empty command, got nil")
	}
}

func TestRun_CommandNotFound(t *testing.T) {
	err := Run([]string{"ssm-env-definitely-does-not-exist-binary"}, map[string]string{"FOO": "bar"})
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}
