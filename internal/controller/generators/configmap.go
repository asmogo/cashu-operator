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
	"bytes"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// GenerateConfigMap creates a ConfigMap containing the config.toml for the mint.
// dbPassword is the postgres password for auto-provisioned databases (can be empty if not applicable).
func GenerateConfigMap(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme, dbPassword string) (*corev1.ConfigMap, error) {
	configToml, err := generateConfigToml(mint, dbPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to generate config.toml: %w", err)
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "config",
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mint.Name + "-config",
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"config.toml": configToml,
		},
	}

	if err := controllerutil.SetControllerReference(mint, configMap, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return configMap, nil
}

// generateConfigToml generates the TOML configuration content.
// dbPassword is the postgres password for auto-provisioned databases (can be empty if not applicable).
// Field names are taken directly from the CDK cdk-mintd config.rs structs.
func generateConfigToml(mint *mintv1alpha1.CashuMint, dbPassword string) (string, error) {
	var buf bytes.Buffer

	// [info] section
	// Fields: url, listen_host, listen_port, mnemonic (via env), http_cache (nested), logging (nested)
	buf.WriteString("[info]\n")
	buf.WriteString(fmt.Sprintf("url = %q\n", mint.Spec.MintInfo.URL))

	listenHost := mint.Spec.MintInfo.ListenHost
	if listenHost == "" {
		listenHost = "0.0.0.0"
	}
	buf.WriteString(fmt.Sprintf("listen_host = %q\n", listenHost))

	listenPort := mint.Spec.MintInfo.ListenPort
	if listenPort == 0 {
		listenPort = 8085
	}
	buf.WriteString(fmt.Sprintf("listen_port = %d\n", listenPort))

	// Mnemonic is injected via CDK_MINTD_MNEMONIC environment variable; no inline value here.

	// [info.http_cache] — nested under [info], not a top-level section
	if mint.Spec.HTTPCache != nil {
		backend := mint.Spec.HTTPCache.Backend
		if backend == "" {
			backend = "memory"
		}
		buf.WriteString("\n[info.http_cache]\n")
		buf.WriteString(fmt.Sprintf("backend = %q\n", backend))

		if mint.Spec.HTTPCache.TTL != nil {
			buf.WriteString(fmt.Sprintf("ttl = %d\n", *mint.Spec.HTTPCache.TTL))
		}
		if mint.Spec.HTTPCache.TTI != nil {
			buf.WriteString(fmt.Sprintf("tti = %d\n", *mint.Spec.HTTPCache.TTI))
		}
		if mint.Spec.HTTPCache.Backend == "redis" && mint.Spec.HTTPCache.Redis != nil {
			if mint.Spec.HTTPCache.Redis.KeyPrefix != "" {
				buf.WriteString(fmt.Sprintf("key_prefix = %q\n", mint.Spec.HTTPCache.Redis.KeyPrefix))
			}
			// connection_string injected via REDIS_CONNECTION_STRING env var
		}
	}

	// [info.logging] — nested under [info]
	if mint.Spec.Logging != nil {
		buf.WriteString("\n[info.logging]\n")
		buf.WriteString("output = \"stderr\"\n")
		if mint.Spec.Logging.Level != "" {
			buf.WriteString(fmt.Sprintf("console_level = %q\n", mint.Spec.Logging.Level))
		}
	}

	// [mint_info] section
	if mint.Spec.MintInfo.Name != "" || mint.Spec.MintInfo.Description != "" ||
		mint.Spec.MintInfo.DescriptionLong != "" || mint.Spec.MintInfo.MOTD != "" ||
		mint.Spec.MintInfo.PubkeyHex != "" || mint.Spec.MintInfo.IconURL != "" ||
		mint.Spec.MintInfo.ContactEmail != "" || mint.Spec.MintInfo.ContactNostrPubkey != "" ||
		mint.Spec.MintInfo.TosURL != "" || mint.Spec.MintInfo.InputFeePPK != nil {

		buf.WriteString("\n[mint_info]\n")
		if mint.Spec.MintInfo.Name != "" {
			buf.WriteString(fmt.Sprintf("name = %q\n", mint.Spec.MintInfo.Name))
		}
		if mint.Spec.MintInfo.Description != "" {
			buf.WriteString(fmt.Sprintf("description = %q\n", mint.Spec.MintInfo.Description))
		}
		if mint.Spec.MintInfo.DescriptionLong != "" {
			buf.WriteString(fmt.Sprintf("description_long = %q\n", mint.Spec.MintInfo.DescriptionLong))
		}
		if mint.Spec.MintInfo.MOTD != "" {
			buf.WriteString(fmt.Sprintf("motd = %q\n", mint.Spec.MintInfo.MOTD))
		}
		if mint.Spec.MintInfo.PubkeyHex != "" {
			buf.WriteString(fmt.Sprintf("pubkey = %q\n", mint.Spec.MintInfo.PubkeyHex))
		}
		if mint.Spec.MintInfo.IconURL != "" {
			buf.WriteString(fmt.Sprintf("icon_url = %q\n", mint.Spec.MintInfo.IconURL))
		}
		if mint.Spec.MintInfo.ContactEmail != "" {
			buf.WriteString(fmt.Sprintf("contact_email = %q\n", mint.Spec.MintInfo.ContactEmail))
		}
		if mint.Spec.MintInfo.ContactNostrPubkey != "" {
			buf.WriteString(fmt.Sprintf("contact_nostr_public_key = %q\n", mint.Spec.MintInfo.ContactNostrPubkey))
		}
		if mint.Spec.MintInfo.TosURL != "" {
			buf.WriteString(fmt.Sprintf("tos_url = %q\n", mint.Spec.MintInfo.TosURL))
		}
		if mint.Spec.MintInfo.InputFeePPK != nil {
			buf.WriteString(fmt.Sprintf("input_fee_ppk = %d\n", *mint.Spec.MintInfo.InputFeePPK))
		}
	}

	// [database] section
	// engine: "sqlite" or "postgres" (lowercase, matches DatabaseEngine serde rename_all = "lowercase")
	buf.WriteString("\n[database]\n")
	buf.WriteString(fmt.Sprintf("engine = %q\n", mint.Spec.Database.Engine))

	// [database.postgres] — only present when engine = "postgres"
	// Note: there is NO [database.sqlite] section; sqlite needs no subsection config.
	if mint.Spec.Database.Engine == mintv1alpha1.DatabaseEnginePostgres && mint.Spec.Database.Postgres != nil {
		buf.WriteString("\n[database.postgres]\n")

		if mint.Spec.Database.Postgres.AutoProvision {
			// Build the URL from the auto-provisioned postgres service and password
			postgresHost := fmt.Sprintf("%s-postgres", mint.Name)
			dbURL := fmt.Sprintf("postgresql://cdk:%s@%s:5432/cdk_mintd?sslmode=disable",
				dbPassword, postgresHost)
			buf.WriteString(fmt.Sprintf("url = %q\n", dbURL))
		} else if mint.Spec.Database.Postgres.URL != "" {
			buf.WriteString(fmt.Sprintf("url = %q\n", mint.Spec.Database.Postgres.URL))
		}
		// If URLSecretRef: URL is injected via CDK_MINTD_POSTGRES_URL env var; omit from config.

		tlsMode := mint.Spec.Database.Postgres.TLSMode
		if tlsMode == "" {
			if mint.Spec.Database.Postgres.AutoProvision {
				tlsMode = "disable"
			} else {
				tlsMode = "require"
			}
		}
		buf.WriteString(fmt.Sprintf("tls_mode = %q\n", tlsMode))

		if mint.Spec.Database.Postgres.MaxConnections != nil {
			buf.WriteString(fmt.Sprintf("max_connections = %d\n", *mint.Spec.Database.Postgres.MaxConnections))
		}
		if mint.Spec.Database.Postgres.ConnectionTimeoutSeconds != nil {
			buf.WriteString(fmt.Sprintf("connection_timeout_seconds = %d\n", *mint.Spec.Database.Postgres.ConnectionTimeoutSeconds))
		}
	}

	// [auth_database.postgres] — top-level section (NOT nested under [auth])
	if mint.Spec.Auth != nil && mint.Spec.Auth.Enabled &&
		mint.Spec.Auth.Database != nil && mint.Spec.Auth.Database.Postgres != nil {
		buf.WriteString("\n[auth_database.postgres]\n")
		// URL injected via CDK_MINTD_AUTH_POSTGRES_URL environment variable
		buf.WriteString("url = \"\"\n")
		buf.WriteString("tls_mode = \"disable\"\n")
	}

	// [ln] section — Lightning backend selector
	// ln_backend values: "cln", "lnd", "lnbits", "fakewallet", "grpcprocessor", "ldk-node"
	buf.WriteString("\n[ln]\n")
	buf.WriteString(fmt.Sprintf("ln_backend = %q\n", mint.Spec.Lightning.Backend))

	if mint.Spec.Lightning.MinMint != nil {
		buf.WriteString(fmt.Sprintf("min_mint = %d\n", *mint.Spec.Lightning.MinMint))
	}
	if mint.Spec.Lightning.MaxMint != nil {
		buf.WriteString(fmt.Sprintf("max_mint = %d\n", *mint.Spec.Lightning.MaxMint))
	}
	if mint.Spec.Lightning.MinMelt != nil {
		buf.WriteString(fmt.Sprintf("min_melt = %d\n", *mint.Spec.Lightning.MinMelt))
	}
	if mint.Spec.Lightning.MaxMelt != nil {
		buf.WriteString(fmt.Sprintf("max_melt = %d\n", *mint.Spec.Lightning.MaxMelt))
	}

	// Lightning backend-specific sections
	switch mint.Spec.Lightning.Backend {
	case mintv1alpha1.LightningBackendLND:
		if mint.Spec.Lightning.LND != nil {
			buf.WriteString("\n[lnd]\n")
			buf.WriteString(fmt.Sprintf("address = %q\n", mint.Spec.Lightning.LND.Address))
			if mint.Spec.Lightning.LND.MacaroonSecretRef != nil {
				buf.WriteString("macaroon_file = \"/secrets/lnd/macaroon\"\n")
			}
			if mint.Spec.Lightning.LND.CertSecretRef != nil {
				buf.WriteString("cert_file = \"/secrets/lnd/cert\"\n")
			}
			if mint.Spec.Lightning.LND.FeePercent != nil {
				buf.WriteString(fmt.Sprintf("fee_percent = %f\n", *mint.Spec.Lightning.LND.FeePercent))
			}
			if mint.Spec.Lightning.LND.ReserveFeeMin != nil {
				buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", *mint.Spec.Lightning.LND.ReserveFeeMin))
			}
		}

	case mintv1alpha1.LightningBackendCLN:
		if mint.Spec.Lightning.CLN != nil {
			buf.WriteString("\n[cln]\n")
			buf.WriteString(fmt.Sprintf("rpc_path = %q\n", mint.Spec.Lightning.CLN.RPCPath))
			if mint.Spec.Lightning.CLN.FeePercent != nil {
				buf.WriteString(fmt.Sprintf("fee_percent = %f\n", *mint.Spec.Lightning.CLN.FeePercent))
			}
			if mint.Spec.Lightning.CLN.ReserveFeeMin != nil {
				buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", *mint.Spec.Lightning.CLN.ReserveFeeMin))
			}
		}

	case mintv1alpha1.LightningBackendLNBits:
		if mint.Spec.Lightning.LNBits != nil {
			buf.WriteString("\n[lnbits]\n")
			buf.WriteString(fmt.Sprintf("lnbits_api = %q\n", mint.Spec.Lightning.LNBits.API))
			// admin_api_key and invoice_api_key injected via env vars LNBITS_ADMIN_API_KEY / LNBITS_INVOICE_API_KEY
			if mint.Spec.Lightning.LNBits.RetroAPI {
				buf.WriteString("retro_api = true\n")
			}
		}

	case mintv1alpha1.LightningBackendFakeWallet:
		buf.WriteString("\n[fake_wallet]\n")
		supportedUnits := []string{"sat"}
		feePercent := 0.02
		reserveFeeMin := int32(1)
		minDelayTime := int32(1)
		maxDelayTime := int32(3)
		if mint.Spec.Lightning.FakeWallet != nil {
			if len(mint.Spec.Lightning.FakeWallet.SupportedUnits) > 0 {
				supportedUnits = mint.Spec.Lightning.FakeWallet.SupportedUnits
			}
			if mint.Spec.Lightning.FakeWallet.FeePercent != nil {
				feePercent = *mint.Spec.Lightning.FakeWallet.FeePercent
			}
			if mint.Spec.Lightning.FakeWallet.ReserveFeeMin != nil {
				reserveFeeMin = *mint.Spec.Lightning.FakeWallet.ReserveFeeMin
			}
			if mint.Spec.Lightning.FakeWallet.MinDelayTime != nil {
				minDelayTime = *mint.Spec.Lightning.FakeWallet.MinDelayTime
			}
			if mint.Spec.Lightning.FakeWallet.MaxDelayTime != nil {
				maxDelayTime = *mint.Spec.Lightning.FakeWallet.MaxDelayTime
			}
		}
		units := strings.Join(supportedUnits, `", "`)
		buf.WriteString(fmt.Sprintf("supported_units = [\"%s\"]\n", units))
		buf.WriteString(fmt.Sprintf("fee_percent = %f\n", feePercent))
		buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", reserveFeeMin))
		buf.WriteString(fmt.Sprintf("min_delay_time = %d\n", minDelayTime))
		buf.WriteString(fmt.Sprintf("max_delay_time = %d\n", maxDelayTime))

	case mintv1alpha1.LightningBackendGRPCProcessor:
		if mint.Spec.Lightning.GRPCProcessor != nil {
			buf.WriteString("\n[grpc_processor]\n")

			// addr is passed to tonic's Channel::from_shared("{addr}:{port}"), which
			// requires a URI scheme. Use "http://" for plaintext, "https://" for TLS.
			// Default to "http://127.0.0.1" when a sidecar is running on localhost.
			addr := mint.Spec.Lightning.GRPCProcessor.Address
			sidecarEnabled := mint.Spec.Lightning.GRPCProcessor.SidecarProcessor != nil &&
				mint.Spec.Lightning.GRPCProcessor.SidecarProcessor.Enabled
			if sidecarEnabled || addr == "" {
				addr = "http://127.0.0.1"
			}
			// Ensure a scheme is present
			if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
				addr = "http://" + addr
			}
			buf.WriteString(fmt.Sprintf("addr = %q\n", addr))

			port := mint.Spec.Lightning.GRPCProcessor.Port
			if port == 0 {
				port = 50051
			}
			buf.WriteString(fmt.Sprintf("port = %d\n", port))

			supportedUnits := []string{"sat"}
			if len(mint.Spec.Lightning.GRPCProcessor.SupportedUnits) > 0 {
				supportedUnits = mint.Spec.Lightning.GRPCProcessor.SupportedUnits
			}
			units := strings.Join(supportedUnits, `", "`)
			buf.WriteString(fmt.Sprintf("supported_units = [\"%s\"]\n", units))

			// tls_dir: CDK expects a directory path containing the certs, not individual file paths.
			// The secret is mounted as a directory at /secrets/grpc-tls.
			if mint.Spec.Lightning.GRPCProcessor.TLSSecretRef != nil {
				buf.WriteString("tls_dir = \"/secrets/grpc-tls\"\n")
			}
		}
	}

	// [ldk_node] section
	if mint.Spec.LDKNode != nil && mint.Spec.LDKNode.Enabled {
		buf.WriteString("\n[ldk_node]\n")
		if mint.Spec.LDKNode.FeePercent != nil {
			buf.WriteString(fmt.Sprintf("fee_percent = %f\n", *mint.Spec.LDKNode.FeePercent))
		}
		if mint.Spec.LDKNode.ReserveFeeMin != nil {
			buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", *mint.Spec.LDKNode.ReserveFeeMin))
		}
		buf.WriteString(fmt.Sprintf("bitcoin_network = %q\n", mint.Spec.LDKNode.BitcoinNetwork))
		buf.WriteString(fmt.Sprintf("chain_source = %q\n", mint.Spec.LDKNode.ChainSourceType))
		if mint.Spec.LDKNode.EsploraURL != "" {
			buf.WriteString(fmt.Sprintf("esplora_url = %q\n", mint.Spec.LDKNode.EsploraURL))
		}
		if mint.Spec.LDKNode.BitcoinRPC != nil {
			buf.WriteString(fmt.Sprintf("bitcoin_rpc_host = %q\n", mint.Spec.LDKNode.BitcoinRPC.Host))
			buf.WriteString(fmt.Sprintf("bitcoin_rpc_port = %d\n", mint.Spec.LDKNode.BitcoinRPC.Port))
		}
		if mint.Spec.LDKNode.StorageDirPath != "" {
			buf.WriteString(fmt.Sprintf("storage_dir_path = %q\n", mint.Spec.LDKNode.StorageDirPath))
		}
		host := mint.Spec.LDKNode.Host
		if host == "" {
			host = "0.0.0.0"
		}
		buf.WriteString(fmt.Sprintf("host = %q\n", host))
		port := mint.Spec.LDKNode.Port
		if port == 0 {
			port = 8090
		}
		buf.WriteString(fmt.Sprintf("port = %d\n", port))
		if len(mint.Spec.LDKNode.AnnounceAddresses) > 0 {
			addresses := strings.Join(mint.Spec.LDKNode.AnnounceAddresses, `", "`)
			buf.WriteString(fmt.Sprintf("announce_addresses = [\"%s\"]\n", addresses))
		}
		buf.WriteString(fmt.Sprintf("gossip_source = %q\n", mint.Spec.LDKNode.GossipSourceType))
		if mint.Spec.LDKNode.RGSURL != "" {
			buf.WriteString(fmt.Sprintf("rgs_url = %q\n", mint.Spec.LDKNode.RGSURL))
		}
		webserverHost := mint.Spec.LDKNode.WebserverHost
		if webserverHost == "" {
			webserverHost = "127.0.0.1"
		}
		buf.WriteString(fmt.Sprintf("webserver_host = %q\n", webserverHost))
		webserverPort := mint.Spec.LDKNode.WebserverPort
		if webserverPort == 0 {
			webserverPort = 8888
		}
		buf.WriteString(fmt.Sprintf("webserver_port = %d\n", webserverPort))
	}

	// [auth] section
	if mint.Spec.Auth != nil && mint.Spec.Auth.Enabled {
		buf.WriteString("\n[auth]\n")
		buf.WriteString("auth_enabled = true\n")
		if mint.Spec.Auth.OpenIDDiscovery != "" {
			buf.WriteString(fmt.Sprintf("openid_discovery = %q\n", mint.Spec.Auth.OpenIDDiscovery))
		}
		if mint.Spec.Auth.OpenIDClientID != "" {
			buf.WriteString(fmt.Sprintf("openid_client_id = %q\n", mint.Spec.Auth.OpenIDClientID))
		}
		if mint.Spec.Auth.MintMaxBat != nil {
			buf.WriteString(fmt.Sprintf("mint_max_bat = %d\n", *mint.Spec.Auth.MintMaxBat))
		}
		if mint.Spec.Auth.EnabledMint != nil {
			buf.WriteString(fmt.Sprintf("enabled_mint = %t\n", *mint.Spec.Auth.EnabledMint))
		}
		if mint.Spec.Auth.EnabledMelt != nil {
			buf.WriteString(fmt.Sprintf("enabled_melt = %t\n", *mint.Spec.Auth.EnabledMelt))
		}
		if mint.Spec.Auth.EnabledSwap != nil {
			buf.WriteString(fmt.Sprintf("enabled_swap = %t\n", *mint.Spec.Auth.EnabledSwap))
		}
		if mint.Spec.Auth.EnabledCheckMintQuote != nil {
			buf.WriteString(fmt.Sprintf("enabled_check_mint_quote = %t\n", *mint.Spec.Auth.EnabledCheckMintQuote))
		}
		if mint.Spec.Auth.EnabledCheckMeltQuote != nil {
			buf.WriteString(fmt.Sprintf("enabled_check_melt_quote = %t\n", *mint.Spec.Auth.EnabledCheckMeltQuote))
		}
		if mint.Spec.Auth.EnabledRestore != nil {
			buf.WriteString(fmt.Sprintf("enabled_restore = %t\n", *mint.Spec.Auth.EnabledRestore))
		}
	}

	// [mint_management_rpc] section (feature: management-rpc)
	// Note: CDK struct has no "enabled" field; presence of the section enables it.
	if mint.Spec.ManagementRPC != nil && mint.Spec.ManagementRPC.Enabled {
		buf.WriteString("\n[mint_management_rpc]\n")
		address := mint.Spec.ManagementRPC.Address
		if address == "" {
			address = "127.0.0.1"
		}
		buf.WriteString(fmt.Sprintf("address = %q\n", address))
		port := mint.Spec.ManagementRPC.Port
		if port == 0 {
			port = 8086
		}
		buf.WriteString(fmt.Sprintf("port = %d\n", port))
	}

	return buf.String(), nil
}
