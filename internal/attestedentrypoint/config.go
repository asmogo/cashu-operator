package attestedentrypoint

import (
	"fmt"
	"strings"
)

const (
	defaultAudience              = "https://sts.googleapis.com"
	defaultHWModel               = "GCP_AMD_SEV"
	defaultMintdBinary           = "cdk-mintd"
	defaultSecretManagerEndpoint = "https://secretmanager.googleapis.com"
	defaultSTSEndpoint           = "https://sts.googleapis.com/v1/token"
	defaultTPMPath               = "/dev/tpmrm0"
)

// Config is populated from environment variables so the wrapper image can stay
// generic across GCP projects, pools, and Secret Manager paths.
type Config struct {
	AttestationProjectID     string
	AttestationLocation      string
	WorkloadIdentityProvider string
	MnemonicSecretVersion    string
	ExpectedAudience         string
	ExpectedHWModel          string
	ExpectedProjectID        string
	ExpectedServiceAccount   string
	ExpectedZone             string
	MintdBinary              string
	SecretManagerEndpoint    string
	STSEndpoint              string
	TPMPath                  string
}

func LoadConfig(environ []string) (Config, error) {
	env := envMap(environ)
	cfg := Config{
		AttestationProjectID:     env["CASHU_ATTESTATION_PROJECT_ID"],
		AttestationLocation:      env["CASHU_ATTESTATION_LOCATION"],
		WorkloadIdentityProvider: env["CASHU_ATTESTATION_WORKLOAD_IDENTITY_PROVIDER"],
		MnemonicSecretVersion:    env["CASHU_MNEMONIC_SECRET_VERSION"],
		ExpectedAudience:         valueOrDefault(env["CASHU_ATTESTATION_EXPECTED_AUDIENCE"], defaultAudience),
		ExpectedHWModel:          valueOrDefault(env["CASHU_ATTESTATION_EXPECTED_HW_MODEL"], defaultHWModel),
		ExpectedProjectID:        env["CASHU_ATTESTATION_EXPECTED_PROJECT_ID"],
		ExpectedServiceAccount:   env["CASHU_ATTESTATION_EXPECTED_SERVICE_ACCOUNT"],
		ExpectedZone:             env["CASHU_ATTESTATION_EXPECTED_ZONE"],
		MintdBinary:              valueOrDefault(env["CASHU_MINTD_BINARY"], defaultMintdBinary),
		SecretManagerEndpoint:    strings.TrimRight(valueOrDefault(env["CASHU_SECRET_MANAGER_ENDPOINT"], defaultSecretManagerEndpoint), "/"),
		STSEndpoint:              valueOrDefault(env["CASHU_STS_ENDPOINT"], defaultSTSEndpoint),
		TPMPath:                  valueOrDefault(env["CASHU_TPM_PATH"], defaultTPMPath),
	}
	if cfg.ExpectedProjectID == "" {
		cfg.ExpectedProjectID = cfg.AttestationProjectID
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	required := map[string]string{
		"CASHU_ATTESTATION_PROJECT_ID":                 c.AttestationProjectID,
		"CASHU_ATTESTATION_LOCATION":                   c.AttestationLocation,
		"CASHU_ATTESTATION_WORKLOAD_IDENTITY_PROVIDER": c.WorkloadIdentityProvider,
		"CASHU_ATTESTATION_EXPECTED_SERVICE_ACCOUNT":   c.ExpectedServiceAccount,
		"CASHU_MNEMONIC_SECRET_VERSION":                c.MnemonicSecretVersion,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

func envMap(environ []string) map[string]string {
	env := make(map[string]string, len(environ))
	for _, kv := range environ {
		name, value, ok := strings.Cut(kv, "=")
		if ok {
			env[name] = value
		}
	}
	return env
}

func valueOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
