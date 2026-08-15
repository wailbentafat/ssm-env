package provider

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeProvider struct {
	secrets map[string]string
	err     error
}

func (f fakeProvider) Fetch(ctx context.Context) (map[string]string, error) {
	return f.secrets, f.err
}

func TestMulti_MergesInOrder(t *testing.T) {
	m := Multi{Providers: []SecretProvider{
		fakeProvider{secrets: map[string]string{"A": "1", "SHARED": "from-first"}},
		fakeProvider{secrets: map[string]string{"B": "2", "SHARED": "from-second"}},
	}}

	got, err := m.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"A": "1", "B": "2", "SHARED": "from-second"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMulti_PropagatesError(t *testing.T) {
	m := Multi{Providers: []SecretProvider{
		fakeProvider{secrets: map[string]string{"A": "1"}},
		fakeProvider{err: errors.New("boom")},
	}}

	_, err := m.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
