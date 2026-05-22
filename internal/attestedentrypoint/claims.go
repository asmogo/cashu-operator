package attestedentrypoint

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type tokenClaims struct {
	Audience              any      `json:"aud"`
	ExpiresAt             int64    `json:"exp"`
	HardwareModel         string   `json:"hwmodel"`
	Issuer                string   `json:"iss"`
	NotBefore             int64    `json:"nbf"`
	SecureBoot            bool     `json:"secboot"`
	GoogleServiceAccounts []string `json:"google_service_accounts"`
	Submods               struct {
		GCE struct {
			ProjectID string `json:"project_id"`
			Zone      string `json:"zone"`
		} `json:"gce"`
	} `json:"submods"`
}

func ValidateAttestationClaims(token string, cfg Config) error {
	var claims tokenClaims
	if err := decodeJWTPayload(token, &claims); err != nil {
		return err
	}
	if claims.Issuer != "https://confidentialcomputing.googleapis.com" {
		return fmt.Errorf("issuer %q does not match Google Cloud Attestation", claims.Issuer)
	}
	if !audienceContains(claims.Audience, cfg.ExpectedAudience) {
		return fmt.Errorf("audience does not contain %q", cfg.ExpectedAudience)
	}
	if claims.HardwareModel != cfg.ExpectedHWModel {
		return fmt.Errorf("hwmodel %q does not match %q", claims.HardwareModel, cfg.ExpectedHWModel)
	}
	if !claims.SecureBoot {
		return fmt.Errorf("secure boot claim is not true")
	}
	if !containsString(claims.GoogleServiceAccounts, cfg.ExpectedServiceAccount) {
		return fmt.Errorf("google_service_accounts does not contain %q", cfg.ExpectedServiceAccount)
	}
	if cfg.ExpectedProjectID != "" && claims.Submods.GCE.ProjectID != cfg.ExpectedProjectID {
		return fmt.Errorf("project_id %q does not match %q", claims.Submods.GCE.ProjectID, cfg.ExpectedProjectID)
	}
	if cfg.ExpectedZone != "" && claims.Submods.GCE.Zone != cfg.ExpectedZone {
		return fmt.Errorf("zone %q does not match %q", claims.Submods.GCE.Zone, cfg.ExpectedZone)
	}
	now := time.Now().Unix()
	if claims.NotBefore != 0 && now < claims.NotBefore {
		return fmt.Errorf("token is not valid yet")
	}
	if claims.ExpiresAt != 0 && now >= claims.ExpiresAt {
		return fmt.Errorf("token is expired")
	}
	return nil
}

func decodeJWTPayload(token string, dst any) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("attestation token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("decode JWT payload: %w", err)
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return fmt.Errorf("parse JWT payload: %w", err)
	}
	return nil
}

func audienceContains(value any, expected string) bool {
	switch aud := value.(type) {
	case string:
		return aud == expected
	case []any:
		for _, item := range aud {
			if s, ok := item.(string); ok && s == expected {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
