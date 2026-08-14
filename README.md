# ssm-env

A tiny, static Go binary that fetches secrets from AWS SSM Parameter Store
and prints them as `export KEY=VALUE` shell lines, so a container entrypoint
can `eval` its output and load secrets straight into the process environment
— no secrets ever touch disk.

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

### Configuration

All configuration is via environment variables — no flags needed for normal
use, so you don't have to template a command string per environment.

| Variable         | Required | Description                                                                                     |
| ---------------- | -------- | ------------------------------------------------------------------------------------------------- |
| `AWS_ENV_PATH`   | yes      | SSM path prefix to fetch, e.g. `/staging/myapp/`. If unset, `ssm-env` exits `0` with no output.  |
| `AWS_REGION`     | no       | Falls back to the AWS SDK's normal region resolution (env var, shared config, or instance metadata region). |

Credentials are resolved via the standard AWS SDK default credential chain:
environment variables → shared config/credentials file → EC2 IMDSv2 → ECS
task role. Nothing custom — whatever the AWS CLI/SDK would use, `ssm-env`
uses.

```sh
ssm-env --version   # print version and exit
ssm-env --help       # usage
```

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
ARG SSM_ENV_VERSION=v0.1.0
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

- `SecureString` parameters are always decrypted before being printed —
  never printed still-encrypted.
- Pagination is handled transparently: a path with more parameters than fit
  in one SSM API page returns all of them.
- If `AWS_ENV_PATH` is unset, `ssm-env` exits `0` with no output (a no-op),
  so it's safe to call unconditionally from an entrypoint that only
  sometimes needs secrets.
- If the path resolves to zero parameters (typo, wrong environment, etc.),
  `ssm-env` prints a warning to stderr but still exits `0` — a typo'd path
  booting a container with zero secrets and no crash is the current v1
  behavior; this is deliberately permissive rather than defaulting to a hard
  failure.
- If AWS authentication fails, or the credentials present lack
  `ssm:GetParametersByPath`, `ssm-env` exits non-zero with a human-readable
  message on stderr — never a raw SDK panic or stack trace.
- All diagnostics go to stderr; only `export` lines go to stdout. This is
  load-bearing: mixing them would break `eval "$(ssm-env)"`.
- `ssm-env` never writes parameter values to disk, a log file, or anywhere
  but stdout — including its own diagnostics. Secrets stay in memory only.

## Migrating from `aws-env`

`ssm-env` keeps the same `AWS_ENV_PATH` variable and the same
`eval $(aws-env)` embedding pattern as `Droplr/aws-env`, so migrating is a
find-and-replace of where the binary comes from, not a rewrite:

1. In your `Dockerfile`, replace the `aws-env` download with the `ssm-env`
   download shown above.
2. Nothing else changes — `AWS_ENV_PATH` and the `eval` call stay the same.
3. If your instances use `HttpTokens: required` (IMDSv2-only), this is the
   fix for the `NoCredentialProviders: no valid providers in chain` crash.

## Scope (v1)

- SSM Parameter Store only — no Secrets Manager backend yet.
- No daemon mode, background caching, or writing secrets to a tmpfs file.
  Every invocation is a fresh, independent fetch.
- No `/proc/<pid>/environ` manipulation or any way to write into a running
  process's environment directly — `ssm-env` only ever prints to stdout.
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
