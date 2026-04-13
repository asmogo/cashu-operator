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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGenerateOrchardContainer_Defaults(t *testing.T) {
	mint := baseMint("orchard-defaults")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled:           true,
		SetupKeySecretRef: orchardSecretRef("orchard-setup", "setup-key"),
		Mint:              &mintv1alpha1.OrchardMintConfig{RPC: &mintv1alpha1.OrchardMintRPCConfig{}},
	}
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{Enabled: true}
	mint.Default()

	container := GenerateOrchardContainer(mint)
	if container.Name != orchardStr {
		t.Fatalf("name = %q, want orchard", container.Name)
	}
	if container.Image != mintv1alpha1.DefaultOrchardImage(mintv1alpha1.DatabaseEngineSQLite) {
		t.Fatalf("image = %q, want sqlite default", container.Image)
	}
	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Fatalf("ImagePullPolicy = %v, want IfNotPresent", container.ImagePullPolicy)
	}
	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != orchardPortDefault {
		t.Fatalf("ports = %+v, want %d", container.Ports, orchardPortDefault)
	}
	if container.LivenessProbe == nil || container.LivenessProbe.TCPSocket == nil {
		t.Fatal("expected TCP liveness probe")
	}
	if container.ReadinessProbe == nil || container.ReadinessProbe.TCPSocket == nil {
		t.Fatal("expected TCP readiness probe")
	}
	if container.LivenessProbe.TCPSocket.Port.StrVal != orchardStr {
		t.Fatalf("liveness probe port = %q, want orchard", container.LivenessProbe.TCPSocket.Port.StrVal)
	}
	if container.ReadinessProbe.TCPSocket.Port.StrVal != orchardStr {
		t.Fatalf("readiness probe port = %q, want orchard", container.ReadinessProbe.TCPSocket.Port.StrVal)
	}
	if container.SecurityContext == nil {
		t.Fatal("expected security context")
	}
	if container.SecurityContext.RunAsNonRoot == nil || *container.SecurityContext.RunAsNonRoot {
		t.Fatal("expected RunAsNonRoot=false")
	}
	if container.SecurityContext.RunAsUser == nil || *container.SecurityContext.RunAsUser != 0 {
		t.Fatalf("RunAsUser = %v, want 0", container.SecurityContext.RunAsUser)
	}

	envs := envVarMap(container.Env)
	if envs["SERVER_HOST"] != mintv1alpha1.DefaultListenHost {
		t.Fatalf("SERVER_HOST = %q, want %q", envs["SERVER_HOST"], mintv1alpha1.DefaultListenHost)
	}
	if envs["SERVER_PORT"] != "3321" {
		t.Fatalf("SERVER_PORT = %q, want 3321", envs["SERVER_PORT"])
	}
	if envs["BASE_PATH"] != "api" {
		t.Fatalf("BASE_PATH = %q, want api", envs["BASE_PATH"])
	}
	if envs["LOG_LEVEL"] != "warn" {
		t.Fatalf("LOG_LEVEL = %q, want warn", envs["LOG_LEVEL"])
	}
	if envs["MINT_TYPE"] != "cdk" {
		t.Fatalf("MINT_TYPE = %q, want cdk", envs["MINT_TYPE"])
	}
	if envs["MINT_API"] != "http://127.0.0.1:8085" {
		t.Fatalf("MINT_API = %q, want loopback mint URL", envs["MINT_API"])
	}
	if envs["MINT_RPC_HOST"] != mintv1alpha1.DefaultLoopbackHost {
		t.Fatalf("MINT_RPC_HOST = %q, want %q", envs["MINT_RPC_HOST"], mintv1alpha1.DefaultLoopbackHost)
	}
	if envs["MINT_RPC_PORT"] != "8086" {
		t.Fatalf("MINT_RPC_PORT = %q, want 8086", envs["MINT_RPC_PORT"])
	}
	if envs["MINT_RPC_MTLS"] != trueStr {
		t.Fatalf("MINT_RPC_MTLS = %q, want true", envs["MINT_RPC_MTLS"])
	}
	if envs["MINT_DATABASE"] != orchardMintSQLitePath {
		t.Fatalf("MINT_DATABASE = %q, want %q", envs["MINT_DATABASE"], orchardMintSQLitePath)
	}
	setupKey := orchardEnvVar(t, container.Env, "SETUP_KEY")
	if setupKey.ValueFrom == nil || setupKey.ValueFrom.SecretKeyRef == nil {
		t.Fatal("SETUP_KEY should come from SecretKeyRef")
	}
	if setupKey.ValueFrom.SecretKeyRef.Name != "orchard-setup" || setupKey.ValueFrom.SecretKeyRef.Key != "setup-key" {
		t.Fatalf("unexpected SETUP_KEY ref: %+v", setupKey.ValueFrom.SecretKeyRef)
	}
}

func TestGenerateOrchardContainer_CustomImageAndPort(t *testing.T) {
	mint := baseMint("orchard-custom")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled:           true,
		Image:             "ghcr.io/example/orchard:v2",
		ImagePullPolicy:   corev1.PullAlways,
		Port:              4422,
		SetupKeySecretRef: orchardSecretRef("orchard-setup", "setup-key"),
	}

	container := GenerateOrchardContainer(mint)
	if container.Image != "ghcr.io/example/orchard:v2" {
		t.Fatalf("image = %q, want custom image", container.Image)
	}
	if container.ImagePullPolicy != corev1.PullAlways {
		t.Fatalf("ImagePullPolicy = %v, want Always", container.ImagePullPolicy)
	}
	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 4422 {
		t.Fatalf("ports = %+v, want 4422", container.Ports)
	}
}

func TestGenerateOrchardMintDatabaseEnvVar(t *testing.T) {
	t.Run("custom orchard database takes precedence", func(t *testing.T) {
		mint := baseMint("orchard-db-custom")
		mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
			Mint: &mintv1alpha1.OrchardMintConfig{
				Database: "postgresql://orchard:secret@db/orchard",
			},
		}

		env := generateOrchardMintDatabaseEnvVar(mint)
		if env == nil || env.Value != "postgresql://orchard:secret@db/orchard" {
			t.Fatalf("env = %+v, want custom orchard database URL", env)
		}
	})

	t.Run("sqlite defaults to mounted mint database file", func(t *testing.T) {
		mint := baseMint("orchard-db-sqlite")

		env := generateOrchardMintDatabaseEnvVar(mint)
		if env == nil || env.Value != orchardMintSQLitePath {
			t.Fatalf("env = %+v, want %q", env, orchardMintSQLitePath)
		}
	})

	t.Run("postgres auto-provision uses generated secret", func(t *testing.T) {
		mint := baseMint("orchard-db-auto")
		mint.Spec.Database = mintv1alpha1.DatabaseConfig{
			Engine: mintv1alpha1.DatabaseEnginePostgres,
			Postgres: &mintv1alpha1.PostgresConfig{
				AutoProvision: true,
			},
		}

		env := generateOrchardMintDatabaseEnvVar(mint)
		if env == nil || env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
			t.Fatalf("env = %+v, want generated postgres secret ref", env)
		}
		if env.ValueFrom.SecretKeyRef.Name != "orchard-db-auto-postgres-secret" || env.ValueFrom.SecretKeyRef.Key != "database-url" {
			t.Fatalf("unexpected secret ref: %+v", env.ValueFrom.SecretKeyRef)
		}
	})

	t.Run("postgres external secret ref is reused", func(t *testing.T) {
		mint := baseMint("orchard-db-secret")
		mint.Spec.Database = mintv1alpha1.DatabaseConfig{
			Engine: mintv1alpha1.DatabaseEnginePostgres,
			Postgres: &mintv1alpha1.PostgresConfig{
				URLSecretRef: orchardSecretRef("external-db", "database-url"),
			},
		}

		env := generateOrchardMintDatabaseEnvVar(mint)
		if env == nil || env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
			t.Fatalf("env = %+v, want external secret ref", env)
		}
		if env.ValueFrom.SecretKeyRef.Name != "external-db" || env.ValueFrom.SecretKeyRef.Key != "database-url" {
			t.Fatalf("unexpected secret ref: %+v", env.ValueFrom.SecretKeyRef)
		}
	})

	t.Run("postgres inline URL is used when present", func(t *testing.T) {
		mint := baseMint("orchard-db-url")
		mint.Spec.Database = mintv1alpha1.DatabaseConfig{
			Engine: mintv1alpha1.DatabaseEnginePostgres,
			Postgres: &mintv1alpha1.PostgresConfig{
				URL: "postgresql://cashu:secret@db:5432/cdk",
			},
		}

		env := generateOrchardMintDatabaseEnvVar(mint)
		if env == nil || env.Value != "postgresql://cashu:secret@db:5432/cdk" {
			t.Fatalf("env = %+v, want inline postgres URL", env)
		}
	})
}

func TestGenerateOrchardEnvironmentVariables_WithOptionalConnections(t *testing.T) {
	mint := baseMint("orchard-env")
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{Enabled: true}
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled:           true,
		Host:              mintv1alpha1.DefaultListenHost,
		Port:              3321,
		BasePath:          "api",
		LogLevel:          "warn",
		SetupKeySecretRef: orchardSecretRef("orchard-setup", "setup-key"),
		ThrottleTTL:       int32Ptr(70000),
		ThrottleLimit:     int32Ptr(7),
		Proxy:             "socks5://tor:9050",
		Compression:       boolPtr(true),
		Mint: &mintv1alpha1.OrchardMintConfig{
			Type:                  "cdk",
			DatabaseCASecretRef:   orchardSecretRef("mint-db-ca", "ca.crt"),
			DatabaseCertSecretRef: orchardSecretRef("mint-db-cert", "tls.crt"),
			DatabaseKeySecretRef:  orchardSecretRef("mint-db-key", "tls.key"),
			RPC:                   &mintv1alpha1.OrchardMintRPCConfig{MTLS: boolPtr(true)},
		},
		Bitcoin: &mintv1alpha1.OrchardBitcoinConfig{
			Type:                 "core",
			RPCHost:              "bitcoin.internal",
			RPCPort:              18443,
			RPCUserSecretRef:     orchardSecretRef("bitcoin-rpc", "rpcuser"),
			RPCPasswordSecretRef: orchardSecretRef("bitcoin-rpc", "rpcpassword"),
		},
		Lightning: &mintv1alpha1.OrchardLightningConfig{
			Type:              lndStr,
			RPCHost:           "lnd.internal",
			RPCPort:           10009,
			MacaroonSecretRef: orchardSecretRef(lndStr, "admin.macaroon"),
			CertSecretRef:     orchardSecretRef(lndStr, "tls.cert"),
		},
		TaprootAssets: &mintv1alpha1.OrchardTaprootAssetsConfig{
			Type:              tapdStr,
			RPCHost:           "tapd.internal",
			RPCPort:           10029,
			MacaroonSecretRef: orchardSecretRef(tapdStr, "admin.macaroon"),
			CertSecretRef:     orchardSecretRef(tapdStr, "tls.cert"),
		},
		AI: &mintv1alpha1.OrchardAIConfig{
			API: "https://ai.example.com",
		},
		ExtraEnv: []corev1.EnvVar{{Name: "EXTRA_ENV", Value: "extra"}},
	}

	envs := generateOrchardEnvironmentVariables(mint)
	envMap := envVarMap(envs)

	if envMap["THROTTLE_TTL"] != "70000" {
		t.Fatalf("THROTTLE_TTL = %q, want 70000", envMap["THROTTLE_TTL"])
	}
	if envMap["THROTTLE_LIMIT"] != "7" {
		t.Fatalf("THROTTLE_LIMIT = %q, want 7", envMap["THROTTLE_LIMIT"])
	}
	if envMap["TOR_PROXY_SERVER"] != "socks5://tor:9050" {
		t.Fatalf("TOR_PROXY_SERVER = %q, want socks5 proxy", envMap["TOR_PROXY_SERVER"])
	}
	if envMap["SERVER_COMPRESSION"] != trueStr {
		t.Fatalf("SERVER_COMPRESSION = %q, want true", envMap["SERVER_COMPRESSION"])
	}
	if envMap["MINT_RPC_KEY"] != orchardManagementRPCTLSMountPath+"/client.key" {
		t.Fatalf("MINT_RPC_KEY = %q, want management RPC key path", envMap["MINT_RPC_KEY"])
	}
	if envMap["MINT_RPC_CERT"] != orchardManagementRPCTLSMountPath+"/client.pem" {
		t.Fatalf("MINT_RPC_CERT = %q, want management RPC cert path", envMap["MINT_RPC_CERT"])
	}
	if envMap["MINT_RPC_CA"] != orchardManagementRPCTLSMountPath+"/ca.pem" {
		t.Fatalf("MINT_RPC_CA = %q, want management RPC CA path", envMap["MINT_RPC_CA"])
	}
	if envMap["BITCOIN_TYPE"] != "core" || envMap["BITCOIN_RPC_HOST"] != "bitcoin.internal" || envMap["BITCOIN_RPC_PORT"] != "18443" {
		t.Fatalf("unexpected bitcoin envs: %+v", envMap)
	}
	if envMap["LIGHTNING_TYPE"] != lndStr || envMap["LIGHTNING_RPC_HOST"] != "lnd.internal" || envMap["LIGHTNING_RPC_PORT"] != "10009" {
		t.Fatalf("unexpected lightning envs: %+v", envMap)
	}
	if envMap["LIGHTNING_MACAROON"] != orchardLightningMacaroonPath {
		t.Fatalf("LIGHTNING_MACAROON = %q, want %q", envMap["LIGHTNING_MACAROON"], orchardLightningMacaroonPath)
	}
	if envMap["LIGHTNING_CERT"] != orchardLightningCertPath {
		t.Fatalf("LIGHTNING_CERT = %q, want %q", envMap["LIGHTNING_CERT"], orchardLightningCertPath)
	}
	if envMap["TAPROOT_ASSETS_TYPE"] != tapdStr || envMap["TAPROOT_ASSETS_RPC_HOST"] != "tapd.internal" || envMap["TAPROOT_ASSETS_RPC_PORT"] != "10029" {
		t.Fatalf("unexpected taproot assets envs: %+v", envMap)
	}
	if envMap["TAPROOT_ASSETS_MACAROON"] != orchardTaprootMacaroonPath {
		t.Fatalf("TAPROOT_ASSETS_MACAROON = %q, want %q", envMap["TAPROOT_ASSETS_MACAROON"], orchardTaprootMacaroonPath)
	}
	if envMap["TAPROOT_ASSETS_CERT"] != orchardTaprootCertPath {
		t.Fatalf("TAPROOT_ASSETS_CERT = %q, want %q", envMap["TAPROOT_ASSETS_CERT"], orchardTaprootCertPath)
	}
	if envMap["AI_API"] != "https://ai.example.com" {
		t.Fatalf("AI_API = %q, want https://ai.example.com", envMap["AI_API"])
	}
	if envMap["EXTRA_ENV"] != "extra" {
		t.Fatalf("EXTRA_ENV = %q, want extra", envMap["EXTRA_ENV"])
	}

	if got := orchardEnvVar(t, envs, "SETUP_KEY"); got.ValueFrom == nil || got.ValueFrom.SecretKeyRef == nil || got.ValueFrom.SecretKeyRef.Name != "orchard-setup" {
		t.Fatalf("unexpected SETUP_KEY env: %+v", got)
	}
	if got := orchardEnvVar(t, envs, "BITCOIN_RPC_USER"); got.ValueFrom == nil || got.ValueFrom.SecretKeyRef == nil || got.ValueFrom.SecretKeyRef.Name != "bitcoin-rpc" {
		t.Fatalf("unexpected BITCOIN_RPC_USER env: %+v", got)
	}
	if got := orchardEnvVar(t, envs, "BITCOIN_RPC_PASSWORD"); got.ValueFrom == nil || got.ValueFrom.SecretKeyRef == nil || got.ValueFrom.SecretKeyRef.Name != "bitcoin-rpc" {
		t.Fatalf("unexpected BITCOIN_RPC_PASSWORD env: %+v", got)
	}
	if got := orchardEnvVar(t, envs, "MINT_DATABASE_CA"); got.ValueFrom == nil || got.ValueFrom.SecretKeyRef == nil || got.ValueFrom.SecretKeyRef.Name != "mint-db-ca" {
		t.Fatalf("unexpected MINT_DATABASE_CA env: %+v", got)
	}
	if got := orchardEnvVar(t, envs, "MINT_DATABASE_CERT"); got.ValueFrom == nil || got.ValueFrom.SecretKeyRef == nil || got.ValueFrom.SecretKeyRef.Name != "mint-db-cert" {
		t.Fatalf("unexpected MINT_DATABASE_CERT env: %+v", got)
	}
	if got := orchardEnvVar(t, envs, "MINT_DATABASE_KEY"); got.ValueFrom == nil || got.ValueFrom.SecretKeyRef == nil || got.ValueFrom.SecretKeyRef.Name != "mint-db-key" {
		t.Fatalf("unexpected MINT_DATABASE_KEY env: %+v", got)
	}
}

func TestGenerateOrchardVolumeMounts_WithMTLSAndSecrets(t *testing.T) {
	mint := baseMint("orchard-mounts")
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{Enabled: true}
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled: true,
		Mint: &mintv1alpha1.OrchardMintConfig{
			RPC: &mintv1alpha1.OrchardMintRPCConfig{MTLS: boolPtr(true)},
		},
		Lightning: &mintv1alpha1.OrchardLightningConfig{
			Type:              "cln",
			RPCHost:           "cln.internal",
			RPCPort:           9736,
			KeySecretRef:      orchardSecretRef("cln", "client-key.pem"),
			CASecretRef:       orchardSecretRef("cln", "ca.pem"),
			MacaroonSecretRef: orchardSecretRef("cln", "ignored"),
			CertSecretRef:     orchardSecretRef("cln", "ignored-cert"),
		},
		TaprootAssets: &mintv1alpha1.OrchardTaprootAssetsConfig{
			RPCHost:           "tapd.internal",
			RPCPort:           10029,
			MacaroonSecretRef: orchardSecretRef(tapdStr, "admin.macaroon"),
			CertSecretRef:     orchardSecretRef(tapdStr, "tls.cert"),
		},
	}

	mounts := generateOrchardVolumeMounts(mint)
	if len(mounts) < 8 {
		t.Fatalf("mount count = %d, want orchard base + secret mounts", len(mounts))
	}

	if got := orchardMount(t, mounts, orchardDataVolumeName); got.MountPath != orchardDataDir {
		t.Fatalf("orchard data mount path = %q, want %q", got.MountPath, orchardDataDir)
	}
	if got := orchardMount(t, mounts, orchardTmpVolumeName); got.MountPath != orchardTmpDir {
		t.Fatalf("orchard tmp mount path = %q, want %q", got.MountPath, orchardTmpDir)
	}
	if got := orchardMount(t, mounts, "data"); got.MountPath != orchardMintDataDir {
		t.Fatalf("mint data mount path = %q, want %q", got.MountPath, orchardMintDataDir)
	}
	if got := orchardMount(t, mounts, managementRPCTLSVolumeName); got.MountPath != orchardManagementRPCTLSMountPath || !got.ReadOnly {
		t.Fatalf("unexpected management RPC TLS mount: %+v", got)
	}
	if got := orchardMount(t, mounts, orchardLightningKeyVolumeName); got.MountPath != orchardLightningKeyPath || got.SubPath != "client-key.pem" || !got.ReadOnly {
		t.Fatalf("unexpected lightning key mount: %+v", got)
	}
	if got := orchardMount(t, mounts, orchardLightningCAVolumeName); got.MountPath != orchardLightningCAPath || got.SubPath != "ca.pem" || !got.ReadOnly {
		t.Fatalf("unexpected lightning CA mount: %+v", got)
	}
	if got := orchardMount(t, mounts, orchardTaprootMacaroonVolumeName); got.MountPath != orchardTaprootMacaroonPath || got.SubPath != "admin.macaroon" || !got.ReadOnly {
		t.Fatalf("unexpected taproot macaroon mount: %+v", got)
	}
	if got := orchardMount(t, mounts, orchardTaprootCertVolumeName); got.MountPath != orchardTaprootCertPath || got.SubPath != "tls.cert" || !got.ReadOnly {
		t.Fatalf("unexpected taproot cert mount: %+v", got)
	}
}

func TestGenerateOrchardVolumes(t *testing.T) {
	t.Run("disabled returns nil", func(t *testing.T) {
		mint := baseMint("orchard-volumes-disabled")
		if volumes := GenerateOrchardVolumes(mint); volumes != nil {
			t.Fatalf("volumes = %+v, want nil", volumes)
		}
	})

	t.Run("enabled includes pvc and secret-backed volumes", func(t *testing.T) {
		mint := baseMint("orchard-volumes")
		mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
			Enabled: true,
			Lightning: &mintv1alpha1.OrchardLightningConfig{
				Type:              lndStr,
				RPCHost:           "lnd.internal",
				RPCPort:           10009,
				MacaroonSecretRef: orchardSecretRef(lndStr, "admin.macaroon"),
				CertSecretRef:     orchardSecretRef(lndStr, "tls.cert"),
			},
			TaprootAssets: &mintv1alpha1.OrchardTaprootAssetsConfig{
				RPCHost:           "tapd.internal",
				RPCPort:           10029,
				MacaroonSecretRef: orchardSecretRef(tapdStr, "admin.macaroon"),
				CertSecretRef:     orchardSecretRef(tapdStr, "tls.cert"),
			},
		}

		volumes := GenerateOrchardVolumes(mint)
		if len(volumes) != 6 {
			t.Fatalf("volume count = %d, want 6", len(volumes))
		}
		if got := orchardVolume(t, volumes, orchardDataVolumeName); got.PersistentVolumeClaim == nil || got.PersistentVolumeClaim.ClaimName != "orchard-volumes-orchard-data" {
			t.Fatalf("unexpected orchard data volume: %+v", got)
		}
		if got := orchardVolume(t, volumes, orchardTmpVolumeName); got.EmptyDir == nil {
			t.Fatalf("unexpected orchard tmp volume: %+v", got)
		}
		if got := orchardVolume(t, volumes, orchardLightningMacaroonVolumeName); got.Secret == nil || got.Secret.SecretName != lndStr {
			t.Fatalf("unexpected lightning macaroon volume: %+v", got)
		}
		if got := orchardVolume(t, volumes, orchardLightningCertVolumeName); got.Secret == nil || got.Secret.SecretName != lndStr {
			t.Fatalf("unexpected lightning cert volume: %+v", got)
		}
		if got := orchardVolume(t, volumes, orchardTaprootMacaroonVolumeName); got.Secret == nil || got.Secret.SecretName != tapdStr {
			t.Fatalf("unexpected taproot macaroon volume: %+v", got)
		}
		if got := orchardVolume(t, volumes, orchardTaprootCertVolumeName); got.Secret == nil || got.Secret.SecretName != tapdStr {
			t.Fatalf("unexpected taproot cert volume: %+v", got)
		}
	})
}

func TestGenerateOrchardPVC(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("orchard-pvc")
	mint.UID = types.UID("orchard-pvc-uid")
	storageClass := fastSSD
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled: true,
		Storage: &mintv1alpha1.StorageConfig{
			Size:             "20Gi",
			StorageClassName: &storageClass,
		},
	}

	pvc, err := GenerateOrchardPVC(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pvc == nil {
		t.Fatal("expected PVC")
	}
	if pvc.Name != "orchard-pvc-orchard-data" {
		t.Fatalf("name = %q, want orchard-pvc-orchard-data", pvc.Name)
	}
	if pvc.Namespace != "default" {
		t.Fatalf("namespace = %q, want default", pvc.Namespace)
	}
	if actual := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; !actual.Equal(resource.MustParse("20Gi")) {
		t.Fatalf("storage request = %s, want 20Gi", actual.String())
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != fastSSD {
		t.Fatalf("StorageClassName = %v, want fast-ssd", pvc.Spec.StorageClassName)
	}
	assertLabelsContain(t, pvc.Labels, "app.kubernetes.io/instance", "orchard-pvc")
	assertLabelsContain(t, pvc.Labels, "app.kubernetes.io/component", orchardStr)
	if len(pvc.OwnerReferences) != 1 || pvc.OwnerReferences[0].Name != mint.Name {
		t.Fatalf("unexpected owner refs: %+v", pvc.OwnerReferences)
	}
}

func TestGenerateOrchardService(t *testing.T) {
	scheme := testScheme(t)

	t.Run("defaults to ClusterIP", func(t *testing.T) {
		mint := baseMint("orchard-service-default")
		mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
			Enabled: true,
			Port:    3321,
		}

		service, err := GenerateOrchardService(mint, scheme)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if service == nil {
			t.Fatal("expected Service")
		}
		if service.Spec.Type != corev1.ServiceTypeClusterIP {
			t.Fatalf("type = %v, want ClusterIP", service.Spec.Type)
		}
		if len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != 3321 {
			t.Fatalf("ports = %+v, want orchard port 3321", service.Spec.Ports)
		}
		if service.Spec.Selector["app.kubernetes.io/instance"] != mint.Name {
			t.Fatalf("selector = %+v, want instance label", service.Spec.Selector)
		}
	})

	t.Run("preserves annotations and load balancer IP", func(t *testing.T) {
		mint := baseMint("orchard-service-lb")
		mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
			Enabled: true,
			Port:    4444,
			Service: &mintv1alpha1.ServiceConfig{
				Type:           corev1.ServiceTypeLoadBalancer,
				LoadBalancerIP: "10.0.0.25",
				Annotations: map[string]string{
					"example.com/expose": trueStr,
				},
			},
		}

		service, err := GenerateOrchardService(mint, scheme)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
			t.Fatalf("type = %v, want LoadBalancer", service.Spec.Type)
		}
		if service.Spec.LoadBalancerIP != "10.0.0.25" {
			t.Fatalf("LoadBalancerIP = %q, want 10.0.0.25", service.Spec.LoadBalancerIP)
		}
		if service.Annotations["example.com/expose"] != trueStr {
			t.Fatalf("annotations = %+v, want custom annotation", service.Annotations)
		}
		if len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != 4444 {
			t.Fatalf("ports = %+v, want orchard port 4444", service.Spec.Ports)
		}
	})
}

func TestGenerateOrchardIngress(t *testing.T) {
	scheme := testScheme(t)

	t.Run("disabled returns nil", func(t *testing.T) {
		mint := baseMint("orchard-ingress-disabled")
		mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{Enabled: true}

		ingress, err := GenerateOrchardIngress(mint, scheme)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ingress != nil {
			t.Fatalf("ingress = %+v, want nil", ingress)
		}
	})

	t.Run("enabled renders cert-manager and TLS configuration", func(t *testing.T) {
		mint := baseMint("orchard-ingress")
		mint.UID = types.UID("orchard-ingress-uid")
		mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
			Enabled: true,
			Port:    4444,
			Ingress: &mintv1alpha1.IngressConfig{
				Enabled: true,
				Host:    "orchard.example.com",
				Annotations: map[string]string{
					"example.com/custom": trueStr,
				},
				TLS: &mintv1alpha1.IngressTLSConfig{
					Enabled: true,
					CertManager: &mintv1alpha1.CertManagerConfig{
						Enabled:    true,
						IssuerName: "letsencrypt",
					},
				},
			},
		}

		ingress, err := GenerateOrchardIngress(mint, scheme)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ingress == nil {
			t.Fatal("expected Ingress")
		}
		if ingress.Name != "orchard-ingress-orchard" {
			t.Fatalf("name = %q, want orchard-ingress-orchard", ingress.Name)
		}
		if ingress.Spec.IngressClassName == nil || *ingress.Spec.IngressClassName != mintv1alpha1.DefaultIngressClassName {
			t.Fatalf("IngressClassName = %v, want %q", ingress.Spec.IngressClassName, mintv1alpha1.DefaultIngressClassName)
		}
		if len(ingress.Spec.Rules) != 1 || ingress.Spec.Rules[0].Host != "orchard.example.com" {
			t.Fatalf("rules = %+v, want orchard host", ingress.Spec.Rules)
		}
		path := ingress.Spec.Rules[0].HTTP.Paths[0]
		if path.Path != "/" {
			t.Fatalf("path = %q, want /", path.Path)
		}
		if path.Backend.Service == nil || path.Backend.Service.Name != "orchard-ingress-orchard" || path.Backend.Service.Port.Number != 4444 {
			t.Fatalf("unexpected ingress backend: %+v", path.Backend.Service)
		}
		if len(ingress.Spec.TLS) != 1 || ingress.Spec.TLS[0].SecretName != "orchard-ingress-orchard-tls" {
			t.Fatalf("TLS = %+v, want generated orchard TLS secret", ingress.Spec.TLS)
		}
		if ingress.Annotations["cert-manager.io/issuer"] != "letsencrypt" {
			t.Fatalf("annotations = %+v, want cert-manager issuer", ingress.Annotations)
		}
		if ingress.Annotations["cert-manager.io/issuer-kind"] != mintv1alpha1.DefaultClusterIssuerKind {
			t.Fatalf("annotations = %+v, want default issuer kind", ingress.Annotations)
		}
		if ingress.Annotations["example.com/custom"] != trueStr {
			t.Fatalf("annotations = %+v, want custom annotation", ingress.Annotations)
		}
		if len(ingress.OwnerReferences) != 1 || ingress.OwnerReferences[0].Name != mint.Name {
			t.Fatalf("unexpected owner refs: %+v", ingress.OwnerReferences)
		}
	})
}

func orchardSecretRef(name, key string) *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  key,
	}
}

func orchardEnvVar(t *testing.T, envs []corev1.EnvVar, name string) corev1.EnvVar {
	t.Helper()
	for _, env := range envs {
		if env.Name == name {
			return env
		}
	}
	t.Fatalf("environment variable %q not found", name)
	return corev1.EnvVar{}
}

func orchardMount(t *testing.T, mounts []corev1.VolumeMount, name string) corev1.VolumeMount {
	t.Helper()
	for _, mount := range mounts {
		if mount.Name == name {
			return mount
		}
	}
	t.Fatalf("volume mount %q not found", name)
	return corev1.VolumeMount{}
}

func orchardVolume(t *testing.T, volumes []corev1.Volume, name string) corev1.Volume {
	t.Helper()
	for _, volume := range volumes {
		if volume.Name == name {
			return volume
		}
	}
	t.Fatalf("volume %q not found", name)
	return corev1.Volume{}
}
