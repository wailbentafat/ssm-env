package fetch

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type fakeClient struct {
	pages [][]types.Parameter
	calls int
	err   error
}

func (f *fakeClient) GetParametersByPath(ctx context.Context, in *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	idx := f.calls
	f.calls++
	if idx >= len(f.pages) {
		return &ssm.GetParametersByPathOutput{}, nil
	}
	out := &ssm.GetParametersByPathOutput{Parameters: f.pages[idx]}
	if idx < len(f.pages)-1 {
		tok := "page-token"
		out.NextToken = &tok
	}
	return out, nil
}

func strp(s string) *string { return &s }

func TestAllUnderPath_SinglePage(t *testing.T) {
	client := &fakeClient{
		pages: [][]types.Parameter{
			{
				{Name: strp("/staging/DB_HOST"), Value: strp("db.internal")},
				{Name: strp("/staging/DB_PASS"), Value: strp("s3cr3t")},
			},
		},
	}

	got, err := AllUnderPath(context.Background(), client, "/staging/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d params, want 2", len(got))
	}
	if got[0].Name != "/staging/DB_HOST" || got[0].Value != "db.internal" {
		t.Errorf("unexpected param 0: %+v", got[0])
	}
}

func TestAllUnderPath_Pagination(t *testing.T) {
	client := &fakeClient{
		pages: [][]types.Parameter{
			{{Name: strp("/staging/A"), Value: strp("1")}},
			{{Name: strp("/staging/B"), Value: strp("2")}},
			{{Name: strp("/staging/C"), Value: strp("3")}},
		},
	}

	got, err := AllUnderPath(context.Background(), client, "/staging/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d params across pages, want 3", len(got))
	}
	if client.calls != 3 {
		t.Errorf("expected 3 API calls for 3 pages, got %d", client.calls)
	}
}

func TestAllUnderPath_Empty(t *testing.T) {
	client := &fakeClient{pages: [][]types.Parameter{{}}}

	got, err := AllUnderPath(context.Background(), client, "/nope/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d params, want 0", len(got))
	}
}

func TestAllUnderPath_APIError(t *testing.T) {
	client := &fakeClient{err: errors.New("access denied")}

	_, err := AllUnderPath(context.Background(), client, "/staging/")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
