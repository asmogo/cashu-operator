/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func int32Ptr(i int32) *int32       { return &i }
func float64Ptr(f float64) *float64 { return &f }

func validMint() *CashuMint {
	return &CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mint", Namespace: "default"},
		Spec: CashuMintSpec{
			MintInfo: MintInfo{URL: "http://test.local"},
			Database: DatabaseConfig{Engine: DatabaseEnginePostgres, Postgres: &PostgresConfig{AutoProvision: true}},
			PaymentBackend: PaymentBackendConfig{
				FakeWallet: &FakeWalletConfig{},
			},
		},
	}
}

// --- Defaulting tests ---

func TestDefault_MintInfo(t *testing.T) {
	m := validMint()
	m.Spec.MintInfo.ListenHost = ""
	m.Spec.MintInfo.ListenPort = 0
	m.Spec.Image = ""
	m.Spec.Replicas = nil

	m.Default()

	if m.Spec.MintInfo.ListenHost != DefaultListenHost {
		t.Errorf("listenHost = %q, want %q", m.Spec.MintInfo.ListenHost, DefaultListenHost)
	}
	if m.Spec.MintInfo.ListenPort != 8085 {
		t.Errorf("listenPort = %d, want 8085", m.Spec.MintInfo.ListenPort)
	}
	if m.Spec.Image == "" {
		t.Error("image should be defaulted")
	}
	if m.Spec.Replicas == nil || *m.Spec.Replicas != 1 {
		t.Error("replicas should default to 1")
	}
}

func TestDefault_Database(t *testing.T) {
	t.Run("empty engine defaults to postgres", func(t *testing.T) {
		m := validMint()
		m.Spec.Database.Engine = ""
		m.Default()
		if m.Spec.Database.Engine != DatabaseEnginePostgres {
			t.Errorf("engine = %q, want %q", m.Spec.Database.Engine, DatabaseEnginePostgres)
		}
	})

	t.Run("postgres autoprovision defaults", func(t *testing.T) {
		m := validMint()
		m.Spec.Database.Postgres.AutoProvision = true
		m.Spec.Database.Postgres.AutoProvisionSpec = &PostgresAutoProvisionSpec{}
		m.Default()
		if m.Spec.Database.Postgres.AutoProvisionSpec.StorageSize != "10Gi" {
			t.Error("storageSize should default to 10Gi")
		}
		if m.Spec.Database.Postgres.AutoProvisionSpec.Version != "15" {
			t.Error("version should default to 15")
		}
	})

	t.Run("postgres tls mode defaults", func(t *testing.T) {
		m := validMint()
		m.Spec.Database.Postgres.AutoProvision = true
		m.Spec.Database.Postgres.TLSMode = ""
		m.Default()
		if m.Spec.Database.Postgres.TLSMode != "disable" {
			t.Errorf("tls mode for autoprov = %q, want disable", m.Spec.Database.Postgres.TLSMode)
		}
	})

	t.Run("postgres tls mode non-autoprov", func(t *testing.T) {
		m := validMint()
		m.Spec.Database.Postgres.AutoProvision = false
		m.Spec.Database.Postgres.URL = "postgresql://u:p@h/d"
		m.Spec.Database.Postgres.TLSMode = ""
		m.Default()
		if m.Spec.Database.Postgres.TLSMode != "require" {
			t.Errorf("tls mode for external = %q, want require", m.Spec.Database.Postgres.TLSMode)
		}
	})

	t.Run("sqlite defaults", func(t *testing.T) {
		m := validMint()
		m.Spec.Database.Engine = DatabaseEngineSQLite
		m.Spec.Database.Postgres = nil
		m.Spec.Database.SQLite = &SQLiteConfig{}
		m.Default()
		if m.Spec.Database.SQLite.DataDir != "/data" {
			t.Errorf("dataDir = %q, want /data", m.Spec.Database.SQLite.DataDir)
		}
	})
}

func TestDefault_PaymentBackend(t *testing.T) {
	t.Run("LND defaults", func(t *testing.T) {
		m := validMint()
		m.Spec.PaymentBackend = PaymentBackendConfig{
			LND: &LNDConfig{Address: "https://lnd:10009"},
		}
		m.Default()
		if m.Spec.PaymentBackend.LND.FeePercent == nil {
			t.Error("feePercent should be defaulted")
		}
		if m.Spec.PaymentBackend.LND.ReserveFeeMin == nil {
			t.Error("reserveFeeMin should be defaulted")
		}
	})

	t.Run("CLN defaults", func(t *testing.T) {
		m := validMint()
		m.Spec.PaymentBackend = PaymentBackendConfig{
			CLN: &CLNConfig{RPCPath: "/rpc"},
		}
		m.Default()
		if m.Spec.PaymentBackend.CLN.FeePercent == nil {
			t.Error("feePercent should be defaulted")
		}
	})

	t.Run("FakeWallet defaults", func(t *testing.T) {
		m := validMint()
		m.Spec.PaymentBackend.FakeWallet.SupportedUnits = nil
		m.Default()
		if len(m.Spec.PaymentBackend.FakeWallet.SupportedUnits) == 0 {
			t.Error("supportedUnits should be defaulted to [sat]")
		}
	})

	t.Run("GRPCProcessor defaults", func(t *testing.T) {
		m := validMint()
		m.Spec.PaymentBackend = PaymentBackendConfig{
			GRPCProcessor: &GRPCProcessorConfig{Address: "localhost"},
		}
		m.Default()
		if m.Spec.PaymentBackend.GRPCProcessor.Port != 50051 {
			t.Errorf("port = %d, want 50051", m.Spec.PaymentBackend.GRPCProcessor.Port)
		}
	})
}

func TestDefault_Ingress(t *testing.T) {
	m := validMint()
	m.Spec.Ingress = &IngressConfig{Enabled: true, Host: "mint.local"}
	m.Default()
	if m.Spec.Ingress.ClassName != "nginx" {
		t.Errorf("className = %q, want nginx", m.Spec.Ingress.ClassName)
	}
}

func TestDefault_Logging(t *testing.T) {
	m := validMint()
	m.Spec.Logging = &LoggingConfig{}
	m.Default()
	if m.Spec.Logging.Level != "info" {
		t.Errorf("level = %q, want info", m.Spec.Logging.Level)
	}
	if m.Spec.Logging.Format != "json" {
		t.Errorf("format = %q, want json", m.Spec.Logging.Format)
	}
}

func TestDefault_Auth(t *testing.T) {
	m := validMint()
	m.Spec.Auth = &AuthConfig{Enabled: true}
	m.Default()
	if m.Spec.Auth.MintMaxBat == nil || *m.Spec.Auth.MintMaxBat != 50 {
		t.Error("mintMaxBat should default to 50")
	}
	if m.Spec.Auth.Mint != AuthLevelClear {
		t.Errorf("mint = %q, want clear", m.Spec.Auth.Mint)
	}
	if m.Spec.Auth.Swap != AuthLevelClear {
		t.Errorf("swap = %q, want clear", m.Spec.Auth.Swap)
	}
}

func TestDefault_Backup(t *testing.T) {
	m := validMint()
	m.Spec.Backup = &BackupConfig{Enabled: true}
	m.Default()
	if m.Spec.Backup.Schedule != "0 */6 * * *" {
		t.Errorf("schedule = %q, want default cron", m.Spec.Backup.Schedule)
	}
	if m.Spec.Backup.RetentionCount == nil || *m.Spec.Backup.RetentionCount != 14 {
		t.Error("retentionCount should default to 14")
	}
}

func TestDefault_LDKNode(t *testing.T) {
	m := validMint()
	m.Spec.LDKNode = &LDKNodeConfig{Enabled: true}
	m.Default()
	if m.Spec.LDKNode.BitcoinNetwork != "signet" {
		t.Errorf("bitcoinNetwork = %q, want signet", m.Spec.LDKNode.BitcoinNetwork)
	}
	if m.Spec.LDKNode.Port != 8090 {
		t.Errorf("port = %d, want 8090", m.Spec.LDKNode.Port)
	}
}

func TestDefault_Prometheus(t *testing.T) {
	m := validMint()
	m.Spec.Prometheus = &PrometheusConfig{Enabled: true}
	m.Default()
	if m.Spec.Prometheus.Address != DefaultListenHost {
		t.Errorf("address = %q, want %q", m.Spec.Prometheus.Address, DefaultListenHost)
	}
	if m.Spec.Prometheus.Port == nil || *m.Spec.Prometheus.Port != 9090 {
		t.Error("port should default to 9090")
	}
}

func TestDefault_QuoteTTL(t *testing.T) {
	m := validMint()
	m.Spec.MintInfo.QuoteTTL = &QuoteTTLConfig{}
	m.Default()
	if m.Spec.MintInfo.QuoteTTL.MintTTL == nil || *m.Spec.MintInfo.QuoteTTL.MintTTL != 600 {
		t.Error("mintTTL should default to 600")
	}
	if m.Spec.MintInfo.QuoteTTL.MeltTTL == nil || *m.Spec.MintInfo.QuoteTTL.MeltTTL != 120 {
		t.Error("meltTTL should default to 120")
	}
}

func TestDefault_HTTPCache(t *testing.T) {
	m := validMint()
	m.Spec.HTTPCache = &HTTPCacheConfig{}
	m.Default()
	if m.Spec.HTTPCache.Backend != "memory" {
		t.Errorf("backend = %q, want memory", m.Spec.HTTPCache.Backend)
	}
}

func TestDefault_ManagementRPC(t *testing.T) {
	m := validMint()
	m.Spec.ManagementRPC = &ManagementRPCConfig{Enabled: true}
	m.Default()
	if m.Spec.ManagementRPC.Address != "127.0.0.1" {
		t.Errorf("address = %q, want 127.0.0.1", m.Spec.ManagementRPC.Address)
	}
	if m.Spec.ManagementRPC.Port != 8086 {
		t.Errorf("port = %d, want 8086", m.Spec.ManagementRPC.Port)
	}
}

// --- Validation tests ---

func TestValidateCreate_Valid(t *testing.T) {
	m := validMint()
	_, err := m.ValidateCreate()
	if err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateCreate_MissingURL(t *testing.T) {
	m := validMint()
	m.Spec.MintInfo.URL = ""
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestValidateCreate_InvalidDBEngine(t *testing.T) {
	m := validMint()
	m.Spec.Database.Engine = "invalid"
	m.Spec.Database.Postgres = nil
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for invalid DB engine")
	}
}

func TestValidateCreate_NoPaymentBackend(t *testing.T) {
	m := validMint()
	m.Spec.PaymentBackend = PaymentBackendConfig{}
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for no payment backend")
	}
}

func TestValidateCreate_MultipleBackends(t *testing.T) {
	m := validMint()
	m.Spec.PaymentBackend = PaymentBackendConfig{
		FakeWallet: &FakeWalletConfig{},
		LND:        &LNDConfig{Address: "https://lnd:10009"},
	}
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for multiple backends")
	}
}

func TestValidateCreate_LNDMissingAddress(t *testing.T) {
	m := validMint()
	m.Spec.PaymentBackend = PaymentBackendConfig{LND: &LNDConfig{}}
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for LND missing address")
	}
}

func TestValidateCreate_CLNMissingRPCPath(t *testing.T) {
	m := validMint()
	m.Spec.PaymentBackend = PaymentBackendConfig{CLN: &CLNConfig{}}
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for CLN missing rpcPath")
	}
}

func TestValidateCreate_LNBitsMissingAPI(t *testing.T) {
	m := validMint()
	m.Spec.PaymentBackend = PaymentBackendConfig{LNBits: &LNBitsConfig{}}
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for LNBits missing API")
	}
}

func TestValidateCreate_IngressMissingHost(t *testing.T) {
	m := validMint()
	m.Spec.Ingress = &IngressConfig{Enabled: true, Host: ""}
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for ingress missing host")
	}
}

func TestValidateCreate_CertManagerMissingIssuer(t *testing.T) {
	m := validMint()
	m.Spec.Ingress = &IngressConfig{
		Enabled: true, Host: "mint.local",
		TLS: &IngressTLSConfig{
			Enabled:     true,
			CertManager: &CertManagerConfig{Enabled: true, IssuerName: ""},
		},
	}
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for cert-manager missing issuer")
	}
}

func TestValidateCreate_ResourceRequestsExceedLimits(t *testing.T) {
	m := validMint()
	m.Spec.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
	}
	_, err := m.ValidateCreate()
	if err == nil {
		t.Error("expected error for requests > limits")
	}
}

func TestValidateCreate_PostgresValidation(t *testing.T) {
	t.Run("missing postgres config", func(t *testing.T) {
		m := validMint()
		m.Spec.Database = DatabaseConfig{Engine: DatabaseEnginePostgres, Postgres: nil}
		_, err := m.ValidateCreate()
		if err == nil {
			t.Error("expected error for missing postgres config")
		}
	})

	t.Run("no url, no secret, no autoprov", func(t *testing.T) {
		m := validMint()
		m.Spec.Database.Postgres = &PostgresConfig{}
		_, err := m.ValidateCreate()
		if err == nil {
			t.Error("expected error for no source")
		}
	})

	t.Run("both url and secretRef", func(t *testing.T) {
		m := validMint()
		m.Spec.Database.Postgres = &PostgresConfig{
			URL: "postgresql://u:p@h/d",
			URLSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "s"},
				Key:                  "k",
			},
		}
		_, err := m.ValidateCreate()
		if err == nil {
			t.Error("expected error for both url and secretRef")
		}
	})
}

func TestValidateCreate_BackupValidation(t *testing.T) {
	t.Run("non-postgres engine", func(t *testing.T) {
		m := validMint()
		m.Spec.Database = DatabaseConfig{Engine: DatabaseEngineSQLite}
		m.Spec.Backup = &BackupConfig{
			Enabled:  true,
			Schedule: "0 * * * *",
			S3: &S3BackupConfig{
				Bucket:                   "b",
				AccessKeyIDSecretRef:     corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "s"}, Key: "k"},
				SecretAccessKeySecretRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "s"}, Key: "k"},
			},
		}
		_, err := m.ValidateCreate()
		if err == nil {
			t.Error("expected error for backup with non-postgres engine")
		}
	})

	t.Run("missing S3", func(t *testing.T) {
		m := validMint()
		m.Spec.Backup = &BackupConfig{Enabled: true, Schedule: "0 * * * *"}
		_, err := m.ValidateCreate()
		if err == nil {
			t.Error("expected error for missing S3")
		}
	})
}

func TestValidateUpdate(t *testing.T) {
	m := validMint()
	_, err := m.ValidateUpdate(nil)
	if err != nil {
		t.Errorf("expected valid update, got error: %v", err)
	}
}

func TestValidateDelete(t *testing.T) {
	m := validMint()
	_, err := m.ValidateDelete()
	if err != nil {
		t.Error("expected no error on delete validation")
	}
}

func TestValidateCreate_GRPCProcessorValidation(t *testing.T) {
	t.Run("no address without sidecar", func(t *testing.T) {
		m := validMint()
		m.Spec.PaymentBackend = PaymentBackendConfig{
			GRPCProcessor: &GRPCProcessorConfig{},
		}
		_, err := m.ValidateCreate()
		if err == nil {
			t.Error("expected error for gRPC without address or sidecar")
		}
	})

	t.Run("sidecar missing image", func(t *testing.T) {
		m := validMint()
		m.Spec.PaymentBackend = PaymentBackendConfig{
			GRPCProcessor: &GRPCProcessorConfig{
				SidecarProcessor: &SidecarProcessorConfig{Enabled: true},
			},
		}
		_, err := m.ValidateCreate()
		if err == nil {
			t.Error("expected error for sidecar missing image")
		}
	})

	t.Run("sidecar TLS missing secret", func(t *testing.T) {
		m := validMint()
		m.Spec.PaymentBackend = PaymentBackendConfig{
			GRPCProcessor: &GRPCProcessorConfig{
				SidecarProcessor: &SidecarProcessorConfig{
					Enabled:   true,
					Image:     "proc:latest",
					EnableTLS: true,
				},
			},
		}
		_, err := m.ValidateCreate()
		if err == nil {
			t.Error("expected error for sidecar TLS missing secret")
		}
	})
}
