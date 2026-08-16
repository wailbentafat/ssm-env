package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	envconfig "github.com/wailbentafat/ssm-env/internal/config"
	"github.com/wailbentafat/ssm-env/internal/provider"
)

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantFlags   []string
		wantCommand []string
	}{
		{"no args", nil, nil, nil},
		{"flags only, no separator", []string{"--version"}, []string{"--version"}, nil},
		{"separator with command", []string{"--", "myapp"}, []string{}, []string{"myapp"}},
		{"flags then command", []string{"--version", "--", "myapp", "--flag1", "--flag2"},
			[]string{"--version"}, []string{"myapp", "--flag1", "--flag2"}},
		{"separator with nothing after", []string{"--"}, []string{}, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotFlags, gotCommand := splitCommand(c.args)
			if !equalStrings(gotFlags, c.wantFlags) {
				t.Errorf("flagArgs = %v, want %v", gotFlags, c.wantFlags)
			}
			if !equalStrings(gotCommand, c.wantCommand) {
				t.Errorf("commandArgs = %v, want %v", gotCommand, c.wantCommand)
			}
		})
	}
}

// equalStrings compares string slices treating nil and empty as equal, so
// tests don't need to care which one splitCommand happens to return.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type nilSSMClient struct{}

func (nilSSMClient) GetParametersByPath(ctx context.Context, in *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	return nil, errors.New("not called in this test")
}

type nilSMClient struct{}

func (nilSMClient) GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return nil, errors.New("not called in this test")
}

type nilRDSClient struct{}

func (nilRDSClient) DescribeDBInstances(ctx context.Context, in *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return nil, errors.New("not called in this test")
}

func TestBuildProvider(t *testing.T) {
	cases := []struct {
		backend string
		want    any
		wantErr bool
	}{
		{envconfig.BackendSSM, provider.SSM{}, false},
		{envconfig.BackendSecretsManager, provider.SecretsManager{}, false},
		{envconfig.BackendBoth, provider.Multi{}, false},
		{"bogus", nil, true},
	}
	for _, c := range cases {
		t.Run(c.backend, func(t *testing.T) {
			got, err := buildProvider(envconfig.Config{Backend: c.backend}, nilSSMClient{}, nilSMClient{}, nilRDSClient{})
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotType := reflect.TypeOf(got)
			wantType := reflect.TypeOf(c.want)
			if gotType != wantType {
				t.Errorf("got provider type %v, want %v", gotType, wantType)
			}
		})
	}
}
