package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type fakeSSMClient struct {
	params []types.Parameter
}

func (f fakeSSMClient) GetParametersByPath(ctx context.Context, in *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	return &ssm.GetParametersByPathOutput{Parameters: f.params}, nil
}

func strp(s string) *string { return &s }

func TestSSM_Fetch(t *testing.T) {
	client := fakeSSMClient{params: []types.Parameter{
		{Name: strp("/staging/DB_HOST"), Value: strp("db.internal")},
		{Name: strp("/staging/db/password"), Value: strp("s3cr3t")},
	}}
	p := SSM{Client: client, Path: "/staging/"}

	got, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"DB_HOST": "db.internal", "db_password": "s3cr3t"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
