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
