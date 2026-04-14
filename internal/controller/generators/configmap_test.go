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

package generators

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected config to contain %q, not found in:\n%s", substr, s)
	}
}

func int64PtrGen(i int64) *int64 { return &i }

func baseMint(name string) *mintv1alpha1.CashuMint {
	return &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo:       mintv1alpha1.MintInfo{URL: "http://test.local"},
			Database:       mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
		},
	}
}

func TestGenerateConfigMap_Basic(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("test-mint")

	cm, err := GenerateConfigMap(mint, scheme, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cm.Name != "test-mint-config" {
		t.Errorf("name = %q, want %q", cm.Name, "test-mint-config")
	}
	config := cm.Data["config.toml"]
	assertContains(t, config, `[info]`)
	assertContains(t, config, `url = "http://test.local"`)
	assertContains(t, config, `engine = "sqlite"`)
	assertContains(t, config, `ln_backend = "fakewallet"`)
	assertContains(t, config, `[fake_wallet]`)
	assertLabelsContain(t, cm.Labels, "app.kubernetes.io/instance", "test-mint")
}

func TestGenerateConfigMap_PostgresAutoProvision(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("pg-mint")
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine: "postgres",
		Postgres: &mintv1alpha1.PostgresConfig{
			AutoProvision: true, MaxConnections: int32Ptr(50), ConnectionTimeoutSeconds: int32Ptr(30),
		},
	}

	cm, _ := GenerateConfigMap(mint, scheme, "secret-password")
	config := cm.Data["config.toml"]
	assertContains(t, config, `engine = "postgres"`)
	assertContains(t, config, "[database.postgres]")
	assertContains(t, config, "secret-password@pg-mint-postgres")
	assertContains(t, config, `tls_mode = "disable"`)
	assertContains(t, config, "max_connections = 50")
	assertContains(t, config, "connection_timeout_seconds = 30")
}

func TestGenerateConfigMap_PostgresExternalURL(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("ext-mint")
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine:   "postgres",
		Postgres: &mintv1alpha1.PostgresConfig{URL: "postgresql://user:pass@db:5432/mydb"},
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, `url = "postgresql://user:pass@db:5432/mydb"`)
	assertContains(t, config, `tls_mode = "require"`)
}

func TestGenerateConfigMap_LND(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("lnd-mint")
	mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
		LND: &mintv1alpha1.LNDConfig{Address: "https://lnd:10009", FeePercent: float64Ptr(0.05), ReserveFeeMin: int32Ptr(10)},
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, "ln_backend = \""+lndStr+"\"")
	assertContains(t, config, `[lnd]`)
	assertContains(t, config, `address = "https://lnd:10009"`)
}

func TestGenerateConfigMap_CLN(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("cln-mint")
	mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
		CLN: &mintv1alpha1.CLNConfig{RPCPath: "/rpc/lightning-rpc", Bolt12: boolPtr(true)},
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, `ln_backend = "cln"`)
	assertContains(t, config, `[cln]`)
	assertContains(t, config, `rpc_path = "/rpc/lightning-rpc"`)
	assertContains(t, config, "bolt12 = true")
}

func TestGenerateConfigMap_LNBits(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("lnbits-mint")
	mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
		LNBits: &mintv1alpha1.LNBitsConfig{API: "https://lnbits.example.com", RetroAPI: true},
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, `ln_backend = "lnbits"`)
	assertContains(t, config, `lnbits_api = "https://lnbits.example.com"`)
	assertContains(t, config, "retro_api = true")
}

func TestGenerateConfigMap_GRPCProcessor(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("grpc-mint")
	mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
		GRPCProcessor: &mintv1alpha1.GRPCProcessorConfig{Address: "http://grpc-server", Port: 9999, SupportedUnits: []string{"sat", "usd"}},
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, `ln_backend = "grpcprocessor"`)
	assertContains(t, config, `addr = "http://grpc-server"`)
	assertContains(t, config, "port = 9999")
}

func TestGenerateConfigMap_GRPCProcessorSidecar(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("sidecar-mint")
	mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
		GRPCProcessor: &mintv1alpha1.GRPCProcessorConfig{
			SidecarProcessor: &mintv1alpha1.SidecarProcessorConfig{Enabled: true, Image: "proc:latest"},
		},
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	assertContains(t, cm.Data["config.toml"], `addr = "http://127.0.0.1"`)
}

func TestGenerateConfigMap_MintInfoSection(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("full-mint")
	feePPK := int32(10)
	mint.Spec.MintInfo = mintv1alpha1.MintInfo{
		URL: "http://test.local", Name: "My Mint", Description: "test",
		DescriptionLong: "longer", MOTD: "Welcome!", PubkeyHex: "abcdef",
		IconURL: "https://icon.url", ContactEmail: "a@m.local",
		ContactNostrPubkey: "npub123", TosURL: "https://tos", InputFeePPK: &feePPK,
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, `[mint_info]`)
	assertContains(t, config, `name = "My Mint"`)
	assertContains(t, config, "input_fee_ppk = 10")
}

func TestGenerateConfigMap_QuoteTTL(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("ttl-mint")
	mint.Spec.MintInfo.QuoteTTL = &mintv1alpha1.QuoteTTLConfig{MintTTL: int32Ptr(600), MeltTTL: int32Ptr(120)}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, "[info.quote_ttl]")
	assertContains(t, config, "mint_ttl = 600")
	assertContains(t, config, "melt_ttl = 120")
}

func TestGenerateConfigMap_HTTPCache(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("cache-mint")
	mint.Spec.HTTPCache = &mintv1alpha1.HTTPCacheConfig{Backend: "memory", TTL: int32Ptr(120), TTI: int32Ptr(60)}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, "[info.http_cache]")
	assertContains(t, config, `backend = "memory"`)
	assertContains(t, config, "ttl = 120")
}

func TestGenerateConfigMap_Auth(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("auth-mint")
	mint.Spec.Auth = &mintv1alpha1.AuthConfig{
		Enabled: true, OpenIDDiscovery: "https://auth.example.com", OpenIDClientID: "my-client",
		MintMaxBat: int32Ptr(100), Mint: "clear", Swap: "blind",
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, "[auth]")
	assertContains(t, config, "auth_enabled = true")
	assertContains(t, config, `mint = "clear"`)
}

func TestGenerateConfigMap_ManagementRPC(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("rpc-mint")
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{Enabled: true, Address: "0.0.0.0", Port: 9999}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	assertContains(t, cm.Data["config.toml"], "[mint_management_rpc]")
	assertContains(t, cm.Data["config.toml"], "port = 9999")
}

func TestGenerateConfigMap_Limits(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("limits-mint")
	mint.Spec.Limits = &mintv1alpha1.LimitsConfig{MaxInputs: int32Ptr(128), MaxOutputs: int32Ptr(256)}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	assertContains(t, cm.Data["config.toml"], "max_inputs = 128")
	assertContains(t, cm.Data["config.toml"], "max_outputs = 256")
}

func TestGenerateConfigMap_Prometheus(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("prom-mint")
	mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true, Address: "0.0.0.0", Port: int32Ptr(9090)}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	assertContains(t, cm.Data["config.toml"], "[prometheus]")
	assertContains(t, cm.Data["config.toml"], "port = 9090")
}

func TestGenerateConfigMap_LDKNode(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("ldk-mint")
	mint.Spec.LDKNode = &mintv1alpha1.LDKNodeConfig{
		Enabled: true, BitcoinNetwork: "signet", ChainSourceType: "esplora",
		EsploraURL: "https://esplora.example.com", Host: "0.0.0.0", Port: 8090,
		GossipSourceType: "rgs", RGSURL: "https://rgs.example.com",
		StorageDirPath: "/data/ldk", AnnounceAddresses: []string{"1.2.3.4:9735"},
	}

	cm, _ := GenerateConfigMap(mint, scheme, "")
	config := cm.Data["config.toml"]
	assertContains(t, config, "[ldk_node]")
	assertContains(t, config, `bitcoin_network = "signet"`)
	assertContains(t, config, `esplora_url = "https://esplora.example.com"`)
}

func TestGenerateConfigMap_PaymentLimits(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("paylimit")
	mint.Spec.PaymentBackend.MinMint = int64PtrGen(100)
	mint.Spec.PaymentBackend.MaxMint = int64PtrGen(1000000)

	cm, _ := GenerateConfigMap(mint, scheme, "")
	assertContains(t, cm.Data["config.toml"], "min_mint = 100")
	assertContains(t, cm.Data["config.toml"], "max_mint = 1000000")
}
