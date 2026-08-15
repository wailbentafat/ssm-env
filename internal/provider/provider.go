// Package provider abstracts fetching secrets from a backing store into a
// common interface, so main.go's flow (fetch, filter, output) doesn't need
// to know whether secrets came from SSM Parameter Store, Secrets Manager,
// or both.
package provider

import "context"

// SecretProvider fetches secrets and returns them as env var name -> value.
type SecretProvider interface {
	Fetch(ctx context.Context) (map[string]string, error)
}

// Multi wraps several providers and merges their results. Providers are
// fetched in order; where the same key is produced by more than one
// provider, the later provider in the list wins.
type Multi struct {
	Providers []SecretProvider
}

func (m Multi) Fetch(ctx context.Context) (map[string]string, error) {
	merged := make(map[string]string)
	for _, p := range m.Providers {
		secrets, err := p.Fetch(ctx)
		if err != nil {
			return nil, err
		}
		for k, v := range secrets {
			merged[k] = v
		}
	}
	return merged, nil
}
