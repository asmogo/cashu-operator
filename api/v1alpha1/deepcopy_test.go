package v1alpha1

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deepCopyTarget struct {
	name string
	obj  any
}

func TestGeneratedDeepCopyMethods(t *testing.T) {
	cashuMint := sampleCashuMintForDeepCopy()

	for _, target := range deepCopyTargets(cashuMint) {
		t.Run(target.name, func(t *testing.T) {
			verifyDeepCopyMethods(t, target.obj)
		})
	}
}

func TestGeneratedDeepCopyObjectMethods(t *testing.T) {
	cashuMint := sampleCashuMintForDeepCopy()

	deepCopiedObject := cashuMint.DeepCopyObject()
	copiedMint, ok := deepCopiedObject.(*CashuMint)
	if !ok {
		t.Fatalf("DeepCopyObject() returned %T, want *CashuMint", deepCopiedObject)
	}
	if copiedMint == cashuMint {
		t.Fatal("DeepCopyObject() returned the original CashuMint pointer")
	}
	if !reflect.DeepEqual(cashuMint, copiedMint) {
		t.Fatal("DeepCopyObject() should preserve CashuMint contents")
	}
	copiedMint.Spec.Image = "ghcr.io/example/other:latest"
	if cashuMint.Spec.Image == copiedMint.Spec.Image {
		t.Fatal("mutating DeepCopyObject() result should not mutate the original CashuMint")
	}

	list := &CashuMintList{
		TypeMeta: metav1.TypeMeta{APIVersion: "mint.cashu.asmogo.github.io/v1alpha1", Kind: "CashuMintList"},
		ListMeta: metav1.ListMeta{ResourceVersion: "7"},
		Items: []CashuMint{
			*cashuMint.DeepCopy(),
			func() CashuMint {
				copy := cashuMint.DeepCopy()
				copy.Name = "copy-two"
				return *copy
			}(),
		},
	}

	deepCopiedListObject := list.DeepCopyObject()
	copiedList, ok := deepCopiedListObject.(*CashuMintList)
	if !ok {
		t.Fatalf("DeepCopyObject() returned %T, want *CashuMintList", deepCopiedListObject)
	}
	if copiedList == list {
		t.Fatal("DeepCopyObject() returned the original CashuMintList pointer")
	}
	if !reflect.DeepEqual(list, copiedList) {
		t.Fatal("DeepCopyObject() should preserve CashuMintList contents")
	}
	copiedList.Items[0].Spec.Image = "ghcr.io/example/list-copy:latest"
	if list.Items[0].Spec.Image == copiedList.Items[0].Spec.Image {
		t.Fatal("mutating DeepCopyObject() result should not mutate the original CashuMintList")
	}
}

func TestGeneratedDeepCopyNilReceivers(t *testing.T) {
	cashuMint := sampleCashuMintForDeepCopy()

	for _, target := range deepCopyTargets(cashuMint) {
		t.Run(target.name, func(t *testing.T) {
			nilValue := reflect.Zero(reflect.TypeOf(target.obj))
			deepCopy := nilValue.MethodByName("DeepCopy")
			if !deepCopy.IsValid() {
				t.Fatalf("%T does not expose DeepCopy()", target.obj)
			}

			copied := deepCopy.Call(nil)[0]
			if !copied.IsNil() {
				t.Fatalf("DeepCopy() on nil %T returned %#v, want nil", target.obj, copied.Interface())
			}
		})
	}
}

func TestGeneratedDeepCopyObjectNilReceivers(t *testing.T) {
	var cashuMint *CashuMint
	if copied := cashuMint.DeepCopyObject(); copied != nil {
		t.Fatalf("(*CashuMint)(nil).DeepCopyObject() = %#v, want nil", copied)
	}

	var list *CashuMintList
	if copied := list.DeepCopyObject(); copied != nil {
		t.Fatalf("(*CashuMintList)(nil).DeepCopyObject() = %#v, want nil", copied)
	}
}

func verifyDeepCopyMethods(t *testing.T, obj any) {
	t.Helper()

	original := reflect.ValueOf(obj)
	if original.Kind() != reflect.Ptr || original.IsNil() {
		t.Fatalf("expected non-nil pointer target, got %T", obj)
	}

	deepCopy := original.MethodByName("DeepCopy")
	if !deepCopy.IsValid() {
		t.Fatalf("%T does not expose DeepCopy()", obj)
	}

	copied := deepCopy.Call(nil)[0]
	if copied.IsNil() {
		t.Fatalf("DeepCopy() returned nil for %T", obj)
	}
	if copied.Pointer() == original.Pointer() {
		t.Fatalf("DeepCopy() returned the original pointer for %T", obj)
	}
	if !reflect.DeepEqual(original.Interface(), copied.Interface()) {
		t.Fatalf("DeepCopy() changed the contents for %T", obj)
	}

	out := reflect.New(original.Type().Elem())
	deepCopyInto := original.MethodByName("DeepCopyInto")
	if !deepCopyInto.IsValid() {
		t.Fatalf("%T does not expose DeepCopyInto()", obj)
	}
	deepCopyInto.Call([]reflect.Value{out})
	if !reflect.DeepEqual(original.Interface(), out.Interface()) {
		t.Fatalf("DeepCopyInto() changed the contents for %T", obj)
	}

	if !mutateValue(copied.Elem()) {
		t.Fatalf("failed to mutate copied value for %T", obj)
	}
	if reflect.DeepEqual(original.Interface(), copied.Interface()) {
		t.Fatalf("mutating the copy should not leave %T equal to the original", obj)
	}
}

func mutateValue(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return false
		}
		return mutateValue(v.Elem())
	case reflect.String:
		if v.CanSet() {
			v.SetString(v.String() + "-copy")
			return true
		}
	case reflect.Bool:
		if v.CanSet() {
			v.SetBool(!v.Bool())
			return true
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.CanSet() {
			v.SetInt(v.Int() + 1)
			return true
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.CanSet() {
			v.SetUint(v.Uint() + 1)
			return true
		}
	case reflect.Float32, reflect.Float64:
		if v.CanSet() {
			v.SetFloat(v.Float() + 1)
			return true
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if mutateValue(v.Index(i)) {
				return true
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		if iter.Next() && v.Type().Elem().Kind() == reflect.String {
			v.SetMapIndex(iter.Key(), reflect.ValueOf(iter.Value().String()+"-copy").Convert(v.Type().Elem()))
			return true
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if mutateValue(v.Field(i)) {
				return true
			}
		}
	}

	return false
}

func deepCopyTargets(cashuMint *CashuMint) []deepCopyTarget {
	list := &CashuMintList{
		TypeMeta: metav1.TypeMeta{APIVersion: "mint.cashu.asmogo.github.io/v1alpha1", Kind: "CashuMintList"},
		ListMeta: metav1.ListMeta{ResourceVersion: "9"},
		Items: []CashuMint{
			*cashuMint.DeepCopy(),
		},
	}

	return []deepCopyTarget{
		{name: "AuthConfig", obj: cashuMint.Spec.Auth},
		{name: "AuthDatabaseConfig", obj: cashuMint.Spec.Auth.Database},
		{name: "BackupConfig", obj: cashuMint.Spec.Backup},
		{name: "BitcoinRPCConfig", obj: cashuMint.Spec.LDKNode.BitcoinRPC},
		{name: "CLNConfig", obj: cashuMint.Spec.PaymentBackend.CLN},
		{name: "CashuMint", obj: cashuMint},
		{name: "CashuMintList", obj: list},
		{name: "CashuMintSpec", obj: &cashuMint.Spec},
		{name: "CashuMintStatus", obj: &cashuMint.Status},
		{name: "CertManagerConfig", obj: cashuMint.Spec.Ingress.TLS.CertManager},
		{name: "DatabaseConfig", obj: &cashuMint.Spec.Database},
		{name: "DatabaseStatus", obj: &cashuMint.Status.DatabaseStatus},
		{name: "FakeWalletConfig", obj: cashuMint.Spec.PaymentBackend.FakeWallet},
		{name: "GRPCProcessorConfig", obj: cashuMint.Spec.PaymentBackend.GRPCProcessor},
		{name: "HTTPCacheConfig", obj: cashuMint.Spec.HTTPCache},
		{name: "IngressConfig", obj: cashuMint.Spec.Ingress},
		{name: "IngressTLSConfig", obj: cashuMint.Spec.Ingress.TLS},
		{name: "LDKNodeConfig", obj: cashuMint.Spec.LDKNode},
		{name: "LNBitsConfig", obj: cashuMint.Spec.PaymentBackend.LNBits},
		{name: "LNDConfig", obj: cashuMint.Spec.PaymentBackend.LND},
		{name: "LimitsConfig", obj: cashuMint.Spec.Limits},
		{name: "LoggingConfig", obj: cashuMint.Spec.Logging},
		{name: "ManagementRPCConfig", obj: cashuMint.Spec.ManagementRPC},
		{name: "MintInfo", obj: &cashuMint.Spec.MintInfo},
		{name: "OrchardAIConfig", obj: cashuMint.Spec.Orchard.AI},
		{name: "OrchardBitcoinConfig", obj: cashuMint.Spec.Orchard.Bitcoin},
		{name: "OrchardConfig", obj: cashuMint.Spec.Orchard},
		{name: "OrchardLightningConfig", obj: cashuMint.Spec.Orchard.Lightning},
		{name: "OrchardMintConfig", obj: cashuMint.Spec.Orchard.Mint},
		{name: "OrchardMintRPCConfig", obj: cashuMint.Spec.Orchard.Mint.RPC},
		{name: "OrchardTaprootAssetsConfig", obj: cashuMint.Spec.Orchard.TaprootAssets},
		{name: "PaymentBackendConfig", obj: &cashuMint.Spec.PaymentBackend},
		{name: "PaymentBackendStatus", obj: &cashuMint.Status.PaymentBackendStatus},
		{name: "PostgresAutoProvisionSpec", obj: cashuMint.Spec.Database.Postgres.AutoProvisionSpec},
		{name: "PostgresConfig", obj: cashuMint.Spec.Database.Postgres},
		{name: "PrometheusConfig", obj: cashuMint.Spec.Prometheus},
		{name: "QuoteTTLConfig", obj: cashuMint.Spec.MintInfo.QuoteTTL},
		{name: "RedisCacheConfig", obj: cashuMint.Spec.HTTPCache.Redis},
		{name: "S3BackupConfig", obj: cashuMint.Spec.Backup.S3},
		{name: "SQLiteConfig", obj: cashuMint.Spec.Database.SQLite},
		{name: "ServiceConfig", obj: cashuMint.Spec.Service},
		{name: "SidecarProcessorConfig", obj: cashuMint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor},
		{name: "StorageConfig", obj: cashuMint.Spec.Storage},
	}
}

func sampleCashuMintForDeepCopy() *CashuMint {
	storageClassName := "fast-storage"
	replicas := int32(1)
	mintQuoteTTL := int32(600)
	meltQuoteTTL := int32(120)
	inputFee := int32(42)
	postgresMaxConnections := int32(25)
	postgresTimeout := int32(15)
	lndFee := 0.04
	lnbitsFee := 0.02
	fakeWalletFee := 0.03
	grpcPort := int32(50052)
	lndReserveFee := int32(4)
	fakeWalletReserveFee := int32(2)
	minDelay := int32(1)
	maxDelay := int32(5)
	ldkFee := 0.05
	ldkReserveFee := int32(8)
	authMaxBatch := int32(75)
	cacheTTL := int32(90)
	cacheTTI := int32(45)
	prometheusPort := int32(9091)
	maxInputs := int32(21)
	maxOutputs := int32(34)
	retentionCount := int32(14)
	minMint := int64(10)
	maxMint := int64(1000)
	minMelt := int64(20)
	maxMelt := int64(2000)
	allowPrivilegeEscalation := false
	readOnlyRootFilesystem := false
	runAsNonRoot := true
	runAsUser := int64(1000)
	fsGroup := int64(1000)
	useKeysetV2 := true
	clnBolt12 := true
	orchardThrottleTTL := int32(60000)
	orchardThrottleLimit := int32(20)
	orchardCompression := true
	orchardMTLS := true

	databasePostgres := &PostgresConfig{
		URL:                      "postgresql://mint:secret@db:5432/cashu",
		URLSecretRef:             secretKeySelectorPtr("database-secret", "url"),
		TLSMode:                  "require",
		MaxConnections:           &postgresMaxConnections,
		ConnectionTimeoutSeconds: &postgresTimeout,
		AutoProvision:            true,
		AutoProvisionSpec: &PostgresAutoProvisionSpec{
			StorageSize:      "50Gi",
			StorageClassName: &storageClassName,
			Resources:        sampleResourceRequirements(),
			Version:          "16",
		},
	}

	authPostgres := &PostgresConfig{
		URL:                      "postgresql://auth:secret@db:5432/auth",
		URLSecretRef:             secretKeySelectorPtr("auth-database-secret", "url"),
		TLSMode:                  "prefer",
		MaxConnections:           &postgresMaxConnections,
		ConnectionTimeoutSeconds: &postgresTimeout,
		AutoProvision:            false,
		AutoProvisionSpec: &PostgresAutoProvisionSpec{
			StorageSize:      "20Gi",
			StorageClassName: &storageClassName,
			Resources:        sampleResourceRequirements(),
			Version:          "15",
		},
	}

	cashuMint := &CashuMint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "mint.cashu.asmogo.github.io/v1alpha1",
			Kind:       "CashuMint",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "deepcopy-mint",
			Namespace:                  "default",
			Generation:                 7,
			Labels:                     map[string]string{"app": "cashu", "component": "mint"},
			Annotations:                map[string]string{"example.com/annotation": "true"},
			Finalizers:                 []string{"example.com/finalizer"},
			DeletionGracePeriodSeconds: int64Ptr(30),
		},
		Spec: CashuMintSpec{
			Image:           "ghcr.io/example/cdk-mintd:test",
			ImagePullPolicy: corev1.PullAlways,
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "registry-secret"},
			},
			Replicas: &replicas,
			MintInfo: MintInfo{
				URL:                "https://mint.example.com",
				ListenHost:         "0.0.0.0",
				ListenPort:         8443,
				MnemonicSecretRef:  secretKeySelectorPtr("mint-secret", "mnemonic"),
				Name:               "Example Mint",
				Description:        "Short description",
				DescriptionLong:    "Longer description for deepcopy coverage",
				MOTD:               "hello world",
				PubkeyHex:          "abcdef0123456789",
				IconURL:            "https://mint.example.com/icon.png",
				ContactEmail:       "ops@example.com",
				ContactNostrPubkey: "npub1example",
				TosURL:             "https://mint.example.com/tos",
				InputFeePPK:        &inputFee,
				EnableSwaggerUI:    true,
				UseKeysetV2:        &useKeysetV2,
				QuoteTTL: &QuoteTTLConfig{
					MintTTL: &mintQuoteTTL,
					MeltTTL: &meltQuoteTTL,
				},
			},
			Database: DatabaseConfig{
				Engine:   DatabaseEnginePostgres,
				Postgres: databasePostgres,
				SQLite:   &SQLiteConfig{DataDir: "/var/lib/sqlite"},
			},
			PaymentBackend: PaymentBackendConfig{
				MinMint: &minMint,
				MaxMint: &maxMint,
				MinMelt: &minMelt,
				MaxMelt: &maxMelt,
				LND: &LNDConfig{
					Address:           "https://lnd:10009",
					MacaroonSecretRef: secretKeySelectorPtr("lnd-secret", "macaroon"),
					CertSecretRef:     secretKeySelectorPtr("lnd-secret", "cert"),
					FeePercent:        &lndFee,
					ReserveFeeMin:     &lndReserveFee,
				},
				CLN: &CLNConfig{
					RPCPath:       "/var/run/cln/lightning-rpc",
					Bolt12:        &clnBolt12,
					FeePercent:    &lndFee,
					ReserveFeeMin: &lndReserveFee,
				},
				LNBits: &LNBitsConfig{
					API:                    "https://lnbits.example.com",
					AdminAPIKeySecretRef:   secretKeySelector("lnbits-secret", "admin"),
					InvoiceAPIKeySecretRef: secretKeySelector("lnbits-secret", "invoice"),
					RetroAPI:               true,
					FeePercent:             &lnbitsFee,
					ReserveFeeMin:          &fakeWalletReserveFee,
				},
				FakeWallet: &FakeWalletConfig{
					SupportedUnits: []string{"sat", "usd"},
					FeePercent:     &fakeWalletFee,
					ReserveFeeMin:  &fakeWalletReserveFee,
					MinDelayTime:   &minDelay,
					MaxDelayTime:   &maxDelay,
				},
				GRPCProcessor: &GRPCProcessorConfig{
					Address:        "processor.default.svc.cluster.local",
					Port:           grpcPort,
					SupportedUnits: []string{"sat", "usd"},
					TLSSecretRef:   secretKeySelectorPtr("grpc-secret", "client.crt"),
					SidecarProcessor: &SidecarProcessorConfig{
						Enabled:         true,
						Image:           "ghcr.io/example/grpc-processor:latest",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/grpc-processor"},
						Args:            []string{"--verbose"},
						Env:             []corev1.EnvVar{{Name: "MODE", Value: "test"}},
						WorkingDir:      "/var/lib/processor",
						Resources:       sampleResourceRequirements(),
						EnableTLS:       true,
						TLSSecretRef:    secretKeySelectorPtr("grpc-server-secret", "tls.crt"),
					},
				},
			},
			LDKNode: &LDKNodeConfig{
				Enabled:           true,
				Image:             "ghcr.io/example/ldk-node:latest",
				FeePercent:        &ldkFee,
				ReserveFeeMin:     &ldkReserveFee,
				BitcoinNetwork:    "signet",
				ChainSourceType:   "bitcoinrpc",
				EsploraURL:        "https://esplora.example.com",
				BitcoinRPC:        &BitcoinRPCConfig{Host: "bitcoin-rpc", Port: 18443, UserSecretRef: secretKeySelector("bitcoin-rpc", "user"), PasswordSecretRef: secretKeySelector("bitcoin-rpc", "password")},
				StorageDirPath:    "/var/lib/ldk",
				LogDirPath:        "/var/log/ldk",
				MnemonicSecretRef: secretKeySelectorPtr("ldk-secret", "mnemonic"),
				Host:              "0.0.0.0",
				Port:              8090,
				AnnounceAddresses: []string{"127.0.0.1:9735"},
				GossipSourceType:  "rgs",
				RGSURL:            "https://rgs.example.com",
				WebserverHost:     "127.0.0.1",
				WebserverPort:     8888,
			},
			Auth: &AuthConfig{
				Enabled:         true,
				OpenIDDiscovery: "https://issuer.example.com/.well-known/openid-configuration",
				OpenIDClientID:  "cashu-operator",
				MintMaxBat:      &authMaxBatch,
				Mint:            AuthLevelBlind,
				GetMintQuote:    AuthLevelClear,
				CheckMintQuote:  AuthLevelNone,
				Melt:            AuthLevelBlind,
				GetMeltQuote:    AuthLevelClear,
				CheckMeltQuote:  AuthLevelNone,
				Swap:            AuthLevelBlind,
				Restore:         AuthLevelClear,
				CheckProofState: AuthLevelNone,
				Database:        &AuthDatabaseConfig{Postgres: authPostgres},
			},
			HTTPCache: &HTTPCacheConfig{
				Backend: "redis",
				TTL:     &cacheTTL,
				TTI:     &cacheTTI,
				Redis: &RedisCacheConfig{
					KeyPrefix:                 "cashu:",
					ConnectionString:          "redis://redis:6379/0",
					ConnectionStringSecretRef: secretKeySelectorPtr("redis-secret", "url"),
				},
			},
			ManagementRPC: &ManagementRPCConfig{
				Enabled:      true,
				Address:      "127.0.0.1",
				Port:         8086,
				TLSSecretRef: &corev1.LocalObjectReference{Name: "management-rpc-tls"},
			},
			Orchard: &OrchardConfig{
				Enabled:           true,
				Image:             DefaultOrchardPostgresImage,
				ImagePullPolicy:   corev1.PullIfNotPresent,
				Host:              DefaultListenHost,
				Port:              3321,
				BasePath:          "api",
				LogLevel:          "warn",
				SetupKeySecretRef: secretKeySelectorPtr("orchard-secret", "setup-key"),
				ThrottleTTL:       &orchardThrottleTTL,
				ThrottleLimit:     &orchardThrottleLimit,
				Proxy:             "socks5://tor:9050",
				Compression:       &orchardCompression,
				Mint: &OrchardMintConfig{
					Type:                  "cdk",
					API:                   "https://mint.example.com",
					Database:              "postgresql://mint:secret@db:5432/cashu",
					DatabaseCASecretRef:   secretKeySelectorPtr("orchard-db", "ca.pem"),
					DatabaseCertSecretRef: secretKeySelectorPtr("orchard-db", "client.pem"),
					DatabaseKeySecretRef:  secretKeySelectorPtr("orchard-db", "client.key"),
					RPC: &OrchardMintRPCConfig{
						Host: DefaultLoopbackHost,
						Port: 8086,
						MTLS: &orchardMTLS,
					},
				},
				Bitcoin: &OrchardBitcoinConfig{
					Type:                 "core",
					RPCHost:              "bitcoin.internal",
					RPCPort:              18443,
					RPCUserSecretRef:     secretKeySelectorPtr("bitcoin-rpc", "user"),
					RPCPasswordSecretRef: secretKeySelectorPtr("bitcoin-rpc", "password"),
				},
				Lightning: &OrchardLightningConfig{
					Type:              "lnd",
					RPCHost:           "lightning.internal",
					RPCPort:           10009,
					MacaroonSecretRef: secretKeySelectorPtr("lightning", "admin.macaroon"),
					CertSecretRef:     secretKeySelectorPtr("lightning", "tls.cert"),
					KeySecretRef:      secretKeySelectorPtr("lightning", "client.key"),
					CASecretRef:       secretKeySelectorPtr("lightning", "ca.pem"),
				},
				TaprootAssets: &OrchardTaprootAssetsConfig{
					Type:              "tapd",
					RPCHost:           "taproot.internal",
					RPCPort:           10029,
					MacaroonSecretRef: secretKeySelectorPtr("taproot-assets", "admin.macaroon"),
					CertSecretRef:     secretKeySelectorPtr("taproot-assets", "tls.cert"),
				},
				AI: &OrchardAIConfig{
					API: "https://ai.example.com",
				},
				Service: &ServiceConfig{
					Type:           corev1.ServiceTypeClusterIP,
					Annotations:    map[string]string{"example.com/orchard-service": "true"},
					LoadBalancerIP: "10.0.0.25",
				},
				Ingress: &IngressConfig{
					Enabled:   true,
					ClassName: "orchard-nginx",
					Host:      "orchard.example.com",
					TLS: &IngressTLSConfig{
						Enabled:    true,
						SecretName: "orchard-tls",
						CertManager: &CertManagerConfig{
							Enabled:    true,
							IssuerName: "orchard-issuer",
							IssuerKind: "Issuer",
						},
					},
					Annotations: map[string]string{"example.com/orchard": "enabled"},
				},
				Storage:   &StorageConfig{Size: "15Gi", StorageClassName: &storageClassName},
				Resources: sampleResourceRequirements(),
				ContainerSecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: &allowPrivilegeEscalation,
					ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
					RunAsNonRoot:             &runAsNonRoot,
					RunAsUser:                &runAsUser,
				},
				ExtraEnv: []corev1.EnvVar{{Name: "ORCHARD_MODE", Value: "test"}},
			},
			Prometheus: &PrometheusConfig{Enabled: true, Address: "0.0.0.0", Port: &prometheusPort},
			Limits:     &LimitsConfig{MaxInputs: &maxInputs, MaxOutputs: &maxOutputs},
			Ingress: &IngressConfig{
				Enabled:   true,
				ClassName: "custom-nginx",
				Host:      "mint.example.com",
				TLS: &IngressTLSConfig{
					Enabled:    true,
					SecretName: "mint-custom-tls",
					CertManager: &CertManagerConfig{
						Enabled:    true,
						IssuerName: "letsencrypt",
						IssuerKind: "Issuer",
					},
				},
				Annotations: map[string]string{"nginx.ingress.kubernetes.io/proxy-body-size": "16m"},
			},
			Service: &ServiceConfig{
				Type:           corev1.ServiceTypeLoadBalancer,
				Annotations:    map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"},
				LoadBalancerIP: "10.0.0.10",
			},
			Resources:    sampleResourceRequirements(),
			NodeSelector: map[string]string{"topology.kubernetes.io/zone": "us-east-1a"},
			Tolerations:  []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "cashu", Effect: corev1.TaintEffectNoSchedule}},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{{
						Weight: 1,
						Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "topology.kubernetes.io/region",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"us-east-1"},
						}}},
					}},
				},
			},
			Logging: &LoggingConfig{Level: "debug", FileLevel: "info", Format: "pretty"},
			Storage: &StorageConfig{Size: "20Gi", StorageClassName: &storageClassName},
			Backup: &BackupConfig{
				Enabled:        true,
				Schedule:       "0 */6 * * *",
				RetentionCount: &retentionCount,
				S3: &S3BackupConfig{
					Bucket:                   "mint-backups",
					Prefix:                   "cashu",
					Region:                   "us-east-1",
					Endpoint:                 "https://s3.example.com",
					AccessKeyIDSecretRef:     secretKeySelector("backup-secret", "access-key-id"),
					SecretAccessKeySecretRef: secretKeySelector("backup-secret", "secret-access-key"),
				},
			},
			PodSecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &runAsUser,
				FSGroup:      &fsGroup,
			},
			ContainerSecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: &allowPrivilegeEscalation,
				ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
				RunAsNonRoot:             &runAsNonRoot,
			},
		},
		Status: CashuMintStatus{
			Phase:              MintPhaseReady,
			Conditions:         []metav1.Condition{{Type: ConditionTypeReady, Status: metav1.ConditionTrue, Reason: "DeepCopyTest", Message: "ready", LastTransitionTime: metav1.NewTime(time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC))}},
			ObservedGeneration: 7,
			BackendType:        PaymentBackendGRPCProcessor,
			DeploymentName:     "deepcopy-mint",
			ServiceName:        "deepcopy-mint",
			IngressName:        "deepcopy-mint",
			ConfigMapName:      "deepcopy-mint-config",
			DatabaseStatus: DatabaseStatus{
				Connected:   true,
				Message:     "database connected",
				LastChecked: timePtr(2026, time.January, 2, 3, 4, 5),
			},
			PaymentBackendStatus: PaymentBackendStatus{
				Connected:   true,
				Message:     "backend connected",
				LastChecked: timePtr(2026, time.January, 2, 4, 5, 6),
			},
			URL:           "https://mint.example.com",
			ReadyReplicas: 1,
		},
	}

	return cashuMint
}

func sampleResourceRequirements() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("250m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
}

func secretKeySelector(name, key string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  key,
	}
}

func secretKeySelectorPtr(name, key string) *corev1.SecretKeySelector {
	selector := secretKeySelector(name, key)
	return &selector
}

func int64Ptr(v int64) *int64 {
	return &v
}

func timePtr(year int, month time.Month, day, hour, minute, second int) *metav1.Time {
	timestamp := metav1.NewTime(time.Date(year, month, day, hour, minute, second, 0, time.UTC))
	return &timestamp
}
