package provider

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type fakeRDSClient struct {
	byInstanceID map[string]string // db instance id -> secret ARN ("" means no managed secret)
	err          error
}

func (f fakeRDSClient) DescribeDBInstances(ctx context.Context, in *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	arn, ok := f.byInstanceID[*in.DBInstanceIdentifier]
	if !ok {
		return &rds.DescribeDBInstancesOutput{}, nil
	}
	instance := types.DBInstance{}
	if arn != "" {
		instance.MasterUserSecret = &types.MasterUserSecret{SecretArn: &arn}
	}
	return &rds.DescribeDBInstancesOutput{DBInstances: []types.DBInstance{instance}}, nil
}

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

func TestSecretsManager_PrefixOverride(t *testing.T) {
	client := fakeSMClient{byID: map[string]string{
		"rds!db-9a65eafa-3c3b-4687-8ead-447e3544d645": `{"username":"postgres","password":"s3cr3t","host":"db.internal","port":5432}`,
	}}
	p := SecretsManager{
		Client:    client,
		SecretIDs: []string{"rds!db-9a65eafa-3c3b-4687-8ead-447e3544d645|POSTGRES"},
	}

	got, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"POSTGRES_USERNAME": "postgres",
		"POSTGRES_PASSWORD": "s3cr3t",
		"POSTGRES_HOST":     "db.internal",
		"POSTGRES_PORT":     "5432",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSecretsManager_PrefixOverride_PlainString(t *testing.T) {
	client := fakeSMClient{byID: map[string]string{
		"rds!db-9a65eafa": "raw-value",
	}}
	p := SecretsManager{Client: client, SecretIDs: []string{"rds!db-9a65eafa|POSTGRES"}}

	got, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"POSTGRES": "raw-value"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSecretsManager_RDSLookup(t *testing.T) {
	rdsClient := fakeRDSClient{byInstanceID: map[string]string{
		"db-bricoram-staging": "rds!db-9a65eafa-3c3b-4687-8ead-447e3544d645",
	}}
	smClient := fakeSMClient{byID: map[string]string{
		"rds!db-9a65eafa-3c3b-4687-8ead-447e3544d645": `{"username":"postgres","password":"s3cr3t"}`,
	}}
	p := SecretsManager{
		Client:    smClient,
		RDSClient: rdsClient,
		SecretIDs: []string{"rds:db-bricoram-staging|POSTGRES"},
	}

	got, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"POSTGRES_USERNAME": "postgres", "POSTGRES_PASSWORD": "s3cr3t"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSecretsManager_RDSLookup_NoClientConfigured(t *testing.T) {
	p := SecretsManager{Client: fakeSMClient{}, SecretIDs: []string{"rds:db-bricoram-staging"}}

	_, err := p.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSecretsManager_RDSLookup_InstanceNotFound(t *testing.T) {
	p := SecretsManager{
		Client:    fakeSMClient{},
		RDSClient: fakeRDSClient{byInstanceID: map[string]string{}},
		SecretIDs: []string{"rds:db-does-not-exist"},
	}

	_, err := p.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSecretsManager_RDSLookup_NoManagedSecret(t *testing.T) {
	p := SecretsManager{
		Client:    fakeSMClient{},
		RDSClient: fakeRDSClient{byInstanceID: map[string]string{"db-bricoram-staging": ""}},
		SecretIDs: []string{"rds:db-bricoram-staging"},
	}

	_, err := p.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSecretsManager_RDSLookup_DescribeError(t *testing.T) {
	p := SecretsManager{
		Client:    fakeSMClient{},
		RDSClient: fakeRDSClient{err: errors.New("access denied")},
		SecretIDs: []string{"rds:db-bricoram-staging"},
	}

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
