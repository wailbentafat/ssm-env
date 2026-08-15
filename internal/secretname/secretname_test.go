package secretname

import "testing"

func TestFromSecretID(t *testing.T) {
	cases := map[string]string{
		"prod/db-password": "DB_PASSWORD",
		"prod/database":    "DATABASE",
		"db-password":      "DB_PASSWORD",
		"a/b/c-d":          "C_D",
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db-password-AbCdEf": "DB_PASSWORD_ABCDEF",
	}
	for in, want := range cases {
		if got := FromSecretID(in); got != want {
			t.Errorf("FromSecretID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromSecretKey(t *testing.T) {
	cases := []struct {
		prefix, key, want string
	}{
		{"DATABASE", "host", "DATABASE_HOST"},
		{"DATABASE", "password", "DATABASE_PASSWORD"},
		{"DATABASE", "max-conns", "DATABASE_MAX_CONNS"},
	}
	for _, c := range cases {
		if got := FromSecretKey(c.prefix, c.key); got != c.want {
			t.Errorf("FromSecretKey(%q, %q) = %q, want %q", c.prefix, c.key, got, c.want)
		}
	}
}
