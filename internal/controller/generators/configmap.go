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
	configToml := generateConfigToml(mint, dbPassword)

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
func generateConfigToml(mint *mintv1alpha1.CashuMint, dbPassword string) string {
	var buf bytes.Buffer
	writeInfoSection(&buf, mint)
	writeMintInfoSection(&buf, mint)
	writeDatabaseSection(&buf, mint, dbPassword)
	writePaymentBackendSection(&buf, mint)
	writeLDKNodeSection(&buf, mint)
	writeAuthSection(&buf, mint)
	writeManagementRPCSection(&buf, mint)
	writeLimitsSection(&buf, mint)
	writePrometheusSection(&buf, mint)
	return buf.String()
}

func writeInfoSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	buf.WriteString("[info]\n")
	fmt.Fprintf(buf, "url = %q\n", mint.Spec.MintInfo.URL)

	listenHost := mint.Spec.MintInfo.ListenHost
	if listenHost == "" {
		listenHost = "0.0.0.0"
	}
	fmt.Fprintf(buf, "listen_host = %q\n", listenHost)

	listenPort := mint.Spec.MintInfo.ListenPort
	if listenPort == 0 {
		listenPort = 8085
	}
	fmt.Fprintf(buf, "listen_port = %d\n", listenPort)

	if mint.Spec.MintInfo.UseKeysetV2 != nil {
		fmt.Fprintf(buf, "use_keyset_v2 = %t\n", *mint.Spec.MintInfo.UseKeysetV2)
	}

	// [info.quote_ttl] — nested under [info]
	if mint.Spec.MintInfo.QuoteTTL != nil {
		buf.WriteString("\n[info.quote_ttl]\n")
		if mint.Spec.MintInfo.QuoteTTL.MintTTL != nil {
			fmt.Fprintf(buf, "mint_ttl = %d\n", *mint.Spec.MintInfo.QuoteTTL.MintTTL)
		}
		if mint.Spec.MintInfo.QuoteTTL.MeltTTL != nil {
			fmt.Fprintf(buf, "melt_ttl = %d\n", *mint.Spec.MintInfo.QuoteTTL.MeltTTL)
		}
	}

	// [info.http_cache] — nested under [info]
	if mint.Spec.HTTPCache != nil {
		backend := mint.Spec.HTTPCache.Backend
		if backend == "" {
			backend = "memory"
		}
		buf.WriteString("\n[info.http_cache]\n")
		fmt.Fprintf(buf, "backend = %q\n", backend)
		if mint.Spec.HTTPCache.TTL != nil {
			fmt.Fprintf(buf, "ttl = %d\n", *mint.Spec.HTTPCache.TTL)
		}
		if mint.Spec.HTTPCache.TTI != nil {
			fmt.Fprintf(buf, "tti = %d\n", *mint.Spec.HTTPCache.TTI)
		}
		if mint.Spec.HTTPCache.Backend == "redis" && mint.Spec.HTTPCache.Redis != nil {
			if mint.Spec.HTTPCache.Redis.KeyPrefix != "" {
				fmt.Fprintf(buf, "key_prefix = %q\n", mint.Spec.HTTPCache.Redis.KeyPrefix)
			}
			if mint.Spec.HTTPCache.Redis.ConnectionString != "" {
				fmt.Fprintf(buf, "connection_string = %q\n", mint.Spec.HTTPCache.Redis.ConnectionString)
			}
		}
	}

	// [info.logging] — nested under [info]
	if mint.Spec.Logging != nil {
		buf.WriteString("\n[info.logging]\n")
		buf.WriteString("output = \"stderr\"\n")
		if mint.Spec.Logging.Level != "" {
			fmt.Fprintf(buf, "console_level = %q\n", mint.Spec.Logging.Level)
		}
		if mint.Spec.Logging.FileLevel != "" {
			fmt.Fprintf(buf, "file_level = %q\n", mint.Spec.Logging.FileLevel)
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
		fmt.Fprintf(buf, "name = %q\n", mi.Name)
	}
	if mi.Description != "" {
		fmt.Fprintf(buf, "description = %q\n", mi.Description)
	}
	if mi.DescriptionLong != "" {
		fmt.Fprintf(buf, "description_long = %q\n", mi.DescriptionLong)
	}
	if mi.MOTD != "" {
		fmt.Fprintf(buf, "motd = %q\n", mi.MOTD)
	}
	if mi.PubkeyHex != "" {
		fmt.Fprintf(buf, "pubkey = %q\n", mi.PubkeyHex)
	}
	if mi.IconURL != "" {
		fmt.Fprintf(buf, "icon_url = %q\n", mi.IconURL)
	}
	if mi.ContactEmail != "" {
		fmt.Fprintf(buf, "contact_email = %q\n", mi.ContactEmail)
	}
	if mi.ContactNostrPubkey != "" {
		fmt.Fprintf(buf, "contact_nostr_public_key = %q\n", mi.ContactNostrPubkey)
	}
	if mi.TosURL != "" {
		fmt.Fprintf(buf, "tos_url = %q\n", mi.TosURL)
	}
	if mi.InputFeePPK != nil {
		fmt.Fprintf(buf, "input_fee_ppk = %d\n", *mi.InputFeePPK)
	}
}

func writeDatabaseSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint, dbPassword string) {
	buf.WriteString("\n[database]\n")
	fmt.Fprintf(buf, "engine = %q\n", mint.Spec.Database.Engine)

	if mint.Spec.Database.Engine == mintv1alpha1.DatabaseEnginePostgres && mint.Spec.Database.Postgres != nil {
		buf.WriteString("\n[database.postgres]\n")
		pg := mint.Spec.Database.Postgres
		if pg.AutoProvision {
			dbURL := fmt.Sprintf("postgresql://cdk:%s@%s-postgres:5432/cdk_mintd?sslmode=disable",
				dbPassword, mint.Name)
			fmt.Fprintf(buf, "url = %q\n", dbURL)
		} else if pg.URL != "" {
			fmt.Fprintf(buf, "url = %q\n", pg.URL)
		}
		tlsMode := pg.TLSMode
		if tlsMode == "" {
			if pg.AutoProvision {
				tlsMode = "disable"
			} else {
				tlsMode = "require"
			}
		}
		fmt.Fprintf(buf, "tls_mode = %q\n", tlsMode)
		if pg.MaxConnections != nil {
			fmt.Fprintf(buf, "max_connections = %d\n", *pg.MaxConnections)
		}
		if pg.ConnectionTimeoutSeconds != nil {
			fmt.Fprintf(buf, "connection_timeout_seconds = %d\n", *pg.ConnectionTimeoutSeconds)
		}
	}

	// [auth_database.postgres] — top-level, NOT nested under [auth]
	if mint.Spec.Auth != nil && mint.Spec.Auth.Enabled &&
		mint.Spec.Auth.Database != nil && mint.Spec.Auth.Database.Postgres != nil {
		buf.WriteString("\n[auth_database.postgres]\n")
		authPg := mint.Spec.Auth.Database.Postgres
		if authPg.URL != "" {
			fmt.Fprintf(buf, "url = %q\n", authPg.URL)
		} else {
			buf.WriteString("url = \"\"\n")
		}
		tlsMode := authPg.TLSMode
		if tlsMode == "" {
			tlsMode = "disable"
		}
		fmt.Fprintf(buf, "tls_mode = %q\n", tlsMode)
		if authPg.MaxConnections != nil {
			fmt.Fprintf(buf, "max_connections = %d\n", *authPg.MaxConnections)
		}
		if authPg.ConnectionTimeoutSeconds != nil {
			fmt.Fprintf(buf, "connection_timeout_seconds = %d\n", *authPg.ConnectionTimeoutSeconds)
		}
	}
}

func writePaymentBackendSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	backend := mint.Spec.PaymentBackend.ActiveBackend()
	buf.WriteString("\n[ln]\n")
	fmt.Fprintf(buf, "ln_backend = %q\n", backend)
	if mint.Spec.PaymentBackend.MinMint != nil {
		fmt.Fprintf(buf, "min_mint = %d\n", *mint.Spec.PaymentBackend.MinMint)
	}
	if mint.Spec.PaymentBackend.MaxMint != nil {
		fmt.Fprintf(buf, "max_mint = %d\n", *mint.Spec.PaymentBackend.MaxMint)
	}
	if mint.Spec.PaymentBackend.MinMelt != nil {
		fmt.Fprintf(buf, "min_melt = %d\n", *mint.Spec.PaymentBackend.MinMelt)
	}
	if mint.Spec.PaymentBackend.MaxMelt != nil {
		fmt.Fprintf(buf, "max_melt = %d\n", *mint.Spec.PaymentBackend.MaxMelt)
	}

	switch backend {
	case mintv1alpha1.PaymentBackendLND:
		writeLNDSection(buf, mint)
	case mintv1alpha1.PaymentBackendCLN:
		writeCLNSection(buf, mint)
	case mintv1alpha1.PaymentBackendLNBits:
		writeLNBitsSection(buf, mint)
	case mintv1alpha1.PaymentBackendFakeWallet:
		writeFakeWalletSection(buf, mint)
	case mintv1alpha1.PaymentBackendGRPCProcessor:
		writeGRPCProcessorSection(buf, mint)
	}
}

func writeLNDSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.PaymentBackend.LND == nil {
		return
	}
	lnd := mint.Spec.PaymentBackend.LND
	buf.WriteString("\n[lnd]\n")
	fmt.Fprintf(buf, "address = %q\n", lnd.Address)
	if lnd.MacaroonSecretRef != nil {
		buf.WriteString("macaroon_file = \"/secrets/lnd/macaroon\"\n")
	}
	if lnd.CertSecretRef != nil {
		buf.WriteString("cert_file = \"/secrets/lnd/cert\"\n")
	}
	if lnd.FeePercent != nil {
		fmt.Fprintf(buf, "fee_percent = %f\n", *lnd.FeePercent)
	}
	if lnd.ReserveFeeMin != nil {
		fmt.Fprintf(buf, "reserve_fee_min = %d\n", *lnd.ReserveFeeMin)
	}
}

func writeCLNSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.PaymentBackend.CLN == nil {
		return
	}
	cln := mint.Spec.PaymentBackend.CLN
	buf.WriteString("\n[cln]\n")
	fmt.Fprintf(buf, "rpc_path = %q\n", cln.RPCPath)
	if cln.Bolt12 != nil {
		fmt.Fprintf(buf, "bolt12 = %t\n", *cln.Bolt12)
	}
	if cln.FeePercent != nil {
		fmt.Fprintf(buf, "fee_percent = %f\n", *cln.FeePercent)
	}
	if cln.ReserveFeeMin != nil {
		fmt.Fprintf(buf, "reserve_fee_min = %d\n", *cln.ReserveFeeMin)
	}
}

func writeLNBitsSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.PaymentBackend.LNBits == nil {
		return
	}
	lnbits := mint.Spec.PaymentBackend.LNBits
	buf.WriteString("\n[lnbits]\n")
	fmt.Fprintf(buf, "lnbits_api = %q\n", lnbits.API)
	if lnbits.RetroAPI {
		buf.WriteString("retro_api = true\n")
	}
	if lnbits.FeePercent != nil {
		fmt.Fprintf(buf, "fee_percent = %f\n", *lnbits.FeePercent)
	}
	if lnbits.ReserveFeeMin != nil {
		fmt.Fprintf(buf, "reserve_fee_min = %d\n", *lnbits.ReserveFeeMin)
	}
}

func writeFakeWalletSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	buf.WriteString("\n[fake_wallet]\n")
	supportedUnits := []string{"sat"}
	feePercent := 0.02
	reserveFeeMin := int32(1)
	minDelayTime := int32(1)
	maxDelayTime := int32(3)
	if fw := mint.Spec.PaymentBackend.FakeWallet; fw != nil {
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
	fmt.Fprintf(buf, "supported_units = [\"%s\"]\n", strings.Join(supportedUnits, `", "`))
	fmt.Fprintf(buf, "fee_percent = %f\n", feePercent)
	fmt.Fprintf(buf, "reserve_fee_min = %d\n", reserveFeeMin)
	fmt.Fprintf(buf, "min_delay_time = %d\n", minDelayTime)
	fmt.Fprintf(buf, "max_delay_time = %d\n", maxDelayTime)
}

func writeGRPCProcessorSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.PaymentBackend.GRPCProcessor == nil {
		return
	}
	gp := mint.Spec.PaymentBackend.GRPCProcessor
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
	fmt.Fprintf(buf, "addr = %q\n", addr)

	port := gp.Port
	if port == 0 {
		port = 50051
	}
	fmt.Fprintf(buf, "port = %d\n", port)

	supportedUnits := []string{"sat"}
	if len(gp.SupportedUnits) > 0 {
		supportedUnits = gp.SupportedUnits
	}
	fmt.Fprintf(buf, "supported_units = [\"%s\"]\n", strings.Join(supportedUnits, `", "`))

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
		fmt.Fprintf(buf, "fee_percent = %f\n", *ldk.FeePercent)
	}
	if ldk.ReserveFeeMin != nil {
		fmt.Fprintf(buf, "reserve_fee_min = %d\n", *ldk.ReserveFeeMin)
	}
	fmt.Fprintf(buf, "bitcoin_network = %q\n", ldk.BitcoinNetwork)
	fmt.Fprintf(buf, "chain_source = %q\n", ldk.ChainSourceType)
	if ldk.EsploraURL != "" {
		fmt.Fprintf(buf, "esplora_url = %q\n", ldk.EsploraURL)
	}
	if ldk.BitcoinRPC != nil {
		fmt.Fprintf(buf, "bitcoin_rpc_host = %q\n", ldk.BitcoinRPC.Host)
		fmt.Fprintf(buf, "bitcoin_rpc_port = %d\n", ldk.BitcoinRPC.Port)
	}
	if ldk.StorageDirPath != "" {
		fmt.Fprintf(buf, "storage_dir_path = %q\n", ldk.StorageDirPath)
	}
	if ldk.LogDirPath != "" {
		fmt.Fprintf(buf, "log_dir_path = %q\n", ldk.LogDirPath)
	}
	host := ldk.Host
	if host == "" {
		host = "0.0.0.0"
	}
	fmt.Fprintf(buf, "ldk_node_host = %q\n", host)
	port := ldk.Port
	if port == 0 {
		port = 8090
	}
	fmt.Fprintf(buf, "ldk_node_port = %d\n", port)
	if len(ldk.AnnounceAddresses) > 0 {
		fmt.Fprintf(buf, "announce_addresses = [\"%s\"]\n", strings.Join(ldk.AnnounceAddresses, `", "`))
	}
	fmt.Fprintf(buf, "gossip_source = %q\n", ldk.GossipSourceType)
	if ldk.RGSURL != "" {
		fmt.Fprintf(buf, "rgs_url = %q\n", ldk.RGSURL)
	}
	webserverHost := ldk.WebserverHost
	if webserverHost == "" {
		webserverHost = "127.0.0.1"
	}
	fmt.Fprintf(buf, "webserver_host = %q\n", webserverHost)
	webserverPort := ldk.WebserverPort
	if webserverPort == 0 {
		webserverPort = 8888
	}
	fmt.Fprintf(buf, "webserver_port = %d\n", webserverPort)
}

func writeAuthSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.Auth == nil || !mint.Spec.Auth.Enabled {
		return
	}
	auth := mint.Spec.Auth
	buf.WriteString("\n[auth]\n")
	buf.WriteString("auth_enabled = true\n")
	if auth.OpenIDDiscovery != "" {
		fmt.Fprintf(buf, "openid_discovery = %q\n", auth.OpenIDDiscovery)
	}
	if auth.OpenIDClientID != "" {
		fmt.Fprintf(buf, "openid_client_id = %q\n", auth.OpenIDClientID)
	}
	if auth.MintMaxBat != nil {
		fmt.Fprintf(buf, "mint_max_bat = %d\n", *auth.MintMaxBat)
	}
	// Per-endpoint auth levels
	if auth.Mint != "" {
		fmt.Fprintf(buf, "mint = %q\n", auth.Mint)
	}
	if auth.GetMintQuote != "" {
		fmt.Fprintf(buf, "get_mint_quote = %q\n", auth.GetMintQuote)
	}
	if auth.CheckMintQuote != "" {
		fmt.Fprintf(buf, "check_mint_quote = %q\n", auth.CheckMintQuote)
	}
	if auth.Melt != "" {
		fmt.Fprintf(buf, "melt = %q\n", auth.Melt)
	}
	if auth.GetMeltQuote != "" {
		fmt.Fprintf(buf, "get_melt_quote = %q\n", auth.GetMeltQuote)
	}
	if auth.CheckMeltQuote != "" {
		fmt.Fprintf(buf, "check_melt_quote = %q\n", auth.CheckMeltQuote)
	}
	if auth.Swap != "" {
		fmt.Fprintf(buf, "swap = %q\n", auth.Swap)
	}
	if auth.Restore != "" {
		fmt.Fprintf(buf, "restore = %q\n", auth.Restore)
	}
	if auth.CheckProofState != "" {
		fmt.Fprintf(buf, "check_proof_state = %q\n", auth.CheckProofState)
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
	fmt.Fprintf(buf, "address = %q\n", address)
	port := mint.Spec.ManagementRPC.Port
	if port == 0 {
		port = 8086
	}
	fmt.Fprintf(buf, "port = %d\n", port)
}

func writeLimitsSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.Limits == nil {
		return
	}
	buf.WriteString("\n[limits]\n")
	if mint.Spec.Limits.MaxInputs != nil {
		fmt.Fprintf(buf, "max_inputs = %d\n", *mint.Spec.Limits.MaxInputs)
	}
	if mint.Spec.Limits.MaxOutputs != nil {
		fmt.Fprintf(buf, "max_outputs = %d\n", *mint.Spec.Limits.MaxOutputs)
	}
}

func writePrometheusSection(buf *bytes.Buffer, mint *mintv1alpha1.CashuMint) {
	if mint.Spec.Prometheus == nil || !mint.Spec.Prometheus.Enabled {
		return
	}
	buf.WriteString("\n[prometheus]\n")
	buf.WriteString("enabled = true\n")
	address := mint.Spec.Prometheus.Address
	if address == "" {
		address = "0.0.0.0"
	}
	fmt.Fprintf(buf, "address = %q\n", address)
	if mint.Spec.Prometheus.Port != nil {
		fmt.Fprintf(buf, "port = %d\n", *mint.Spec.Prometheus.Port)
	}
}
