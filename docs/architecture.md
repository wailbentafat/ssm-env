# Architecture

`ssm-env` is intentionally small: one binary, one external call type, no
persistent state. This doc explains the internal package layout and why
things are split the way they are, for anyone extending it.

## Package layout

```
main.go                  CLI entry: flag parsing, env var reading, wiring
internal/fetch/           SSM API access (pagination, decryption)
internal/escape/          POSIX shell single-quote escaping
internal/envname/         Parameter name -> env var name mapping
```

Each package does one thing and is tested in isolation, independent of AWS
credentials or a real SSM endpoint:

- **`internal/fetch`** depends on an `SSMClient` interface (the subset of
  the AWS SDK's SSM client this tool actually calls), not the concrete SDK
  client. Tests substitute a fake that returns canned pages, so pagination
  logic is verified without network access or mocking the whole SDK.
- **`internal/escape`** has no AWS dependency at all. Its test suite
  round-trips adversarial values through a real `sh -c` invocation — the
  actual eval path a caller will use — rather than only asserting on the
  escaped string shape, since a plausible-looking escape function can still
  be wrong in ways that only show up when a real shell parses it.
- **`internal/envname`** is a pure string function (prefix stripping) kept
  separate from `fetch` because "how AWS names a parameter" and "how we
  derive an env var name from it" are different concerns that could
  diverge (e.g. if a future version sanitizes invalid env var characters).

## Data flow

```
AWS_ENV_PATH (env var)
        |
        v
config.LoadDefaultConfig()  --  resolves region + credentials via the
        |                        standard SDK chain: env vars -> shared
        |                        config -> EC2 IMDSv2 -> ECS task role
        v
fetch.AllUnderPath()  --  GetParametersByPath, WithDecryption: true,
        |                  following NextToken until exhausted
        v
envname.FromParam()  --  strip path prefix per parameter
        |
        v
escape.ShellSingleQuote()  --  make each value eval-safe
        |
        v
stdout: export NAME='value'   (one line per parameter)
stderr: diagnostics only
```

No step writes to disk. No step caches results. Every invocation repeats
the whole chain from scratch — see the README's "Behavior notes" for why
that's a deliberate v1 choice, not a gap.

## Why AWS SDK v2

The predecessor tool (`Droplr/aws-env`) is built against `aws-sdk-go` v1
from ~2018, before AWS introduced IMDSv2. Its credential resolution only
knows the old, unauthenticated IMDSv1 GET request, so it fails outright on
any instance with `HttpTokens: required`. `aws-sdk-go-v2`'s default
credential provider chain already implements the IMDSv2
token-then-fetch handshake correctly, so adopting it is most of the actual
fix — not custom protocol code in this repo.

## Extension points

- **Additional backend (e.g. Secrets Manager)**: would live as a sibling to
  `internal/fetch`, behind a small interface `main.go` selects between
  based on configuration. Not implemented in v1 (see README Scope).
- **`--strict` flag**: would change the zero-parameters branch in `main.go`
  from "warn and exit 0" to "error and exit 1". Deferred until there's real
  usage data on how often a typo'd path actually happens.
