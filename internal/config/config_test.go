package config

import (
	"reflect"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := Load(nil)
	if cfg.Path != "" {
		t.Errorf("Path = %q, want empty", cfg.Path)
	}
	if cfg.SecretIDs != nil {
		t.Errorf("SecretIDs = %v, want nil", cfg.SecretIDs)
	}
	if cfg.Backend != BackendSSM {
		t.Errorf("Backend = %q, want %q", cfg.Backend, BackendSSM)
	}
	if cfg.OnlyDeclared {
		t.Error("OnlyDeclared = true, want false")
	}
}

func TestLoad_AllSet(t *testing.T) {
	environ := []string{
		"AWS_ENV_PATH=/staging/myapp/",
		"AWS_ENV_SECRET_IDS=staging/db-creds, staging/api-key",
		"AWS_ENV_BACKEND=both",
		"AWS_ENV_ONLY_DECLARED=true",
		"UNRELATED=ignored",
	}
	cfg := Load(environ)

	if cfg.Path != "/staging/myapp/" {
		t.Errorf("Path = %q", cfg.Path)
	}
	want := []string{"staging/db-creds", "staging/api-key"}
	if !reflect.DeepEqual(cfg.SecretIDs, want) {
		t.Errorf("SecretIDs = %v, want %v", cfg.SecretIDs, want)
	}
	if cfg.Backend != BackendBoth {
		t.Errorf("Backend = %q, want %q", cfg.Backend, BackendBoth)
	}
	if !cfg.OnlyDeclared {
		t.Error("OnlyDeclared = false, want true")
	}
}

func TestLoad_SecretIDsCSVEdgeCases(t *testing.T) {
	cases := map[string][]string{
		"":             nil,
		"a":            {"a"},
		"a,b":          {"a", "b"},
		"a, b":         {"a", "b"},
		"a,,b":         {"a", "b"},
		"a,":           {"a"},
		" a , b ,,c ,": {"a", "b", "c"},
	}
	for in, want := range cases {
		got := splitCSV(in)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("splitCSV(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLoad_BackendDefaultsWhenEmpty(t *testing.T) {
	cfg := Load([]string{"AWS_ENV_BACKEND="})
	if cfg.Backend != BackendSSM {
		t.Errorf("Backend = %q, want default %q", cfg.Backend, BackendSSM)
	}
}
