package attestedentrypoint

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"

	confidentialcomputing "cloud.google.com/go/confidentialcomputing/apiv1"
	confidentialcomputingpb "cloud.google.com/go/confidentialcomputing/apiv1/confidentialcomputingpb"
	tpmclient "github.com/google/go-tpm-tools/client"
	attestpb "github.com/google/go-tpm-tools/proto/attest"
	"github.com/google/go-tpm/legacy/tpm2"
)

type GoogleGKEAttestor struct {
	HTTPClient *http.Client
}

func (a GoogleGKEAttestor) Attest(ctx context.Context, cfg Config) (string, error) {
	if _, err := os.Stat(cfg.TPMPath); err != nil {
		return "", fmt.Errorf("vTPM device %s unavailable: %w", cfg.TPMPath, err)
	}

	cc, err := confidentialcomputing.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("create Confidential Computing client: %w", err)
	}
	defer cc.Close()

	parent := fmt.Sprintf("projects/%s/locations/%s", cfg.AttestationProjectID, cfg.AttestationLocation)
	challenge, err := cc.CreateChallenge(ctx, &confidentialcomputingpb.CreateChallengeRequest{Parent: parent})
	if err != nil {
		return "", fmt.Errorf("create challenge: %w", err)
	}
	if challenge.GetName() == "" || challenge.GetTpmNonce() == "" {
		return "", fmt.Errorf("challenge response did not include a name and TPM nonce")
	}

	tpm, err := tpm2.OpenTPM(cfg.TPMPath)
	if err != nil {
		return "", fmt.Errorf("open vTPM: %w", err)
	}
	defer tpm.Close()

	ak, err := tpmclient.GceAttestationKeyECC(tpm)
	if err != nil {
		return "", fmt.Errorf("load GCE attestation key: %w", err)
	}
	defer ak.Close()

	fetcher := a.HTTPClient
	if fetcher == nil {
		fetcher = http.DefaultClient
	}
	attestation, err := ak.Attest(tpmclient.AttestOpts{
		Nonce:            []byte(challenge.GetTpmNonce()),
		CertChainFetcher: fetcher,
	})
	if err != nil {
		return "", fmt.Errorf("quote vTPM: %w", err)
	}

	resp, err := cc.VerifyConfidentialGke(ctx, &confidentialcomputingpb.VerifyConfidentialGkeRequest{
		Challenge: challenge.GetName(),
		TeeAttestation: &confidentialcomputingpb.VerifyConfidentialGkeRequest_TpmAttestation{
			TpmAttestation: convertTPMAttestation(attestation),
		},
	})
	if err != nil {
		return "", fmt.Errorf("verify Confidential GKE attestation: %w", err)
	}
	if resp.GetAttestationToken() == "" {
		return "", fmt.Errorf("attestation verification returned an empty token")
	}
	return resp.GetAttestationToken(), nil
}

func convertTPMAttestation(in *attestpb.Attestation) *confidentialcomputingpb.TpmAttestation {
	out := &confidentialcomputingpb.TpmAttestation{
		TcgEventLog:       in.GetEventLog(),
		CanonicalEventLog: in.GetCanonicalEventLog(),
		AkCert:            in.GetAkCert(),
		CertChain:         in.GetIntermediateCerts(),
	}
	for _, quote := range in.GetQuotes() {
		pcrValues := map[int32][]byte{}
		var hashAlgo int32
		if quote.GetPcrs() != nil {
			hashAlgo = int32(quote.GetPcrs().GetHash())
			for index, value := range quote.GetPcrs().GetPcrs() {
				pcrValues[int32(index)] = value
			}
		}
		out.Quotes = append(out.Quotes, &confidentialcomputingpb.TpmAttestation_Quote{
			HashAlgo:     hashAlgo,
			PcrValues:    pcrValues,
			RawQuote:     quote.GetQuote(),
			RawSignature: quote.GetRawSig(),
		})
	}
	return out
}

type GoogleSTSExchanger struct {
	HTTPClient *http.Client
}

func (e GoogleSTSExchanger) Exchange(ctx context.Context, cfg Config, attestationToken string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Set("audience", cfg.WorkloadIdentityProvider)
	form.Set("scope", "https://www.googleapis.com/auth/cloud-platform")
	form.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")
	form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	form.Set("subject_token", attestationToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.STSEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := doJSON(e.HTTPClient, req, &response); err != nil {
		return "", err
	}
	if response.AccessToken == "" {
		return "", fmt.Errorf("STS response did not include access_token")
	}
	return response.AccessToken, nil
}

type GoogleSecretManagerReader struct {
	HTTPClient *http.Client
}

func (r GoogleSecretManagerReader) ReadMnemonic(ctx context.Context, cfg Config, accessToken string) (string, error) {
	secretURL := fmt.Sprintf("%s/v1/%s:access", cfg.SecretManagerEndpoint, cfg.MnemonicSecretVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, secretURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	var response struct {
		Payload struct {
			Data string `json:"data"`
		} `json:"payload"`
	}
	if err := doJSON(r.HTTPClient, req, &response); err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(response.Payload.Data)
	if err != nil {
		return "", fmt.Errorf("decode Secret Manager payload: %w", err)
	}
	return string(decoded), nil
}

type SyscallExecutor struct{}

func (SyscallExecutor) Exec(binary string, args []string, env []string) error {
	path, err := resolveBinary(binary)
	if err != nil {
		return err
	}
	return syscall.Exec(path, args, env)
}

func resolveBinary(binary string) (string, error) {
	if strings.Contains(binary, "/") {
		return binary, nil
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("find %s in PATH: %w", binary, err)
	}
	return path, nil
}

func doJSON(client *http.Client, req *http.Request, dst any) error {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s returned %s: %s", req.URL.Host, resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(dst); err != nil {
		return fmt.Errorf("decode %s response: %w", req.URL.Host, err)
	}
	return nil
}
