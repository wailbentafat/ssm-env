# ssm-env

A tiny, static Go binary that fetches secrets from AWS SSM Parameter Store
and/or AWS Secrets Manager and either prints them as `export KEY=VALUE`
shell lines (for a container entrypoint to `eval`) or loads them straight
into a child process's environment via `ssm-env -- CMD` — no secrets ever
touch disk, and in exec mode, no secrets ever touch stdout either.

It's a maintained, modern successor to [Droplr/aws-env](https://github.com/Droplr/aws-env),
which is unmaintained, built against Go 1.10 and a ~2018-era AWS SDK, and
**crashes on any EC2 instance that requires IMDSv2** (the secure, modern
default for instance metadata) with:

```
NoCredentialProviders: no valid providers in chain
```

`aws-env` only knows how to make the old, unauthenticated IMDSv1 GET request
for instance credentials. If your instance has `HttpTokens: required` set
(IMDSv2-only — the AWS-recommended and increasingly default configuration),
`aws-env` cannot authenticate at all. `ssm-env` uses the current
[AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2), whose default
credential chain already implements the IMDSv2 token-then-fetch flow
correctly, so this just works.

## Usage

```sh
# In a Dockerfile entrypoint:
export AWS_ENV_PATH=/staging/myapp/
eval "$(ssm-env)"
```

That's it. `ssm-env` reads every parameter under `/staging/myapp/` in SSM
Parameter Store, decrypts any `SecureString` values, strips the path prefix,
and prints:

```sh
export DB_HOST='db.internal'
export DB_PASSWORD='correct horse battery staple'
```

Values are shell-escaped, so secrets containing spaces, quotes, `$`,
backticks, or newlines are safe to `eval`.

### Exec mode: `ssm-env -- CMD`

`eval "$(ssm-env)"` necessarily prints secrets to stdout for the shell to
capture — fine for most setups, but that means secrets pass through a
shell and can land in `set -x` traces, process loggers, or anything else
capturing stdout. Exec mode avoids that entirely:

```dockerfile
ENTRYPOINT ["ssm-env", "--", "myapp", "--flag1", "--flag2"]
```

`ssm-env` fetches secrets, loads them into its own process environment
with `os.Setenv`, then replaces itself with `myapp` via `syscall.Exec` —
same PID, same memory, `ssm-env` itself ceases to exist. Nothing is
printed; no shell is involved. This is the recommended mode for new
setups; the legacy `eval` mode (no `--`) remains the default when no
command is given, so existing Dockerfiles keep working unchanged.

| Invocation           | Behavior                                                          |
| --------------------- | ------------------------------------------------------------------ |
| `ssm-env`             | Legacy mode — prints `export KEY='VALUE'` lines to stdout.        |
| `ssm-env -- myapp`    | Exec mode — loads secrets into the process env, execs `myapp`.    |

### Configuration

All configuration is via environment variables — no flags needed for normal
use, so you don't have to template a command string per environment.

| Variable                | Required | Description                                                                                     |
| ------------------------ | -------- | ------------------------------------------------------------------------------------------------- |
| `AWS_ENV_PATH`           | no*      | SSM path prefix to fetch, e.g. `/staging/myapp/`.                                                |
| `AWS_ENV_SECRET_IDS`     | no*      | Comma-separated Secrets Manager secret names or ARNs to fetch, e.g. `staging/db-creds,staging/api-key`. |
| `AWS_ENV_BACKEND`        | no       | Which backend(s) to fetch from: `ssm` (default), `secretsmanager`, or `both`.                    |
| `AWS_REGION`             | no       | Falls back to the AWS SDK's normal region resolution: env var → shared config file → EC2 instance metadata region (IMDS), in that order. |
| `AWS_ENV_ONLY_DECLARED`  | no       | If `true`, only export secrets whose name is already declared as an env var in the container (see below). Default: export everything found. Applies to both backends. |

\* At least one of `AWS_ENV_PATH` or `AWS_ENV_SECRET_IDS` must be set for
`ssm-env` to fetch anything; if both are unset, `ssm-env` exits `0` with no
output. Which of the two matters depends on `AWS_ENV_BACKEND` — e.g.
`AWS_ENV_SECRET_IDS` is ignored when `AWS_ENV_BACKEND=ssm` (the default).

Credentials are resolved via the standard AWS SDK default credential chain:
environment variables → shared config/credentials file → EC2 IMDSv2 → ECS
task role. Nothing custom — whatever the AWS CLI/SDK would use, `ssm-env`
uses.

```sh
ssm-env --version   # print version and exit
ssm-env --help       # usage
```

### AWS Secrets Manager backend

Set `AWS_ENV_BACKEND=secretsmanager` (or `both`, to combine with SSM) and
list secrets in `AWS_ENV_SECRET_IDS`:

```sh
export AWS_ENV_BACKEND=secretsmanager
export AWS_ENV_SECRET_IDS=prod/db-password,prod/database
eval "$(ssm-env)"
```

Each secret is mapped to env var(s) based on its content:

- **Plain string secret** named `prod/db-password` with value
  `the-secret-value` becomes one env var. The name is derived from the last
  `/`-separated segment of the secret name/ARN, with `-` replaced by `_`,
  uppercased:
  ```sh
  export DB_PASSWORD='the-secret-value'
  ```
- **JSON object secret** named `prod/database` becomes one env var per
  top-level key, prefixed the same way:
  ```json
  {"host": "db.internal", "password": "correct horse battery staple", "port": "5432"}
  ```
  ```sh
  export DATABASE_HOST='db.internal'
  export DATABASE_PASSWORD='correct horse battery staple'
  export DATABASE_PORT='5432'
  ```

With `AWS_ENV_BACKEND=both`, `ssm-env` fetches from SSM and Secrets Manager
and merges the results; if the same env var name comes from both, the
Secrets Manager value wins:

```yaml
environment:
  - AWS_ENV_BACKEND=both
  - AWS_ENV_PATH=/staging/myapp/           # SSM parameters
  - AWS_ENV_SECRET_IDS=staging/db-creds    # Secrets Manager
```

Binary (non-string) secret values are not supported and cause `ssm-env` to
exit non-zero with an error naming the secret.

### Only exporting declared variables

By default `ssm-env` exports every secret it finds (from SSM, Secrets
Manager, or both), even if several services share the same path prefix or
secret. If you'd rather a container only receive the secrets it actually
declares it needs, set `AWS_ENV_ONLY_DECLARED=true`. `ssm-env` will then
only export secrets whose name is already present as an environment
variable in the container — the filter runs after fetching and merging, so
it applies the same way regardless of backend.

**This requires the trailing `=` form in Compose, not the bare-name form.**
Compose has two different `environment:` syntaxes and they behave
differently:

- `- DB_HOST` (bare, no `=`) tells Compose to look `DB_HOST` up on the
  *host* shell (or `.env` file) that ran `docker compose up`. If it's not
  set there, Compose **does not create the variable in the container at
  all** — not even empty. `ssm-env` (and anything else checking
  `os.Environ()`) will never see it.
- `- DB_HOST=` (trailing `=`, explicit empty value) tells Compose to set
  `DB_HOST` in the container directly, to an empty string, right now. This
  is the form `ssm-env` needs — the variable genuinely exists in the
  container's environment (as an empty value) before `ssm-env` runs, so it
  shows up as a name `ssm-env` can match against and fill in.

```yaml
services:
  app:
    image: myapp
    environment:
      - AWS_ENV_PATH=/staging/myapp/
      - AWS_ENV_ONLY_DECLARED=true
      - DB_HOST=      # trailing = required -- bare "DB_HOST" is silently ignored
      - DB_PASSWORD=  # same
```

Here, even if `/staging/myapp/` in SSM has a dozen parameters, only
`DB_HOST` and `DB_PASSWORD` get exported — because those are the only names
this container already declared. No separate allowlist file to maintain;
the Compose file you're already writing is the allowlist. If none of the
fetched parameters match a declared name, `ssm-env` prints a warning to
stderr (and still exits `0`).

### Nested parameter names

SSM lets you nest parameters further for organization, e.g.
`/staging/myapp/db/password` under path `/staging/myapp/`. Any slashes left
after stripping the prefix are converted to underscores, so that becomes
`export db_password=...` rather than the invalid `export db/password=...`.
If you want a flat `DB_PASSWORD`, name the parameter without nesting.

### Re-running inside a live container (`docker exec`)

An entrypoint's `eval "$(ssm-env)"` only exports variables into that
process's own environment (PID 1). A separate `docker exec` session does
**not** inherit those — so re-run `ssm-env` in the exec session too:

```sh
docker exec app sh -c 'eval "$(ssm-env)" && python manage.py migrate'
```

`ssm-env` is stateless and fast (see below), so re-fetching per `docker exec`
call is a non-issue, not something to work around with caching. There is no
cache or hidden state — every invocation is a fresh, independent fetch.

## Installing

### In a Dockerfile (the primary use case)

```dockerfile
ARG SSM_ENV_VERSION=v2.0.0
ADD https://github.com/wailbentafat/ssm-env/releases/download/${SSM_ENV_VERSION}/ssm-env_linux_amd64 /usr/local/bin/ssm-env
ADD https://github.com/wailbentafat/ssm-env/releases/download/${SSM_ENV_VERSION}/checksums.txt /tmp/checksums.txt
RUN grep "ssm-env_linux_amd64$" /tmp/checksums.txt | sha256sum -c - \
    && chmod +x /usr/local/bin/ssm-env \
    && rm /tmp/checksums.txt
```

For `arm64` (e.g. Graviton), use `ssm-env_linux_arm64` instead.

### Locally, via `go install`

```sh
go install github.com/wailbentafat/ssm-env@latest
```

## Size and performance

- Binary size: ~9 MB, statically linked (`CGO_ENABLED=0`), no libc/glibc
  dependency. Runs unmodified in `scratch`, `alpine`, `distroless`, or any
  other base image — verified against a bare `scratch` image with nothing
  else installed.
- Cold start: single-digit milliseconds for the no-op case
  (`AWS_ENV_PATH` unset); a real fetch of ~50 parameters comfortably
  completes in well under 1 second, including the AWS API round trip.

Regressions here (bloating past tens of MB, adding multi-second startup)
would defeat the point of replacing a fast, tiny tool — treat them as bugs.

## Behavior notes

- `SecureString` SSM parameters are always decrypted before being used —
  never printed or exec'd still-encrypted.
- Pagination is handled transparently: an SSM path with more parameters
  than fit in one API page returns all of them.
- If neither `AWS_ENV_PATH` nor `AWS_ENV_SECRET_IDS` is set, `ssm-env`
  exits `0` with no output (a no-op), so it's safe to call unconditionally
  from an entrypoint that only sometimes needs secrets.
- If fetching resolves to zero secrets (typo'd path, empty secret list,
  wrong environment, etc.), `ssm-env` prints a warning to stderr but still
  exits `0` — a typo booting a container with zero secrets and no crash is
  deliberate, rather than defaulting to a hard failure.
- If AWS authentication fails, or the credentials present lack the
  necessary `ssm:GetParametersByPath` / `secretsmanager:GetSecretValue`
  permission, `ssm-env` exits non-zero with a human-readable message on
  stderr — never a raw SDK panic or stack trace.
- In legacy (eval) mode, all diagnostics go to stderr; only `export` lines
  go to stdout. This is load-bearing: mixing them would break
  `eval "$(ssm-env)"`.
- In exec mode, secrets are loaded via `os.Setenv` and never printed
  anywhere, by either `ssm-env` or the child process's `execve` call.
- `ssm-env` never writes secret values to disk or a log file. Secrets
  stay in memory only (and, in exec mode, in the replaced process's own
  environment).

## Migrating from `aws-env`

`ssm-env` keeps the same `AWS_ENV_PATH` variable and the same
`eval $(aws-env)` embedding pattern as `Droplr/aws-env`, so migrating is a
find-and-replace of where the binary comes from, not a rewrite:

1. In your `Dockerfile`, replace the `aws-env` download with the `ssm-env`
   download shown above.
2. Nothing else changes — `AWS_ENV_PATH` and the `eval` call stay the same.
3. If your instances use `HttpTokens: required` (IMDSv2-only), this is the
   fix for the `NoCredentialProviders: no valid providers in chain` crash.

## Scope

- SSM Parameter Store and AWS Secrets Manager only — no Vault, GCP Secret
  Manager, or Azure Key Vault (the `SecretProvider` interface supports
  adding one; see [`docs/architecture.md`](docs/architecture.md)).
- No daemon mode, background caching, or writing secrets to a tmpfs file.
  Every invocation is a fresh, independent fetch.
- No secret write/management commands — `ssm-env` is read-only by design.
- No `/proc/<pid>/environ` manipulation of an already-running process —
  exec mode loads secrets into `ssm-env`'s own environment before
  replacing itself with the target command, it does not reach into an
  unrelated running process.
- No custom retry/backoff flags — relies on the AWS SDK's own default retry
  behavior.
- Linux only (`linux/amd64`, `linux/arm64`); no Windows containers.
- No secret rotation or "notify on change" — this is a fetch-once tool.

## Further reading

- [`docs/architecture.md`](docs/architecture.md) — package layout, data flow, why AWS SDK v2 fixes the IMDSv2 crash.
- [`docs/iam-policy.md`](docs/iam-policy.md) — minimal IAM policy for `ssm:GetParametersByPath` + KMS decrypt, and where it attaches on EC2 vs ECS.

## Development

```sh
go build ./...
go vet ./...
go test ./... -v
```

Releases are built via [GoReleaser](https://goreleaser.com/) on every
`v*` tag push (see `.github/workflows/release.yml`), producing checksummed
static binaries for `linux/amd64` and `linux/arm64`.

## License

MIT — see [LICENSE](LICENSE).
