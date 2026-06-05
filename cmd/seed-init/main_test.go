package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedInitLocalProviderCreatesAndReusesEncryptedSeed(t *testing.T) {
	dir := t.TempDir()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	seedPath := filepath.Join(dir, "seed.enc")
	baseConfigPath := filepath.Join(dir, "base.toml")
	outputConfigPath := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(baseConfigPath, []byte("[info]\nurl = \"http://mint.test\"\n\n[bdk]\nnetwork = \"signet\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SEED_INIT_PROVIDER", providerLocal)
	t.Setenv("SEED_INIT_LOCAL_KEY_B64", key)
	t.Setenv("SEED_INIT_SEED_PATH", seedPath)
	t.Setenv("SEED_INIT_BASE_CONFIG_PATH", baseConfigPath)
	t.Setenv("SEED_INIT_OUTPUT_CONFIG_PATH", outputConfigPath)
	t.Setenv("SEED_INIT_AAD", "cashu-tdx-test/tdx-mint")
	t.Setenv("SEED_INIT_WRITE_BDK_MNEMONIC", "true")

	if err := run(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	firstConfig, err := os.ReadFile(outputConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(firstConfig), "mnemonic = ") {
		t.Fatalf("rendered config missing mnemonic: %s", firstConfig)
	}
	if !strings.Contains(string(firstConfig), "[bdk]") {
		t.Fatalf("rendered config missing bdk section: %s", firstConfig)
	}
	seedEnvelope, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	firstMnemonicLine := extractMnemonicLine(t, string(firstConfig))
	if strings.Contains(string(seedEnvelope), strings.TrimPrefix(firstMnemonicLine, "mnemonic = ")) {
		t.Fatal("encrypted seed envelope contains plaintext mnemonic")
	}

	if err := run(context.Background()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	secondConfig, err := os.ReadFile(outputConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := extractMnemonicLine(t, string(secondConfig)); got != firstMnemonicLine {
		t.Fatalf("mnemonic changed across runs: first %q second %q", firstMnemonicLine, got)
	}

	t.Setenv("SEED_INIT_AAD", "other-namespace/tdx-mint")
	if err := run(context.Background()); err == nil {
		t.Fatal("expected AAD mismatch error")
	}
}

func TestSetTOMLStringReplacesExistingKey(t *testing.T) {
	config := "[info]\nurl = \"http://mint.test\"\nmnemonic = \"old\"\n\n[database]\nengine = \"sqlite\"\n"
	updated := setTOMLString(config, "info", "mnemonic", "new value")
	if strings.Count(updated, "mnemonic = ") != 1 {
		t.Fatalf("expected one mnemonic key, got config:\n%s", updated)
	}
	if !strings.Contains(updated, "mnemonic = \"new value\"") {
		t.Fatalf("mnemonic not replaced:\n%s", updated)
	}
}

func TestGoogleKMSProviderWrapUnwrap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization header = %q", got)
		}
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, ":encrypt"):
			plaintext, err := base64.StdEncoding.DecodeString(req["plaintext"])
			if err != nil {
				t.Fatalf("decode plaintext: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"ciphertext": base64.StdEncoding.EncodeToString(append([]byte("wrapped:"), plaintext...)),
			})
		case strings.HasSuffix(r.URL.Path, ":decrypt"):
			ciphertext, err := base64.StdEncoding.DecodeString(req["ciphertext"])
			if err != nil {
				t.Fatalf("decode ciphertext: %v", err)
			}
			plaintext := strings.TrimPrefix(string(ciphertext), "wrapped:")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"plaintext": base64.StdEncoding.EncodeToString([]byte(plaintext)),
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("GOOGLE_OAUTH_ACCESS_TOKEN", "test-token")
	t.Setenv("GOOGLE_KMS_ENDPOINT", server.URL)
	t.Setenv("GOOGLE_KMS_KEY_NAME", "projects/p/locations/l/keyRings/r/cryptoKeys/k")
	provider, err := newGoogleKMSProviderFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := provider.Wrap(context.Background(), []byte("dek"))
	if err != nil {
		t.Fatalf("Wrap() error = %v", err)
	}
	if !strings.HasPrefix(wrapped, "googlekms:v1:") {
		t.Fatalf("wrapped value has unexpected prefix: %q", wrapped)
	}
	plaintext, err := provider.Unwrap(context.Background(), wrapped)
	if err != nil {
		t.Fatalf("Unwrap() error = %v", err)
	}
	if string(plaintext) != "dek" {
		t.Fatalf("plaintext = %q, want dek", plaintext)
	}
}

func extractMnemonicLine(t *testing.T, config string) string {
	t.Helper()
	for _, line := range strings.Split(config, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "mnemonic = ") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatal("mnemonic line not found")
	return ""
}
