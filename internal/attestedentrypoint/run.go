package attestedentrypoint

import (
	"context"
	"fmt"
	"strings"
)

type Attestor interface {
	Attest(context.Context, Config) (string, error)
}

type TokenExchanger interface {
	Exchange(context.Context, Config, string) (string, error)
}

type SecretReader interface {
	ReadMnemonic(context.Context, Config, string) (string, error)
}

type Executor interface {
	Exec(binary string, args []string, env []string) error
}

type Dependencies struct {
	Attestor       Attestor
	TokenExchanger TokenExchanger
	SecretReader   SecretReader
	Executor       Executor
}

func (d Dependencies) Validate() error {
	if d.Attestor == nil {
		return fmt.Errorf("attestor dependency is required")
	}
	if d.TokenExchanger == nil {
		return fmt.Errorf("token exchanger dependency is required")
	}
	if d.SecretReader == nil {
		return fmt.Errorf("secret reader dependency is required")
	}
	if d.Executor == nil {
		return fmt.Errorf("executor dependency is required")
	}
	return nil
}

func Run(ctx context.Context, args []string, environ []string, deps Dependencies) error {
	if err := deps.Validate(); err != nil {
		return err
	}
	cfg, err := LoadConfig(environ)
	if err != nil {
		return err
	}

	attestationToken, err := deps.Attestor.Attest(ctx, cfg)
	if err != nil {
		return fmt.Errorf("attestation failed: %w", err)
	}
	if err := ValidateAttestationClaims(attestationToken, cfg); err != nil {
		return fmt.Errorf("attestation token rejected: %w", err)
	}

	accessToken, err := deps.TokenExchanger.Exchange(ctx, cfg, attestationToken)
	if err != nil {
		return fmt.Errorf("STS token exchange failed: %w", err)
	}
	mnemonic, err := deps.SecretReader.ReadMnemonic(ctx, cfg, accessToken)
	if err != nil {
		return fmt.Errorf("Secret Manager read failed: %w", err)
	}
	if strings.TrimSpace(mnemonic) == "" {
		return fmt.Errorf("Secret Manager returned an empty mnemonic")
	}

	childArgs := []string{cfg.MintdBinary}
	if len(args) > 1 {
		childArgs = append(childArgs, args[1:]...)
	}
	return deps.Executor.Exec(cfg.MintdBinary, childArgs, withMnemonic(environ, mnemonic))
}

func withMnemonic(environ []string, mnemonic string) []string {
	out := make([]string, 0, len(environ)+1)
	for _, kv := range environ {
		if strings.HasPrefix(kv, "CDK_MINTD_MNEMONIC=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "CDK_MINTD_MNEMONIC="+mnemonic)
	return out
}
