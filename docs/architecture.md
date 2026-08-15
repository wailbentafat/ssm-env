# Architecture

`ssm-env` is intentionally small: one binary, no persistent state, secret
*fetching* kept separate from secret *sourcing* and from *output*. This doc
explains the internal package layout and why things are split the way they
are, for anyone extending it.

## Package layout

```
main.go                    CLI entry: flag parsing, wiring config -> provider -> filter -> output/exec
internal/config/           Env var parsing into a typed Config, no AWS/IO
internal/provider/         SecretProvider interface + SSM, SecretsManager, Multi implementations
internal/fetch/            SSM API access (pagination, decryption) -- used by provider.SSM
internal/secretname/       Secrets Manager secret name/key -> env var name mapping
internal/envname/          SSM parameter name -> env var name mapping
internal/escape/           POSIX shell single-quote escaping (legacy/eval mode only)
internal/declared/         Set of already-declared env var names (for AWS_ENV_ONLY_DECLARED)
internal/execwrap/         os.Setenv + syscall.Exec process-replace wrapper (exec mode only)
```

Each package does one thing and is tested in isolation, independent of AWS
credentials or a real SSM/Secrets Manager endpoint:

- **`internal/config`** is a pure function of `[]string` ("KEY=VALUE"
  pairs) to a `Config` struct — no `os.Getenv` calls anywhere else in the
  codebase. This is what "config decoupled from business logic" means
  here: `main.go` and `internal/provider` only ever see a `Config` value,
  never read the environment directly, so swapping the config source
  (flags, a file) later touches only this package.
- **`internal/provider`** defines the `SecretProvider` interface
  (`Fetch(ctx) (map[string]string, error)`) that `main.go` depends on,
  plus three implementations: `SSM`, `SecretsManager`, and `Multi` (which
  wraps several providers and merges their results, later ones winning on
  key collisions). `main.go` never talks to the AWS SDK directly — it
  builds one `SecretProvider` from `Config` and calls `Fetch` once.
- **`internal/fetch`** depends on an `SSMClient` interface (the subset of
  the AWS SDK's SSM client this tool actually calls), not the concrete SDK
  client. Tests substitute a fake that returns canned pages, so pagination
  logic is verified without network access or mocking the whole SDK.
  `provider.SecretsManager` follows the same pattern with a
  `SecretsManagerClient` interface.
- **`internal/escape`** has no AWS dependency at all. Its test suite
  round-trips adversarial values through a real `sh -c` invocation — the
  actual eval path a caller will use — rather than only asserting on the
  escaped string shape, since a plausible-looking escape function can still
  be wrong in ways that only show up when a real shell parses it. Exec
  mode never calls this package: secrets go into `os.Setenv` directly, not
  through a shell, so there's nothing to escape.
- **`internal/envname`** / **`internal/secretname`** are pure string
  functions kept separate from `fetch`/`provider` because "how AWS names a
  parameter/secret" and "how we derive an env var name from it" are
  different concerns that could diverge per backend (SSM strips a
  known path prefix; Secrets Manager has no equivalent prefix, so it uses
  the secret's basename instead).
- **`internal/execwrap`** isolates the one piece of process-replacing,
  irreversible-by-nature code (`syscall.Exec`) behind a small `Run(args,
  secrets)` function, so it can be tested for its error paths (missing
  command, unresolvable binary) without actually replacing the test
  process — the success path is inherently untestable in-process, since
  it never returns.

## Data flow

```
Config (internal/config.Load, from os.Environ())
        |
        v
config.LoadDefaultConfig()  --  resolves region + credentials via the
        |                        standard SDK chain: env vars -> shared
        |                        config -> EC2 IMDSv2 -> ECS task role
        v
buildProvider(cfg)  --  selects provider.SSM, provider.SecretsManager,
        |                or provider.Multi{SSM, SecretsManager} based on
        |                cfg.Backend
        v
provider.Fetch(ctx)  --  map[envVarName]value, backend-specific:
        |                 SSM: fetch.AllUnderPath + envname.FromParam
        |                 Secrets Manager: one GetSecretValue per secret ID,
        |                 JSON vs plain-string detection, secretname mapping
        v
declared.Names() filter  --  if AWS_ENV_ONLY_DECLARED, drop names not
        |                     already present in the container's env
        v
   +---------------------------+
   |                           |
no command given          command given after `--`
   |                           |
   v                           v
escape.ShellSingleQuote()  execwrap.Run()  --  os.Setenv per secret, then
   + print to stdout                          syscall.Exec(cmd), replacing
   (legacy/eval mode)                          this process (exec mode)
```

No step writes to disk. No step caches results. Every invocation repeats
the whole chain from scratch — see the README's "Behavior notes" for why
that's deliberate, not a gap.

## Why AWS SDK v2

The predecessor tool (`Droplr/aws-env`) is built against `aws-sdk-go` v1
from ~2018, before AWS introduced IMDSv2. Its credential resolution only
knows the old, unauthenticated IMDSv1 GET request, so it fails outright on
any instance with `HttpTokens: required`. `aws-sdk-go-v2`'s default
credential provider chain already implements the IMDSv2
token-then-fetch handshake correctly, so adopting it is most of the actual
fix — not custom protocol code in this repo.

## Why exec mode replaces the process instead of forking

`execwrap.Run` uses `syscall.Exec`, not `os/exec.Command(...).Run()`. Exec
replaces the current process image in place — same PID, no parent process
left running. A forked child, by contrast, would leave `ssm-env` alive as
the container's PID 1, responsible for reaping the child and forwarding
signals correctly (SIGTERM on `docker stop`, etc.) — a whole class of
init-process bugs this design avoids by simply not existing anymore once
the target command starts.

## Extension points

- **Additional backend (e.g. Vault, GCP Secret Manager)**: implement
  `provider.SecretProvider` (one `Fetch(ctx) (map[string]string, error)`
  method) as a new type in `internal/provider`, wire it into
  `buildProvider` in `main.go`. No changes needed to `main.go`'s
  fetch/filter/output flow, `internal/config`, or exec mode.
- **`--strict` flag**: would change the zero-secrets branch in `main.go`
  from "warn and exit 0" to "error and exit 1". Deferred until there's real
  usage data on how often a typo'd path/secret ID actually happens.
