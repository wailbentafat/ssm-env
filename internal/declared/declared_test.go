package declared

import "testing"

func TestNames(t *testing.T) {
	environ := []string{
		"DB_HOST=",
		"DB_PASSWORD=already-set",
		"PATH=/usr/bin:/bin",
		"WEIRD=has=equals=signs",
	}

	set := Names(environ)

	for _, name := range []string{"DB_HOST", "DB_PASSWORD", "PATH", "WEIRD"} {
		if _, ok := set[name]; !ok {
			t.Errorf("expected %q in declared set", name)
		}
	}
	if _, ok := set["NOT_PRESENT"]; ok {
		t.Error("did not expect NOT_PRESENT in declared set")
	}
	if len(set) != 4 {
		t.Errorf("got %d names, want 4", len(set))
	}
}

func TestNames_Empty(t *testing.T) {
	set := Names(nil)
	if len(set) != 0 {
		t.Errorf("got %d names, want 0", len(set))
	}
}

// TestNames_ComposeTrailingEquals locks in the specific scenario the
// README documents for Docker Compose: `- DB_HOST=` (trailing `=`) sets an
// empty-valued env var that Go's os.Environ() does report, so Names must
// pick it up. Compose's *other* syntax, bare `- DB_HOST` (no `=`), never
// creates the variable in the container at all when it's unset on the
// host -- verified against a real `docker compose up` -- so there is
// nothing to test on this side for that form: it simply never reaches
// os.Environ() as an entry, and Names correctly has no way to see it.
func TestNames_ComposeTrailingEquals(t *testing.T) {
	// Mirrors `environment: [DB_HOST=, DB_PASSWORD=]` in a compose file.
	environ := []string{"DB_HOST=", "DB_PASSWORD="}

	set := Names(environ)

	for _, name := range []string{"DB_HOST", "DB_PASSWORD"} {
		if _, ok := set[name]; !ok {
			t.Errorf("expected %q (declared via Compose trailing-= syntax) in declared set", name)
		}
	}
}
