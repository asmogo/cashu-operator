package attestedentrypoint

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"
	"time"
)

func TestRunSuccessExecsMintdWithMnemonic(t *testing.T) {
	exec := &fakeExecutor{}
	deps := Dependencies{
		Attestor:       fakeAttestor{token: testToken(t, map[string]any{})},
		TokenExchanger: fakeExchanger{token: "sts-token"},
		SecretReader:   fakeSecretReader{mnemonic: "abandon abandon abandon"},
		Executor:       exec,
	}

	err := Run(context.Background(), []string{"cashu-attested-entrypoint", "--log", "debug"}, testEnv(), deps)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if exec.binary != defaultMintdBinary {
		t.Errorf("binary = %q, want %q", exec.binary, defaultMintdBinary)
	}
	if !slices.Equal(exec.args, []string{defaultMintdBinary, "--log", "debug"}) {
		t.Errorf("args = %#v", exec.args)
	}
	if !slices.Contains(exec.env, "CDK_MINTD_MNEMONIC=abandon abandon abandon") {
		t.Error("CDK_MINTD_MNEMONIC was not passed to child environment")
	}
	if slices.Contains(exec.env, "CDK_MINTD_MNEMONIC=old") {
		t.Error("existing CDK_MINTD_MNEMONIC was not removed")
	}
}

func TestRunMissingVTPMFailsClosed(t *testing.T) {
	exec := &fakeExecutor{}
	deps := Dependencies{
		Attestor:       fakeAttestor{err: errors.New("vTPM device /dev/tpmrm0 unavailable")},
		TokenExchanger: fakeExchanger{token: "sts-token"},
		SecretReader:   fakeSecretReader{mnemonic: "mnemonic"},
		Executor:       exec,
	}

	err := Run(context.Background(), []string{"cashu-attested-entrypoint"}, testEnv(), deps)
	if err == nil {
		t.Fatal("expected Run to fail")
	}
	if exec.called {
		t.Fatal("mintd was started after attestation failure")
	}
}

func TestRunSecretManagerDenialFailsClosed(t *testing.T) {
	exec := &fakeExecutor{}
	deps := Dependencies{
		Attestor:       fakeAttestor{token: testToken(t, map[string]any{})},
		TokenExchanger: fakeExchanger{token: "sts-token"},
		SecretReader:   fakeSecretReader{err: errors.New("permission denied")},
		Executor:       exec,
	}

	err := Run(context.Background(), []string{"cashu-attested-entrypoint"}, testEnv(), deps)
	if err == nil {
		t.Fatal("expected Run to fail")
	}
	if exec.called {
		t.Fatal("mintd was started after Secret Manager denial")
	}
}

func TestValidateAttestationClaimsRejectsWrongClaims(t *testing.T) {
	tests := map[string]map[string]any{
		"audience":        {"aud": "https://example.invalid"},
		"hwmodel":         {"hwmodel": "GCP_INTEL_TDX"},
		"project":         {"submods": map[string]any{"gce": map[string]any{"project_id": "other-project", "zone": "us-central1-a"}}},
		"service account": {"google_service_accounts": []string{"other@cashu-prod.iam.gserviceaccount.com"}},
		"secure boot":     {"secboot": false},
	}

	cfg := testConfig()
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateAttestationClaims(testToken(t, overrides), cfg); err == nil {
				t.Fatal("expected validation to fail")
			}
		})
	}
}

func TestResolveBinarySearchesPath(t *testing.T) {
	tmp := t.TempDir()
	binaryPath := tmp + "/cdk-mintd"
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp)

	got, err := resolveBinary("cdk-mintd")
	if err != nil {
		t.Fatalf("resolveBinary returned error: %v", err)
	}
	if got != binaryPath {
		t.Fatalf("resolveBinary = %q, want %q", got, binaryPath)
	}
}

func TestRunTrimsMnemonic(t *testing.T) {
	exec := &fakeExecutor{}
	deps := Dependencies{
		Attestor:       fakeAttestor{token: testToken(t, map[string]any{})},
		TokenExchanger: fakeExchanger{token: "sts-token"},
		SecretReader:   fakeSecretReader{mnemonic: " abandon abandon abandon \n"},
		Executor:       exec,
	}

	err := Run(context.Background(), []string{"cashu-attested-entrypoint"}, testEnv(), deps)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !slices.Contains(exec.env, "CDK_MINTD_MNEMONIC=abandon abandon abandon") {
		t.Fatalf("trimmed mnemonic not passed to child environment: %#v", exec.env)
	}
}

func testEnv() []string {
	return []string{
		"CASHU_ATTESTATION_PROJECT_ID=cashu-prod",
		"CASHU_ATTESTATION_LOCATION=us-central1",
		"CASHU_ATTESTATION_WORKLOAD_IDENTITY_PROVIDER=//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/cashu/providers/confidential-gke",
		"CASHU_ATTESTATION_EXPECTED_SERVICE_ACCOUNT=cashumint-attested@cashu-prod.iam.gserviceaccount.com",
		"CASHU_ATTESTATION_EXPECTED_ZONE=us-central1-a",
		"CASHU_MNEMONIC_SECRET_VERSION=projects/cashu-prod/secrets/mint-mnemonic/versions/latest",
		"CDK_MINTD_MNEMONIC=old",
	}
}

func testConfig() Config {
	cfg, err := LoadConfig(testEnv())
	if err != nil {
		panic(err)
	}
	return cfg
}

func testToken(t *testing.T, overrides map[string]any) string {
	t.Helper()
	claims := map[string]any{
		"aud":                     defaultAudience,
		"exp":                     time.Now().Add(time.Hour).Unix(),
		"hwmodel":                 defaultHWModel,
		"iss":                     "https://confidentialcomputing.googleapis.com",
		"nbf":                     time.Now().Add(-time.Minute).Unix(),
		"secboot":                 true,
		"google_service_accounts": []string{"cashumint-attested@cashu-prod.iam.gserviceaccount.com"},
		"submods": map[string]any{
			"gce": map[string]any{
				"project_id": "cashu-prod",
				"zone":       "us-central1-a",
			},
		},
	}
	for key, value := range overrides {
		claims[key] = value
	}
	header := map[string]any{"alg": "RS256", "typ": "JWT"}
	return jwtPart(t, header) + "." + jwtPart(t, claims) + ".signature"
}

func jwtPart(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

type fakeAttestor struct {
	token string
	err   error
}

func (f fakeAttestor) Attest(context.Context, Config) (string, error) {
	return f.token, f.err
}

type fakeExchanger struct {
	token string
	err   error
}

func (f fakeExchanger) Exchange(context.Context, Config, string) (string, error) {
	return f.token, f.err
}

type fakeSecretReader struct {
	mnemonic string
	err      error
}

func (f fakeSecretReader) ReadMnemonic(context.Context, Config, string) (string, error) {
	return f.mnemonic, f.err
}

type fakeExecutor struct {
	called bool
	binary string
	args   []string
	env    []string
}

func (f *fakeExecutor) Exec(binary string, args []string, env []string) error {
	f.called = true
	f.binary = binary
	f.args = append([]string(nil), args...)
	f.env = append([]string(nil), env...)
	return nil
}
