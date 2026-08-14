# IAM policy

`ssm-env` needs read access to the path it's configured to fetch, plus KMS
decrypt access if any parameters under that path are `SecureString`. Scope
both to the specific path prefix — don't grant account-wide SSM access.

## Minimal policy

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SsmEnvReadParameters",
      "Effect": "Allow",
      "Action": "ssm:GetParametersByPath",
      "Resource": "arn:aws:ssm:REGION:ACCOUNT_ID:parameter/staging/myapp/*"
    },
    {
      "Sid": "SsmEnvDecrypt",
      "Effect": "Allow",
      "Action": "kms:Decrypt",
      "Resource": "arn:aws:kms:REGION:ACCOUNT_ID:key/KEY_ID"
    }
  ]
}
```

Replace `REGION`, `ACCOUNT_ID`, `/staging/myapp/*`, and `KEY_ID` (the KMS key
your `SecureString` parameters are encrypted under — `alias/aws/ssm` if
you're using the AWS-managed default).

## Where this attaches

- **EC2**: attach to the instance profile / IAM role the instance uses.
  `ssm-env` picks this up via IMDSv2 automatically — no extra configuration.
- **ECS/Fargate**: attach to the task role (not the task execution role).
  `ssm-env` picks this up via `AWS_CONTAINER_CREDENTIALS_RELATIVE_URI`,
  which ECS sets automatically.
- **Local dev**: attach to whatever IAM user/role your local AWS credentials
  resolve to (env vars or `~/.aws/credentials`).

## Common failure

If credentials are found but lack `ssm:GetParametersByPath`, `ssm-env` exits
non-zero with the underlying AWS error plus a hint to check the IAM policy —
it does not distinguish "no credentials" from "wrong permissions" beyond
surfacing the SDK's own error text, so check for `AccessDeniedException` in
the message specifically.
