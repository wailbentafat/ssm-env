// Command ssm-env loads secrets from AWS SSM Parameter Store and/or AWS
// Secrets Manager into a container's environment.
//
// In its default (legacy) mode it prints `export KEY=VALUE` lines to
// stdout, for a shell entrypoint to `eval`. Given a command after `--`, it
// instead loads secrets directly into its own process and execs the
// command, so secrets never pass through stdout or a shell. See README.md
// for the full usage and migration guide from Droplr/aws-env.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	envconfig "github.com/wailbentafat/ssm-env/internal/config"
	"github.com/wailbentafat/ssm-env/internal/declared"
	"github.com/wailbentafat/ssm-env/internal/escape"
	"github.com/wailbentafat/ssm-env/internal/execwrap"
	"github.com/wailbentafat/ssm-env/internal/fetch"
	"github.com/wailbentafat/ssm-env/internal/provider"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

// fetchTimeout bounds config loading (including IMDS calls) and the
// SSM/Secrets Manager API calls, so an unreachable IMDS endpoint or a slow
// API response can't hang a container's boot sequence indefinitely.
const fetchTimeout = 10 * time.Second

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	flagArgs, commandArgs := splitCommand(args)

	fs := flag.NewFlagSet("ssm-env", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "ssm-env loads secrets from SSM Parameter Store and/or Secrets Manager.\n\n")
		fmt.Fprintf(stderr, "Usage:\n  eval \"$(ssm-env)\"          # legacy mode: print export lines\n  ssm-env -- CMD [ARGS...]  # exec mode: load into env, then exec CMD\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	cfg := envconfig.Load(os.Environ())
	if cfg.Path == "" && len(cfg.SecretIDs) == 0 {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	// WithEC2IMDSRegion makes region resolution fall back to IMDS, mirroring
	// how credential resolution already does -- without it, an EC2 instance
	// with no AWS_REGION env var and no ~/.aws/config fails with "missing
	// region" even though it has a perfectly good IAM role.
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithEC2IMDSRegion())
	if err != nil {
		fmt.Fprintf(stderr, "ssm-env: failed to load AWS configuration: %v\n", err)
		return 1
	}

	secretProvider, err := buildProvider(cfg, ssm.NewFromConfig(awsCfg), secretsmanager.NewFromConfig(awsCfg))
	if err != nil {
		fmt.Fprintf(stderr, "ssm-env: %v\n", err)
		return 2
	}

	secrets, err := secretProvider.Fetch(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "ssm-env: %v\n\nssm-env: verify AWS credentials are available (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, "+
			"or an EC2 instance / ECS task IAM role) and that they have the necessary ssm:GetParametersByPath / secretsmanager:GetSecretValue permissions.\n", err)
		return 1
	}

	if len(secrets) == 0 {
		fmt.Fprintf(stderr, "ssm-env: warning: no secrets found\n")
		return 0
	}

	if cfg.OnlyDeclared {
		declaredNames := declared.Names(os.Environ())
		before := len(secrets)
		for name := range secrets {
			if _, ok := declaredNames[name]; !ok {
				delete(secrets, name)
			}
		}
		if len(secrets) == 0 {
			fmt.Fprintf(stderr, "ssm-env: warning: %s is set, but none of the %d secret(s) found match an already-declared "+
				"environment variable name\n", envconfig.OnlyDeclaredEnvVar, before)
			return 0
		}
	}

	if len(commandArgs) > 0 {
		// Run only returns on failure: on success it replaces this process
		// image entirely via syscall.Exec, so nothing after it executes.
		err := execwrap.Run(commandArgs, secrets)
		fmt.Fprintf(stderr, "ssm-env: %v\n", err)
		return 1
	}

	printExports(stdout, secrets)
	return 0
}

// buildProvider selects and wires the SecretProvider(s) named by
// cfg.Backend, keeping backend selection separate from the fetch/filter/
// output flow in run.
func buildProvider(cfg envconfig.Config, ssmClient fetch.SSMClient, smClient provider.SecretsManagerClient) (provider.SecretProvider, error) {
	ssmProvider := provider.SSM{Client: ssmClient, Path: cfg.Path}
	smProvider := provider.SecretsManager{Client: smClient, SecretIDs: cfg.SecretIDs}

	switch cfg.Backend {
	case envconfig.BackendSSM:
		return ssmProvider, nil
	case envconfig.BackendSecretsManager:
		return smProvider, nil
	case envconfig.BackendBoth:
		return provider.Multi{Providers: []provider.SecretProvider{ssmProvider, smProvider}}, nil
	default:
		return nil, fmt.Errorf("%s: unknown backend %q (want %q, %q, or %q)",
			envconfig.BackendEnvVar, cfg.Backend, envconfig.BackendSSM, envconfig.BackendSecretsManager, envconfig.BackendBoth)
	}
}

// splitCommand separates ssm-env's own flags from a target command given
// after a literal "--" separator, e.g. ["--version", "--", "myapp", "-x"]
// becomes (["--version"], ["myapp", "-x"]). Without a "--", every arg is
// treated as an ssm-env flag and there is no command.
func splitCommand(args []string) (flagArgs, commandArgs []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func printExports(stdout *os.File, secrets map[string]string) {
	names := make([]string, 0, len(secrets))
	for name := range secrets {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(stdout, "export %s=%s\n", name, escape.ShellSingleQuote(secrets[name]))
	}
}
