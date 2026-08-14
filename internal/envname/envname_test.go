package envname

import "testing"

func TestFromParam(t *testing.T) {
	cases := []struct{ paramName, prefix, want string }{
		{"/staging/DB_HOST", "/staging/", "DB_HOST"},
		{"/staging/DB_HOST", "/staging", "DB_HOST"},
		{"/staging/db/password", "/staging/", "db_password"},
		{"/staging/a/b/c", "/staging/", "a_b_c"},
		{"/a/B", "/a/", "B"},
	}
	for _, c := range cases {
		if got := FromParam(c.paramName, c.prefix); got != c.want {
			t.Errorf("FromParam(%q, %q) = %q, want %q", c.paramName, c.prefix, got, c.want)
		}
	}
}
