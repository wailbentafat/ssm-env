// Package fetch retrieves and decrypts SSM Parameter Store parameters under
// a path prefix, handling pagination transparently.
package fetch

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// SSMClient is the subset of the SSM API this package needs, so callers can
// substitute a fake in tests instead of talking to real AWS.
type SSMClient interface {
	GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
}

// Param is a single decrypted parameter with its full SSM name.
type Param struct {
	Name  string
	Value string
}

// AllUnderPath fetches every parameter under path, following pagination and
// decrypting SecureString values, and returns them once the whole path has
// been read.
func AllUnderPath(ctx context.Context, client SSMClient, path string) ([]Param, error) {
	var out []Param
	var nextToken *string

	for {
		resp, err := client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
			Path:           &path,
			Recursive:      aBool(true),
			WithDecryption: aBool(true),
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching parameters under %q: %w", path, err)
		}

		for _, p := range resp.Parameters {
			out = append(out, Param{Name: derefName(p), Value: derefValue(p)})
		}

		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		nextToken = resp.NextToken
	}

	return out, nil
}

func aBool(b bool) *bool { return &b }

func derefName(p types.Parameter) string {
	if p.Name == nil {
		return ""
	}
	return *p.Name
}

func derefValue(p types.Parameter) string {
	if p.Value == nil {
		return ""
	}
	return *p.Value
}
