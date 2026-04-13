package generators

import (
	"bytes"
	"testing"

	corev1 "k8s.io/api/core/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestWriteDatabaseSection_IncludesAuthDatabaseConfig(t *testing.T) {
	mint := baseMint("db-rich")
	maxConnections := int32(7)
	connectionTimeout := int32(11)
	authMaxConnections := int32(3)
	authConnectionTimeout := int32(5)
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine: mintv1alpha1.DatabaseEnginePostgres,
		Postgres: &mintv1alpha1.PostgresConfig{
			AutoProvision:            true,
			MaxConnections:           &maxConnections,
			ConnectionTimeoutSeconds: &connectionTimeout,
		},
	}
	mint.Spec.Auth = &mintv1alpha1.AuthConfig{
		Enabled: true,
		Database: &mintv1alpha1.AuthDatabaseConfig{
			Postgres: &mintv1alpha1.PostgresConfig{
				MaxConnections:           &authMaxConnections,
				ConnectionTimeoutSeconds: &authConnectionTimeout,
			},
		},
	}

	var buf bytes.Buffer
	writeDatabaseSection(&buf, mint, "secret-password")
	config := buf.String()

	assertContains(t, config, `url = "postgresql://cdk:secret-password@db-rich-postgres:5432/cdk_mintd?sslmode=disable"`)
	assertContains(t, config, `tls_mode = "disable"`)
	assertContains(t, config, "[auth_database.postgres]")
	assertContains(t, config, `url = ""`)
	assertContains(t, config, "max_connections = 3")
	assertContains(t, config, "connection_timeout_seconds = 5")
}

func TestWritePaymentBackendSection_CoversOptionalFields(t *testing.T) {
	t.Run(lndStr, func(t *testing.T) {
		mint := baseMint("lnd-rich")
		feePercent := 0.05
		reserveFeeMin := int32(10)
		minMelt := int64(5)
		maxMelt := int64(50)
		mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
			MinMelt: &minMelt,
			MaxMelt: &maxMelt,
			LND: &mintv1alpha1.LNDConfig{
				Address:           "https://lnd:10009",
				MacaroonSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnd-secret"}, Key: "macaroon"},
				CertSecretRef:     &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnd-secret"}, Key: "cert"},
				FeePercent:        &feePercent,
				ReserveFeeMin:     &reserveFeeMin,
			},
		}

		var buf bytes.Buffer
		writePaymentBackendSection(&buf, mint)
		config := buf.String()
		assertContains(t, config, `ln_backend = lndStr`)
		assertContains(t, config, "min_melt = 5")
		assertContains(t, config, "max_melt = 50")
		assertContains(t, config, `macaroon_file = "/secrets/lnd/macaroon"`)
		assertContains(t, config, `cert_file = "/secrets/lnd/cert"`)
		assertContains(t, config, "fee_percent = 0.050000")
		assertContains(t, config, "reserve_fee_min = 10")
	})

	t.Run("cln", func(t *testing.T) {
		mint := baseMint("cln-rich")
		feePercent := 0.06
		reserveFeeMin := int32(12)
		mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
			CLN: &mintv1alpha1.CLNConfig{
				RPCPath:       "/rpc/lightning-rpc",
				FeePercent:    &feePercent,
				ReserveFeeMin: &reserveFeeMin,
			},
		}

		var buf bytes.Buffer
		writePaymentBackendSection(&buf, mint)
		config := buf.String()
		assertContains(t, config, `ln_backend = "cln"`)
		assertContains(t, config, "fee_percent = 0.060000")
		assertContains(t, config, "reserve_fee_min = 12")
	})

	t.Run("lnbits", func(t *testing.T) {
		mint := baseMint("lnbits-rich")
		feePercent := 0.03
		reserveFeeMin := int32(6)
		mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
			LNBits: &mintv1alpha1.LNBitsConfig{
				API:                    "https://lnbits.example.com",
				AdminAPIKeySecretRef:   corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnbits"}, Key: "admin"},
				InvoiceAPIKeySecretRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnbits"}, Key: "invoice"},
				FeePercent:             &feePercent,
				ReserveFeeMin:          &reserveFeeMin,
			},
		}

		var buf bytes.Buffer
		writePaymentBackendSection(&buf, mint)
		config := buf.String()
		assertContains(t, config, `ln_backend = "lnbits"`)
		assertContains(t, config, "fee_percent = 0.030000")
		assertContains(t, config, "reserve_fee_min = 6")
	})

	t.Run("fake wallet", func(t *testing.T) {
		mint := baseMint("fake-wallet-rich")
		feePercent := 0.07
		reserveFeeMin := int32(8)
		minDelayTime := int32(2)
		maxDelayTime := int32(9)
		mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
			FakeWallet: &mintv1alpha1.FakeWalletConfig{
				SupportedUnits: []string{"sat", "usd"},
				FeePercent:     &feePercent,
				ReserveFeeMin:  &reserveFeeMin,
				MinDelayTime:   &minDelayTime,
				MaxDelayTime:   &maxDelayTime,
			},
		}

		var buf bytes.Buffer
		writePaymentBackendSection(&buf, mint)
		config := buf.String()
		assertContains(t, config, `ln_backend = "fakewallet"`)
		assertContains(t, config, `supported_units = ["sat", "usd"]`)
		assertContains(t, config, "fee_percent = 0.070000")
		assertContains(t, config, "reserve_fee_min = 8")
		assertContains(t, config, "min_delay_time = 2")
		assertContains(t, config, "max_delay_time = 9")
	})
}

func TestWriteAuthSection_WritesAllConfiguredEndpointLevels(t *testing.T) {
	mint := baseMint("auth-rich")
	mint.Spec.Auth = &mintv1alpha1.AuthConfig{
		Enabled:         true,
		OpenIDDiscovery: "https://issuer.example.com/.well-known/openid-configuration",
		OpenIDClientID:  "cashu-operator",
		MintMaxBat:      int32Ptr(99),
		Mint:            mintv1alpha1.AuthLevelBlind,
		GetMintQuote:    mintv1alpha1.AuthLevelClear,
		CheckMintQuote:  mintv1alpha1.AuthLevelNone,
		Melt:            mintv1alpha1.AuthLevelBlind,
		GetMeltQuote:    mintv1alpha1.AuthLevelClear,
		CheckMeltQuote:  mintv1alpha1.AuthLevelNone,
		Swap:            mintv1alpha1.AuthLevelBlind,
		Restore:         mintv1alpha1.AuthLevelClear,
		CheckProofState: mintv1alpha1.AuthLevelNone,
	}

	var buf bytes.Buffer
	writeAuthSection(&buf, mint)
	config := buf.String()

	assertContains(t, config, "[auth]")
	assertContains(t, config, `openid_discovery = "https://issuer.example.com/.well-known/openid-configuration"`)
	assertContains(t, config, `openid_client_id = "cashu-operator"`)
	assertContains(t, config, "mint_max_bat = 99")
	assertContains(t, config, `mint = "blind"`)
	assertContains(t, config, `get_mint_quote = "clear"`)
	assertContains(t, config, `check_mint_quote = "none"`)
	assertContains(t, config, `melt = "blind"`)
	assertContains(t, config, `get_melt_quote = "clear"`)
	assertContains(t, config, `check_melt_quote = "none"`)
	assertContains(t, config, `swap = "blind"`)
	assertContains(t, config, `restore = "clear"`)
	assertContains(t, config, `check_proof_state = "none"`)
}
