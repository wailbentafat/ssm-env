package provider

import (
	"context"

	"github.com/wailbentafat/ssm-env/internal/envname"
	"github.com/wailbentafat/ssm-env/internal/fetch"
)

// SSM is a SecretProvider backed by AWS SSM Parameter Store.
type SSM struct {
	Client fetch.SSMClient
	Path   string
}

func (s SSM) Fetch(ctx context.Context) (map[string]string, error) {
	params, err := fetch.AllUnderPath(ctx, s.Client, s.Path)
	if err != nil {
		return nil, err
	}

	secrets := make(map[string]string, len(params))
	for _, p := range params {
		secrets[envname.FromParam(p.Name, s.Path)] = p.Value
	}
	return secrets, nil
}
