// Command ssm-env prints `export KEY=VALUE` lines for every AWS SSM
// Parameter Store parameter under the path in $AWS_ENV_PATH, so a shell
// entrypoint can `eval` its output to load secrets into the process
// environment. See README.md for the full usage and migration guide from
// Droplr/aws-env.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/wailbentafat/ssm-env/internal/envname"
	"github.com/wailbentafat/ssm-env/internal/escape"
	"github.com/wailbentafat/ssm-env/internal/fetch"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

const (
	pathEnvVar = "AWS_ENV_PATH"

	// fetchTimeout bounds config loading (including IMDS calls) and the SSM
	// API call, so an unreachable IMDS endpoint or a slow SSM response can't
	// hang a container's boot sequence indefinitely.
	fetchTimeout = 10 * time.Second
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("ssm-env", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "ssm-env prints `export KEY=VALUE` lines for SSM parameters under $%s.\n\n", pathEnvVar)
		fmt.Fprintf(stderr, "Usage:\n  eval \"$(ssm-env)\"\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	path := os.Getenv(pathEnvVar)
	if path == "" {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "ssm-env: failed to load AWS configuration: %v\n", err)
		return 1
	}

	client := ssm.NewFromConfig(cfg)
	params, err := fetch.AllUnderPath(ctx, client, path)
	if err != nil {
		fmt.Fprintf(stderr, "ssm-env: %v\n\nssm-env: verify AWS credentials are available (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, "+
			"or an EC2 instance / ECS task IAM role) and that they have ssm:GetParametersByPath on %q.\n", err, path)
		return 1
	}

	if len(params) == 0 {
		fmt.Fprintf(stderr, "ssm-env: warning: no parameters found under %q\n", path)
		return 0
	}

	for _, p := range params {
		name := envname.FromParam(p.Name, path)
		fmt.Fprintf(stdout, "export %s=%s\n", name, escape.ShellSingleQuote(p.Value))
	}
	return 0
}
