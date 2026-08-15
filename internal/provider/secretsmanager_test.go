package provider

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type fakeSMClient struct {
	byID map[string]string // secret id -> SecretString
	err  error
}

func (f fakeSMClient) GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	s, ok := f.byID[*in.SecretId]
	if !ok {
		return &secretsmanager.GetSecretValueOutput{}, nil // no SecretString set
	}
	return &secretsmanager.GetSecretValueOutput{SecretString: &s}, nil
}

func TestSecretsManager_PlainStringSecret(t *testing.T) {
	client := fakeSMClient{byID: map[string]string{
		"prod/db-password": "the-secret-value",
	}}
	p := SecretsManager{Client: client, SecretIDs: []string{"prod/db-password"}}

	got, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"DB_PASSWORD": "the-secret-value"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSecretsManager_JSONSecret(t *testing.T) {
	client := fakeSMClient{byID: map[string]string{
		"prod/database": `{"host":"db.internal","password":"correct horse battery staple","port":"5432"}`,
	}}
	p := SecretsManager{Client: client, SecretIDs: []string{"prod/database"}}

	got, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"DATABASE_HOST":     "db.internal",
		"DATABASE_PASSWORD": "correct horse battery staple",
		"DATABASE_PORT":     "5432",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSecretsManager_MultipleSecretIDs(t *testing.T) {
	client := fakeSMClient{byID: map[string]string{
		"prod/db-password": "s3cr3t",
		"prod/api-key":     "abc123",
	}}
	p := SecretsManager{Client: client, SecretIDs: []string{"prod/db-password", "prod/api-key"}}

	got, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"DB_PASSWORD": "s3cr3t", "API_KEY": "abc123"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSecretsManager_APIError(t *testing.T) {
	client := fakeSMClient{err: errors.New("access denied")}
	p := SecretsManager{Client: client, SecretIDs: []string{"prod/db-password"}}

	_, err := p.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSecretsManager_BinarySecretUnsupported(t *testing.T) {
	client := fakeSMClient{byID: map[string]string{}} // GetSecretValue succeeds with no SecretString
	p := SecretsManager{Client: client, SecretIDs: []string{"prod/binary-blob"}}

	_, err := p.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for binary secret, got nil")
	}
}
