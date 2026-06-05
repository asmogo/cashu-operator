package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cdk "github.com/cashubtc/cdk-go/bindings/cdkffi"
)

const (
	providerLocal        = "local"
	providerVaultTransit = "vaultTransit"
	providerGoogleKMS    = "googleKMS"

	cipherAES256GCM = "AES-256-GCM"
)

type seedEnvelope struct {
	Version    int    `json:"version"`
	Cipher     string `json:"cipher"`
	AAD        string `json:"aad,omitempty"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	WrappedDEK string `json:"wrappedDek"`
	CreatedAt  string `json:"createdAt"`
}

type provider interface {
	Wrap(ctx context.Context, plaintext []byte) (string, error)
	Unwrap(ctx context.Context, wrapped string) ([]byte, error)
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("seed-init failed: %v", err)
	}
}

func run(ctx context.Context) error {
	seedPath := getenvDefault("SEED_INIT_SEED_PATH", "/data/seed.enc")
	baseConfigPath := getenvDefault("SEED_INIT_BASE_CONFIG_PATH", "/config/base.toml")
	outputConfigPath := getenvDefault("SEED_INIT_OUTPUT_CONFIG_PATH", "/data/config.toml")
	aad := os.Getenv("SEED_INIT_AAD")
	writeBDKMnemonic := strings.EqualFold(os.Getenv("SEED_INIT_WRITE_BDK_MNEMONIC"), "true")

	provider, err := newProviderFromEnv()
	if err != nil {
		return err
	}

	mnemonic, err := loadOrCreateMnemonic(ctx, seedPath, []byte(aad), provider)
	if err != nil {
		return err
	}
	if err := renderConfig(baseConfigPath, outputConfigPath, mnemonic, writeBDKMnemonic); err != nil {
		return err
	}
	log.Printf("seed material ready; rendered %s", outputConfigPath)
	return nil
}

func newProviderFromEnv() (provider, error) {
	switch os.Getenv("SEED_INIT_PROVIDER") {
	case providerLocal:
		return newLocalProviderFromEnv()
	case providerVaultTransit:
		return newVaultTransitProviderFromEnv()
	case providerGoogleKMS:
		return newGoogleKMSProviderFromEnv()
	case "":
		return nil, errors.New("SEED_INIT_PROVIDER is required")
	default:
		return nil, fmt.Errorf("unsupported SEED_INIT_PROVIDER %q", os.Getenv("SEED_INIT_PROVIDER"))
	}
}

func loadOrCreateMnemonic(ctx context.Context, seedPath string, aad []byte, p provider) (string, error) {
	envelope, err := readEnvelope(seedPath)
	if err == nil {
		return decryptEnvelope(ctx, envelope, aad, p)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	mnemonic, err := generateMnemonic()
	if err != nil {
		return "", err
	}
	envelope, err = encryptMnemonic(ctx, mnemonic, aad, p)
	if err != nil {
		return "", err
	}
	if err := writeEnvelope(seedPath, envelope); err != nil {
		return "", err
	}
	log.Printf("created encrypted seed envelope at %s", seedPath)
	return mnemonic, nil
}

func generateMnemonic() (string, error) {
	mnemonic, err := cdk.GenerateMnemonic()
	if err != nil {
		return "", fmt.Errorf("generate mnemonic with cdk-go: %w", err)
	}
	return mnemonic, nil
}

func encryptMnemonic(ctx context.Context, mnemonic string, aad []byte, p provider) (*seedEnvelope, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(crand.Reader, dek); err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(crand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	wrappedDEK, err := p.Wrap(ctx, dek)
	if err != nil {
		return nil, fmt.Errorf("wrap DEK: %w", err)
	}
	return &seedEnvelope{
		Version:    1,
		Cipher:     cipherAES256GCM,
		AAD:        string(aad),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(gcm.Seal(nil, nonce, []byte(mnemonic), aad)),
		WrappedDEK: wrappedDEK,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func decryptEnvelope(ctx context.Context, envelope *seedEnvelope, aad []byte, p provider) (string, error) {
	if envelope.Version != 1 {
		return "", fmt.Errorf("unsupported seed envelope version %d", envelope.Version)
	}
	if envelope.Cipher != cipherAES256GCM {
		return "", fmt.Errorf("unsupported seed envelope cipher %q", envelope.Cipher)
	}
	if envelope.AAD != "" && envelope.AAD != string(aad) {
		return "", fmt.Errorf("seed envelope AAD %q does not match current AAD %q", envelope.AAD, string(aad))
	}
	dek, err := p.Unwrap(ctx, envelope.WrappedDEK)
	if err != nil {
		return "", fmt.Errorf("unwrap DEK: %w", err)
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", fmt.Errorf("decrypt seed: %w", err)
	}
	return string(plaintext), nil
}

func readEnvelope(path string) (*seedEnvelope, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var envelope seedEnvelope
	if err := json.Unmarshal(b, &envelope); err != nil {
		return nil, fmt.Errorf("decode seed envelope %s: %w", path, err)
	}
	return &envelope, nil
}

func writeEnvelope(path string, envelope *seedEnvelope) error {
	b, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("encode seed envelope: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create seed dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write seed envelope: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("commit seed envelope: %w", err)
	}
	return nil
}

func renderConfig(basePath, outputPath, mnemonic string, writeBDKMnemonic bool) error {
	b, err := os.ReadFile(basePath)
	if err != nil {
		return fmt.Errorf("read base config: %w", err)
	}
	config := string(b)
	config = setTOMLString(config, "info", "mnemonic", mnemonic)
	if writeBDKMnemonic {
		config = setTOMLString(config, "bdk", "mnemonic", mnemonic)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp := outputPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, outputPath); err != nil {
		return fmt.Errorf("commit config: %w", err)
	}
	return nil
}

func setTOMLString(config, section, key, value string) string {
	lineValue := key + " = " + strconv.Quote(value)
	lines := strings.Split(config, "\n")
	sectionHeader := "[" + section + "]"
	sectionStart := -1
	sectionEnd := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == sectionHeader {
			sectionStart = i
			continue
		}
		if sectionStart >= 0 && i > sectionStart && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sectionEnd = i
			break
		}
	}
	if sectionStart == -1 {
		if !strings.HasSuffix(config, "\n") {
			config += "\n"
		}
		return config + "\n" + sectionHeader + "\n" + lineValue + "\n"
	}
	for i := sectionStart + 1; i < sectionEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = lineValue
			return strings.Join(lines, "\n")
		}
	}
	updated := append([]string{}, lines[:sectionEnd]...)
	updated = append(updated, lineValue)
	updated = append(updated, lines[sectionEnd:]...)
	return strings.Join(updated, "\n")
}

type localProvider struct {
	key []byte
}

func newLocalProviderFromEnv() (*localProvider, error) {
	encoded := os.Getenv("SEED_INIT_LOCAL_KEY_B64")
	if encoded == "" {
		return nil, errors.New("SEED_INIT_LOCAL_KEY_B64 is required for local provider")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode local wrapping key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("local wrapping key must decode to 32 bytes, got %d", len(key))
	}
	return &localProvider{key: key}, nil
}

func (p *localProvider) Wrap(_ context.Context, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(crand.Reader, nonce); err != nil {
		return "", err
	}
	wrapped := append(nonce, gcm.Seal(nil, nonce, plaintext, nil)...)
	return "local:v1:" + base64.StdEncoding.EncodeToString(wrapped), nil
}

func (p *localProvider) Unwrap(_ context.Context, wrapped string) ([]byte, error) {
	encoded := strings.TrimPrefix(wrapped, "local:v1:")
	if encoded == wrapped {
		return nil, errors.New("local wrapped DEK has invalid prefix")
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(b) < gcm.NonceSize() {
		return nil, errors.New("local wrapped DEK is too short")
	}
	nonce := b[:gcm.NonceSize()]
	ciphertext := b[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

type vaultTransitProvider struct {
	address string
	mount   string
	keyName string
	token   string
	client  *http.Client
}

type googleKMSProvider struct {
	endpoint string
	keyName  string
	client   *http.Client
}

func newGoogleKMSProviderFromEnv() (*googleKMSProvider, error) {
	keyName := os.Getenv("GOOGLE_KMS_KEY_NAME")
	if keyName == "" {
		return nil, errors.New("GOOGLE_KMS_KEY_NAME is required")
	}
	return &googleKMSProvider{
		endpoint: strings.TrimRight(getenvDefault("GOOGLE_KMS_ENDPOINT", "https://cloudkms.googleapis.com"), "/"),
		keyName:  strings.Trim(keyName, "/"),
		client:   &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (p *googleKMSProvider) Wrap(ctx context.Context, plaintext []byte) (string, error) {
	body := map[string]string{"plaintext": base64.StdEncoding.EncodeToString(plaintext)}
	var response struct {
		Ciphertext string `json:"ciphertext"`
	}
	if err := p.doJSON(ctx, http.MethodPost, p.kmsPath("encrypt"), body, &response); err != nil {
		return "", err
	}
	if response.Ciphertext == "" {
		return "", errors.New("Google KMS encrypt returned empty ciphertext")
	}
	return "googlekms:v1:" + response.Ciphertext, nil
}

func (p *googleKMSProvider) Unwrap(ctx context.Context, wrapped string) ([]byte, error) {
	ciphertext := strings.TrimPrefix(wrapped, "googlekms:v1:")
	if ciphertext == wrapped {
		return nil, errors.New("Google KMS wrapped DEK has invalid prefix")
	}
	body := map[string]string{"ciphertext": ciphertext}
	var response struct {
		Plaintext string `json:"plaintext"`
	}
	if err := p.doJSON(ctx, http.MethodPost, p.kmsPath("decrypt"), body, &response); err != nil {
		return nil, err
	}
	if response.Plaintext == "" {
		return nil, errors.New("Google KMS decrypt returned empty plaintext")
	}
	return base64.StdEncoding.DecodeString(response.Plaintext)
}

func (p *googleKMSProvider) kmsPath(action string) string {
	return "/v1/" + p.keyName + ":" + action
}

func (p *googleKMSProvider) doJSON(ctx context.Context, method, path string, body any, out any) error {
	token, err := googleAccessToken(ctx, p.client)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.endpoint+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Google KMS request %s %s failed: status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return err
		}
	}
	return nil
}

func googleAccessToken(ctx context.Context, client *http.Client) (string, error) {
	if token := os.Getenv("GOOGLE_OAUTH_ACCESS_TOKEN"); token != "" {
		return token, nil
	}
	metadataHost := getenvDefault("GCE_METADATA_HOST", "metadata.google.internal")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+metadataHost+"/computeMetadata/v1/instance/service-accounts/default/token", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch Google metadata token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch Google metadata token failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if response.AccessToken == "" {
		return "", errors.New("Google metadata token response missing access_token")
	}
	return response.AccessToken, nil
}

func newVaultTransitProviderFromEnv() (*vaultTransitProvider, error) {
	address := os.Getenv("VAULT_ADDR")
	if address == "" {
		return nil, errors.New("VAULT_ADDR is required")
	}
	mount := getenvDefault("VAULT_TRANSIT_MOUNT", "transit")
	keyName := os.Getenv("VAULT_TRANSIT_KEY")
	if keyName == "" {
		return nil, errors.New("VAULT_TRANSIT_KEY is required")
	}
	p := &vaultTransitProvider{
		address: strings.TrimRight(address, "/"),
		mount:   strings.Trim(mount, "/"),
		keyName: keyName,
		client:  &http.Client{Timeout: 20 * time.Second},
	}
	token, err := p.authenticate(context.Background())
	if err != nil {
		return nil, err
	}
	p.token = token
	return p, nil
}

func (p *vaultTransitProvider) authenticate(ctx context.Context) (string, error) {
	switch os.Getenv("VAULT_AUTH_METHOD") {
	case "token":
		token := os.Getenv("VAULT_TOKEN")
		if token == "" {
			return "", errors.New("VAULT_TOKEN is required for token auth")
		}
		return token, nil
	case "kubernetes":
		mount := strings.Trim(getenvDefault("VAULT_K8S_AUTH_MOUNT", "kubernetes"), "/")
		role := os.Getenv("VAULT_K8S_AUTH_ROLE")
		if role == "" {
			return "", errors.New("VAULT_K8S_AUTH_ROLE is required for kubernetes auth")
		}
		jwtPath := getenvDefault("VAULT_K8S_JWT_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token")
		jwt, err := os.ReadFile(jwtPath)
		if err != nil {
			return "", fmt.Errorf("read Kubernetes JWT: %w", err)
		}
		body := map[string]string{"role": role, "jwt": strings.TrimSpace(string(jwt))}
		var response struct {
			Auth struct {
				ClientToken string `json:"client_token"`
			} `json:"auth"`
		}
		if err := p.doJSON(ctx, http.MethodPost, "/v1/auth/"+mount+"/login", "", body, &response); err != nil {
			return "", err
		}
		if response.Auth.ClientToken == "" {
			return "", errors.New("Vault Kubernetes auth returned empty client token")
		}
		return response.Auth.ClientToken, nil
	default:
		return "", errors.New("VAULT_AUTH_METHOD must be token or kubernetes")
	}
}

func (p *vaultTransitProvider) Wrap(ctx context.Context, plaintext []byte) (string, error) {
	body := map[string]string{"plaintext": base64.StdEncoding.EncodeToString(plaintext)}
	var response struct {
		Data struct {
			Ciphertext string `json:"ciphertext"`
		} `json:"data"`
	}
	if err := p.doJSON(ctx, http.MethodPost, p.transitPath("encrypt"), p.token, body, &response); err != nil {
		return "", err
	}
	if response.Data.Ciphertext == "" {
		return "", errors.New("Vault transit encrypt returned empty ciphertext")
	}
	return response.Data.Ciphertext, nil
}

func (p *vaultTransitProvider) Unwrap(ctx context.Context, wrapped string) ([]byte, error) {
	body := map[string]string{"ciphertext": wrapped}
	var response struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}
	if err := p.doJSON(ctx, http.MethodPost, p.transitPath("decrypt"), p.token, body, &response); err != nil {
		return nil, err
	}
	if response.Data.Plaintext == "" {
		return nil, errors.New("Vault transit decrypt returned empty plaintext")
	}
	return base64.StdEncoding.DecodeString(response.Data.Plaintext)
}

func (p *vaultTransitProvider) transitPath(action string) string {
	return "/v1/" + p.mount + "/" + action + "/" + url.PathEscape(p.keyName)
}

func (p *vaultTransitProvider) doJSON(ctx context.Context, method, path, token string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.address+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Vault request %s %s failed: status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return err
		}
	}
	return nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
