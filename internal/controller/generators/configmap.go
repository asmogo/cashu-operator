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
	writeInfoSection(&buf, mint)
	writeMintInfoSection(&buf, mint)
	writeDatabaseSection(&buf, mint, dbPassword)
	writeLnSection(&buf, mint)
	writeLDKNodeSection(&buf, mint)
	writeAuthSection(&buf, mint)
	writeManagementRPCSection(&buf, mint)
	return buf.String(), nil
}

func writeInfoSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
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

	// [info.http_cache] — nested under [info]
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
		if mint.Spec.HTTPCache.Backend == "redis" && mint.Spec.HTTPCache.Redis != nil &&
			mint.Spec.HTTPCache.Redis.KeyPrefix != "" {
			buf.WriteString(fmt.Sprintf("key_prefix = %q\n", mint.Spec.HTTPCache.Redis.KeyPrefix))
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
}

func writeMintInfoSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	mi := mint.Spec.MintInfo
	if mi.Name == "" && mi.Description == "" && mi.DescriptionLong == "" && mi.MOTD == "" &&
		mi.PubkeyHex == "" && mi.IconURL == "" && mi.ContactEmail == "" &&
		mi.ContactNostrPubkey == "" && mi.TosURL == "" && mi.InputFeePPK == nil {
		return
	}
	buf.WriteString("\n[mint_info]\n")
	if mi.Name != "" {
		buf.WriteString(fmt.Sprintf("name = %q\n", mi.Name))
	}
	if mi.Description != "" {
		buf.WriteString(fmt.Sprintf("description = %q\n", mi.Description))
	}
	if mi.DescriptionLong != "" {
		buf.WriteString(fmt.Sprintf("description_long = %q\n", mi.DescriptionLong))
	}
	if mi.MOTD != "" {
		buf.WriteString(fmt.Sprintf("motd = %q\n", mi.MOTD))
	}
	if mi.PubkeyHex != "" {
		buf.WriteString(fmt.Sprintf("pubkey = %q\n", mi.PubkeyHex))
	}
	if mi.IconURL != "" {
		buf.WriteString(fmt.Sprintf("icon_url = %q\n", mi.IconURL))
	}
	if mi.ContactEmail != "" {
		buf.WriteString(fmt.Sprintf("contact_email = %q\n", mi.ContactEmail))
	}
	if mi.ContactNostrPubkey != "" {
		buf.WriteString(fmt.Sprintf("contact_nostr_public_key = %q\n", mi.ContactNostrPubkey))
	}
	if mi.TosURL != "" {
		buf.WriteString(fmt.Sprintf("tos_url = %q\n", mi.TosURL))
	}
	if mi.InputFeePPK != nil {
		buf.WriteString(fmt.Sprintf("input_fee_ppk = %d\n", *mi.InputFeePPK))
	}
}

func writeDatabaseSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint, dbPassword string) {
	buf.WriteString("\n[database]\n")
	buf.WriteString(fmt.Sprintf("engine = %q\n", mint.Spec.Database.Engine))

	if mint.Spec.Database.Engine == mintv1alpha1.DatabaseEnginePostgres && mint.Spec.Database.Postgres != nil {
		buf.WriteString("\n[database.postgres]\n")
		pg := mint.Spec.Database.Postgres
		if pg.AutoProvision {
			dbURL := fmt.Sprintf("postgresql://cdk:%s@%s-postgres:5432/cdk_mintd?sslmode=disable",
				dbPassword, mint.Name)
			buf.WriteString(fmt.Sprintf("url = %q\n", dbURL))
		} else if pg.URL != "" {
			buf.WriteString(fmt.Sprintf("url = %q\n", pg.URL))
		}
		tlsMode := pg.TLSMode
		if tlsMode == "" {
			if pg.AutoProvision {
				tlsMode = "disable"
			} else {
				tlsMode = "require"
			}
		}
		buf.WriteString(fmt.Sprintf("tls_mode = %q\n", tlsMode))
		if pg.MaxConnections != nil {
			buf.WriteString(fmt.Sprintf("max_connections = %d\n", *pg.MaxConnections))
		}
		if pg.ConnectionTimeoutSeconds != nil {
			buf.WriteString(fmt.Sprintf("connection_timeout_seconds = %d\n", *pg.ConnectionTimeoutSeconds))
		}
	}

	// [auth_database.postgres] — top-level, NOT nested under [auth]
	if mint.Spec.Auth != nil && mint.Spec.Auth.Enabled &&
		mint.Spec.Auth.Database != nil && mint.Spec.Auth.Database.Postgres != nil {
		buf.WriteString("\n[auth_database.postgres]\n")
		buf.WriteString("url = \"\"\n")
		buf.WriteString("tls_mode = \"disable\"\n")
	}
}

func writeLnSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
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

	switch mint.Spec.Lightning.Backend {
	case mintv1alpha1.LightningBackendLND:
		writeLNDSection(buf, mint)
	case mintv1alpha1.LightningBackendCLN:
		writeCLNSection(buf, mint)
	case mintv1alpha1.LightningBackendLNBits:
		writeLNBitsSection(buf, mint)
	case mintv1alpha1.LightningBackendFakeWallet:
		writeFakeWalletSection(buf, mint)
	case mintv1alpha1.LightningBackendGRPCProcessor:
		writeGRPCProcessorSection(buf, mint)
	}
}

func writeLNDSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.Lightning.LND == nil {
		return
	}
	lnd := mint.Spec.Lightning.LND
	buf.WriteString("\n[lnd]\n")
	buf.WriteString(fmt.Sprintf("address = %q\n", lnd.Address))
	if lnd.MacaroonSecretRef != nil {
		buf.WriteString("macaroon_file = \"/secrets/lnd/macaroon\"\n")
	}
	if lnd.CertSecretRef != nil {
		buf.WriteString("cert_file = \"/secrets/lnd/cert\"\n")
	}
	if lnd.FeePercent != nil {
		buf.WriteString(fmt.Sprintf("fee_percent = %f\n", *lnd.FeePercent))
	}
	if lnd.ReserveFeeMin != nil {
		buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", *lnd.ReserveFeeMin))
	}
}

func writeCLNSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.Lightning.CLN == nil {
		return
	}
	cln := mint.Spec.Lightning.CLN
	buf.WriteString("\n[cln]\n")
	buf.WriteString(fmt.Sprintf("rpc_path = %q\n", cln.RPCPath))
	if cln.FeePercent != nil {
		buf.WriteString(fmt.Sprintf("fee_percent = %f\n", *cln.FeePercent))
	}
	if cln.ReserveFeeMin != nil {
		buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", *cln.ReserveFeeMin))
	}
}

func writeLNBitsSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.Lightning.LNBits == nil {
		return
	}
	buf.WriteString("\n[lnbits]\n")
	buf.WriteString(fmt.Sprintf("lnbits_api = %q\n", mint.Spec.Lightning.LNBits.API))
	if mint.Spec.Lightning.LNBits.RetroAPI {
		buf.WriteString("retro_api = true\n")
	}
}

func writeFakeWalletSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	buf.WriteString("\n[fake_wallet]\n")
	supportedUnits := []string{"sat"}
	feePercent := 0.02
	reserveFeeMin := int32(1)
	minDelayTime := int32(1)
	maxDelayTime := int32(3)
	if fw := mint.Spec.Lightning.FakeWallet; fw != nil {
		if len(fw.SupportedUnits) > 0 {
			supportedUnits = fw.SupportedUnits
		}
		if fw.FeePercent != nil {
			feePercent = *fw.FeePercent
		}
		if fw.ReserveFeeMin != nil {
			reserveFeeMin = *fw.ReserveFeeMin
		}
		if fw.MinDelayTime != nil {
			minDelayTime = *fw.MinDelayTime
		}
		if fw.MaxDelayTime != nil {
			maxDelayTime = *fw.MaxDelayTime
		}
	}
	buf.WriteString(fmt.Sprintf("supported_units = [\"%s\"]\n", strings.Join(supportedUnits, `", "`)))
	buf.WriteString(fmt.Sprintf("fee_percent = %f\n", feePercent))
	buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", reserveFeeMin))
	buf.WriteString(fmt.Sprintf("min_delay_time = %d\n", minDelayTime))
	buf.WriteString(fmt.Sprintf("max_delay_time = %d\n", maxDelayTime))
}

func writeGRPCProcessorSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.Lightning.GRPCProcessor == nil {
		return
	}
	gp := mint.Spec.Lightning.GRPCProcessor
	buf.WriteString("\n[grpc_processor]\n")

	// addr is passed to tonic Channel::from_shared("{addr}:{port}") which requires a URI scheme.
	addr := gp.Address
	sidecarEnabled := gp.SidecarProcessor != nil && gp.SidecarProcessor.Enabled
	if sidecarEnabled || addr == "" {
		addr = "http://127.0.0.1"
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	buf.WriteString(fmt.Sprintf("addr = %q\n", addr))

	port := gp.Port
	if port == 0 {
		port = 50051
	}
	buf.WriteString(fmt.Sprintf("port = %d\n", port))

	supportedUnits := []string{"sat"}
	if len(gp.SupportedUnits) > 0 {
		supportedUnits = gp.SupportedUnits
	}
	buf.WriteString(fmt.Sprintf("supported_units = [\"%s\"]\n", strings.Join(supportedUnits, `", "`)))

	if gp.TLSSecretRef != nil {
		buf.WriteString("tls_dir = \"/secrets/grpc-tls\"\n")
	}
}

func writeLDKNodeSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.LDKNode == nil || !mint.Spec.LDKNode.Enabled {
		return
	}
	ldk := mint.Spec.LDKNode
	buf.WriteString("\n[ldk_node]\n")
	if ldk.FeePercent != nil {
		buf.WriteString(fmt.Sprintf("fee_percent = %f\n", *ldk.FeePercent))
	}
	if ldk.ReserveFeeMin != nil {
		buf.WriteString(fmt.Sprintf("reserve_fee_min = %d\n", *ldk.ReserveFeeMin))
	}
	buf.WriteString(fmt.Sprintf("bitcoin_network = %q\n", ldk.BitcoinNetwork))
	buf.WriteString(fmt.Sprintf("chain_source = %q\n", ldk.ChainSourceType))
	if ldk.EsploraURL != "" {
		buf.WriteString(fmt.Sprintf("esplora_url = %q\n", ldk.EsploraURL))
	}
	if ldk.BitcoinRPC != nil {
		buf.WriteString(fmt.Sprintf("bitcoin_rpc_host = %q\n", ldk.BitcoinRPC.Host))
		buf.WriteString(fmt.Sprintf("bitcoin_rpc_port = %d\n", ldk.BitcoinRPC.Port))
	}
	if ldk.StorageDirPath != "" {
		buf.WriteString(fmt.Sprintf("storage_dir_path = %q\n", ldk.StorageDirPath))
	}
	host := ldk.Host
	if host == "" {
		host = "0.0.0.0"
	}
	buf.WriteString(fmt.Sprintf("host = %q\n", host))
	port := ldk.Port
	if port == 0 {
		port = 8090
	}
	buf.WriteString(fmt.Sprintf("port = %d\n", port))
	if len(ldk.AnnounceAddresses) > 0 {
		buf.WriteString(fmt.Sprintf("announce_addresses = [\"%s\"]\n", strings.Join(ldk.AnnounceAddresses, `", "`)))
	}
	buf.WriteString(fmt.Sprintf("gossip_source = %q\n", ldk.GossipSourceType))
	if ldk.RGSURL != "" {
		buf.WriteString(fmt.Sprintf("rgs_url = %q\n", ldk.RGSURL))
	}
	webserverHost := ldk.WebserverHost
	if webserverHost == "" {
		webserverHost = "127.0.0.1"
	}
	buf.WriteString(fmt.Sprintf("webserver_host = %q\n", webserverHost))
	webserverPort := ldk.WebserverPort
	if webserverPort == 0 {
		webserverPort = 8888
	}
	buf.WriteString(fmt.Sprintf("webserver_port = %d\n", webserverPort))
}

func writeAuthSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.Auth == nil || !mint.Spec.Auth.Enabled {
		return
	}
	auth := mint.Spec.Auth
	buf.WriteString("\n[auth]\n")
	buf.WriteString("auth_enabled = true\n")
	if auth.OpenIDDiscovery != "" {
		buf.WriteString(fmt.Sprintf("openid_discovery = %q\n", auth.OpenIDDiscovery))
	}
	if auth.OpenIDClientID != "" {
		buf.WriteString(fmt.Sprintf("openid_client_id = %q\n", auth.OpenIDClientID))
	}
	if auth.MintMaxBat != nil {
		buf.WriteString(fmt.Sprintf("mint_max_bat = %d\n", *auth.MintMaxBat))
	}
	if auth.EnabledMint != nil {
		buf.WriteString(fmt.Sprintf("enabled_mint = %t\n", *auth.EnabledMint))
	}
	if auth.EnabledMelt != nil {
		buf.WriteString(fmt.Sprintf("enabled_melt = %t\n", *auth.EnabledMelt))
	}
	if auth.EnabledSwap != nil {
		buf.WriteString(fmt.Sprintf("enabled_swap = %t\n", *auth.EnabledSwap))
	}
	if auth.EnabledCheckMintQuote != nil {
		buf.WriteString(fmt.Sprintf("enabled_check_mint_quote = %t\n", *auth.EnabledCheckMintQuote))
	}
	if auth.EnabledCheckMeltQuote != nil {
		buf.WriteString(fmt.Sprintf("enabled_check_melt_quote = %t\n", *auth.EnabledCheckMeltQuote))
	}
	if auth.EnabledRestore != nil {
		buf.WriteString(fmt.Sprintf("enabled_restore = %t\n", *auth.EnabledRestore))
	}
}

func writeManagementRPCSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.ManagementRPC == nil || !mint.Spec.ManagementRPC.Enabled {
		return
	}
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
