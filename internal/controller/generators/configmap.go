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

// GenerateConfigMap creates a ConfigMap containing the config.toml for the mint
// dbPassword is the postgres password for auto-provisioned databases (can be empty if not applicable)
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

// generateConfigToml generates the TOML configuration content
// dbPassword is the postgres password for auto-provisioned databases (can be empty if not applicable)
func generateConfigToml(mint *mintv1alpha1.CashuMint, dbPassword string) (string, error) {
	var buf bytes.Buffer

	// [info] section
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

	// Mnemonic is loaded via environment variable for security
	if mint.Spec.MintInfo.MnemonicSecretRef != nil {
		buf.WriteString("# Mnemonic loaded from secret via CDK_MINTD_MNEMONIC environment variable\n")
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

		buf.WriteString(fmt.Sprintf("swagger_ui = %t\n", mint.Spec.MintInfo.EnableSwaggerUI))
	}

	// [database] section
	buf.WriteString("\n[database]\n")
	buf.WriteString(fmt.Sprintf("engine = %q\n", mint.Spec.Database.Engine))

	switch mint.Spec.Database.Engine {
	case "postgres":
		if mint.Spec.Database.Postgres != nil {
			buf.WriteString("\n[database.postgres]\n")

			// cdk-mintd requires the database URL to be in the config file
			if mint.Spec.Database.Postgres.AutoProvision {
				// Construct the URL for auto-provisioned postgres with password in cleartext
				postgresHost := fmt.Sprintf("%s-postgres", mint.Name)
				postgresUser := "cdk"
				postgresDB := "cdk_mintd"
				dbURL := fmt.Sprintf("postgresql://%s:%s@%s:5432/%s?sslmode=disable",
					postgresUser, dbPassword, postgresHost, postgresDB)
				buf.WriteString(fmt.Sprintf("url = %q\n", dbURL))
			} else if mint.Spec.Database.Postgres.URL != "" {
				// Direct URL specified (not recommended for production)
				buf.WriteString(fmt.Sprintf("url = %q\n", mint.Spec.Database.Postgres.URL))
			} else {
				// URL from secret - commented out as we need the actual URL in the config
				buf.WriteString("# Database URL must be provided via spec.database.postgres.url or urlSecretRef\n")
			}

			tlsMode := mint.Spec.Database.Postgres.TLSMode
			if tlsMode == "" {
				// Default to 'disable' for auto-provisioned postgres (internal cluster communication)
				// and 'require' for external postgres
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
				buf.WriteString(fmt.Sprintf("connection_timeout = %d\n", *mint.Spec.Database.Postgres.ConnectionTimeoutSeconds))
			}
		}
	case "sqlite":
		if mint.Spec.Database.SQLite != nil {
			buf.WriteString("\n[database.sqlite]\n")
			dataDir := mint.Spec.Database.SQLite.DataDir
			if dataDir == "" {
				dataDir = "/data"
			}
			buf.WriteString(fmt.Sprintf("db_file = %q\n", dataDir+"/mint.db"))
		}
	case "redb":
		buf.WriteString("\n[database.redb]\n")
		buf.WriteString("db_file = \"/data/mint.redb\"\n")
	}

	// [ln] section - Lightning backend configuration
	buf.WriteString("\n[ln]\n")
	buf.WriteString(fmt.Sprintf("ln_backend = %q\n", mint.Spec.Lightning.Backend))

	if mint.Spec.Lightning.MinMint != nil {
		buf.WriteString(fmt.Sprintf("min_mint_amount = %d\n", *mint.Spec.Lightning.MinMint))
	}
	if mint.Spec.Lightning.MaxMint != nil {
		buf.WriteString(fmt.Sprintf("max_mint_amount = %d\n", *mint.Spec.Lightning.MaxMint))
	}
	if mint.Spec.Lightning.MinMelt != nil {
		buf.WriteString(fmt.Sprintf("min_melt_amount = %d\n", *mint.Spec.Lightning.MinMelt))
	}
	if mint.Spec.Lightning.MaxMelt != nil {
		buf.WriteString(fmt.Sprintf("max_melt_amount = %d\n", *mint.Spec.Lightning.MaxMelt))
	}

	// Lightning backend specific configuration
	switch mint.Spec.Lightning.Backend {
	case "lnd":
		if mint.Spec.Lightning.LND != nil {
			buf.WriteString("\n[lnd]\n")
			buf.WriteString(fmt.Sprintf("address = %q\n", mint.Spec.Lightning.LND.Address))

			// Macaroon and cert paths - these are mounted from secrets
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

	case "cln":
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

	case "lnbits":
		if mint.Spec.Lightning.LNBits != nil {
			buf.WriteString("\n[lnbits]\n")
			buf.WriteString(fmt.Sprintf("api = %q\n", mint.Spec.Lightning.LNBits.API))
			// API keys loaded from secrets via environment variables
			buf.WriteString("# admin_api_key loaded from secret via environment variable\n")
			buf.WriteString("# invoice_api_key loaded from secret via environment variable\n")

			if mint.Spec.Lightning.LNBits.RetroAPI {
				buf.WriteString("retro_api = true\n")
			}
		}

	case "fakewallet":
		buf.WriteString("\n[fake_wallet]\n")

		// Set defaults
		supportedUnits := []string{"sat"}
		feePercent := 0.02
		reserveFeeMin := int32(1)
		minDelayTime := int32(1)
		maxDelayTime := int32(3)

		// Override with user-specified values if provided
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

		// Write configuration
		units := strings.Join(supportedUnits, `", "`)
		buf.WriteString(fmt.Sprintf("supported_units = [\"%s\"]\n", units))
		buf.WriteString(fmt.Sprintf("fee_percent = %f\n", feePercent))
		buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", reserveFeeMin))
		buf.WriteString(fmt.Sprintf("min_delay_time = %d\n", minDelayTime))
		buf.WriteString(fmt.Sprintf("max_delay_time = %d\n", maxDelayTime))

	case "grpcprocessor":
		if mint.Spec.Lightning.GRPCProcessor != nil {
			buf.WriteString("\n[grpc_processor]\n")
			buf.WriteString(fmt.Sprintf("addr = %q\n", mint.Spec.Lightning.GRPCProcessor.Address))
			buf.WriteString(fmt.Sprintf("port = %d\n", mint.Spec.Lightning.GRPCProcessor.Port))

			// Default supported units
			supportedUnits := []string{"sat"}
			if len(mint.Spec.Lightning.GRPCProcessor.SupportedUnits) > 0 {
				supportedUnits = mint.Spec.Lightning.GRPCProcessor.SupportedUnits
			}
			units := strings.Join(supportedUnits, `", "`)
			buf.WriteString(fmt.Sprintf("supported_units = [\"%s\"]\n", units))

			// TLS configuration if provided
			if mint.Spec.Lightning.GRPCProcessor.TLSSecretRef != nil {
				buf.WriteString("tls_cert_path = \"/secrets/grpc/client.crt\"\n")
				buf.WriteString("tls_key_path = \"/secrets/grpc/client.key\"\n")
				buf.WriteString("tls_ca_path = \"/secrets/grpc/ca.crt\"\n")
			}
		}
	}

	// [ldk_node] section if enabled
	if mint.Spec.LDKNode != nil && mint.Spec.LDKNode.Enabled {
		buf.WriteString("\n[ldk_node]\n")
		buf.WriteString("enabled = true\n")

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
			buf.WriteString("# Bitcoin RPC credentials loaded from secrets via environment variables\n")
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

	// [auth] section if enabled
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

		// Auth database configuration
		if mint.Spec.Auth.Database != nil && mint.Spec.Auth.Database.Postgres != nil {
			buf.WriteString("\n[auth.database.postgres]\n")
			buf.WriteString("# Auth database URL loaded from CDK_MINTD_AUTH_POSTGRES_URL environment variable\n")
		}
	}

	// [http_cache] section if configured
	if mint.Spec.HTTPCache != nil {
		buf.WriteString("\n[http_cache]\n")
		buf.WriteString(fmt.Sprintf("backend = %q\n", mint.Spec.HTTPCache.Backend))

		if mint.Spec.HTTPCache.TTL != nil {
			buf.WriteString(fmt.Sprintf("ttl = %d\n", *mint.Spec.HTTPCache.TTL))
		}
		if mint.Spec.HTTPCache.TTI != nil {
			buf.WriteString(fmt.Sprintf("tti = %d\n", *mint.Spec.HTTPCache.TTI))
		}

		if mint.Spec.HTTPCache.Backend == "redis" && mint.Spec.HTTPCache.Redis != nil {
			buf.WriteString("\n[http_cache.redis]\n")
			buf.WriteString(fmt.Sprintf("key_prefix = %q\n", mint.Spec.HTTPCache.Redis.KeyPrefix))
			buf.WriteString("# Redis connection string loaded from secret via environment variable\n")
		}
	}

	// [management_rpc] section if enabled
	if mint.Spec.ManagementRPC != nil && mint.Spec.ManagementRPC.Enabled {
		buf.WriteString("\n[management_rpc]\n")
		buf.WriteString("enabled = true\n")

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
