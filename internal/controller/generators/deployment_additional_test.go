package generators

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGenerateEnvironmentVariables_CoversDirectAndSecretSources(t *testing.T) {
	mint := baseMint("env-rich")
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine: mintv1alpha1.DatabaseEnginePostgres,
		Postgres: &mintv1alpha1.PostgresConfig{
			URL: "postgresql://user:pass@db:5432/cashu",
		},
	}
	mint.Spec.Auth = &mintv1alpha1.AuthConfig{
		Enabled: true,
		Database: &mintv1alpha1.AuthDatabaseConfig{
			Postgres: &mintv1alpha1.PostgresConfig{
				URLSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "auth-db"}, Key: "url"},
			},
		},
	}
	mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
		LNBits: &mintv1alpha1.LNBitsConfig{
			API:                    "https://lnbits.example.com",
			AdminAPIKeySecretRef:   corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnbits"}, Key: "admin"},
			InvoiceAPIKeySecretRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnbits"}, Key: "invoice"},
		},
	}
	mint.Spec.LDKNode = &mintv1alpha1.LDKNodeConfig{
		Enabled: true,
		BitcoinRPC: &mintv1alpha1.BitcoinRPCConfig{
			UserSecretRef:     corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "bitcoin-rpc"}, Key: "user"},
			PasswordSecretRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "bitcoin-rpc"}, Key: "password"},
		},
		MnemonicSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "ldk"}, Key: "mnemonic"},
	}
	mint.Spec.HTTPCache = &mintv1alpha1.HTTPCacheConfig{
		Backend: "redis",
		Redis: &mintv1alpha1.RedisCacheConfig{
			ConnectionString: "redis://cache:6379/0",
		},
	}

	envVars := generateEnvironmentVariables(mint)
	values := envVarMap(envVars)

	if values["CDK_MINTD_POSTGRES_URL"] != "postgresql://user:pass@db:5432/cashu" {
		t.Fatalf("CDK_MINTD_POSTGRES_URL = %q, want direct postgres URL", values["CDK_MINTD_POSTGRES_URL"])
	}
	if values["REDIS_CONNECTION_STRING"] != "redis://cache:6379/0" {
		t.Fatalf("REDIS_CONNECTION_STRING = %q, want direct redis URL", values["REDIS_CONNECTION_STRING"])
	}
	assertEnvSecretRef(t, envVars, "CDK_MINTD_AUTH_POSTGRES_URL", "auth-db", "url")
	assertEnvSecretRef(t, envVars, "LNBITS_ADMIN_API_KEY", "lnbits", "admin")
	assertEnvSecretRef(t, envVars, "LNBITS_INVOICE_API_KEY", "lnbits", "invoice")
	assertEnvSecretRef(t, envVars, "BITCOIN_RPC_USER", "bitcoin-rpc", "user")
	assertEnvSecretRef(t, envVars, "BITCOIN_RPC_PASSWORD", "bitcoin-rpc", "password")
	assertEnvSecretRef(t, envVars, "CDK_LDK_NODE_MNEMONIC", "ldk", "mnemonic")
}

func TestGenerateEnvironmentVariables_UsesRedisSecretWhenProvided(t *testing.T) {
	mint := baseMint("redis-secret")
	mint.Spec.HTTPCache = &mintv1alpha1.HTTPCacheConfig{
		Backend: "redis",
		Redis: &mintv1alpha1.RedisCacheConfig{
			ConnectionStringSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "redis-secret"}, Key: "url"},
		},
	}

	envVars := generateEnvironmentVariables(mint)
	assertEnvSecretRef(t, envVars, "REDIS_CONNECTION_STRING", "redis-secret", "url")
}

func TestGenerateVolumeMounts_CoversBackendSecrets(t *testing.T) {
	t.Run(lndStr, func(t *testing.T) {
		mint := baseMint("lnd-mounts")
		mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
			LND: &mintv1alpha1.LNDConfig{
				Address:           "https://lnd:10009",
				MacaroonSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnd-secret"}, Key: "macaroon"},
				CertSecretRef:     &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnd-secret"}, Key: "cert"},
			},
		}

		mounts := generateVolumeMounts(mint)
		assertVolumeMount(t, mounts, "lnd-macaroon", "/secrets/lnd/macaroon", "macaroon")
		assertVolumeMount(t, mounts, "lnd-cert", "/secrets/lnd/cert", "cert")
	})

	t.Run("grpc", func(t *testing.T) {
		mint := baseMint("grpc-mounts")
		mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
			GRPCProcessor: &mintv1alpha1.GRPCProcessorConfig{
				Address:      "processor.default.svc.cluster.local",
				TLSSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "grpc-secret"}, Key: "client.crt"},
			},
		}

		mounts := generateVolumeMounts(mint)
		assertVolumeMount(t, mounts, "grpc-tls", "/secrets/grpc", "")
	})
}

func TestGenerateVolumes_CoversBackendSecretVolumes(t *testing.T) {
	t.Run(lndStr, func(t *testing.T) {
		mint := baseMint("lnd-volumes")
		mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
			LND: &mintv1alpha1.LNDConfig{
				Address:           "https://lnd:10009",
				MacaroonSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnd-secret"}, Key: "macaroon"},
				CertSecretRef:     &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "lnd-secret"}, Key: "cert"},
			},
		}

		volumes := generateVolumes(mint)
		assertSecretVolume(t, volumes, "lnd-macaroon", "lnd-secret")
		assertSecretVolume(t, volumes, "lnd-cert", "lnd-secret")
	})

	t.Run("grpc with sidecar tls", func(t *testing.T) {
		mint := baseMint("grpc-volumes")
		mint.Spec.Database = mintv1alpha1.DatabaseConfig{
			Engine: mintv1alpha1.DatabaseEnginePostgres,
			Postgres: &mintv1alpha1.PostgresConfig{
				AutoProvision: true,
			},
		}
		mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
			GRPCProcessor: &mintv1alpha1.GRPCProcessorConfig{
				Address:      "processor.default.svc.cluster.local",
				TLSSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "grpc-secret"}, Key: "client.crt"},
				SidecarProcessor: &mintv1alpha1.SidecarProcessorConfig{
					Enabled:      true,
					EnableTLS:    true,
					TLSSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "sidecar-secret"}, Key: "tls.crt"},
				},
			},
		}

		volumes := generateVolumes(mint)
		assertEmptyDirVolume(t, volumes, "data")
		assertSecretVolume(t, volumes, "grpc-tls", "grpc-secret")
		assertSecretVolume(t, volumes, "sidecar-tls", "sidecar-secret")
	})
}

func assertEnvSecretRef(t *testing.T, envVars []corev1.EnvVar, name, secretName, key string) {
	t.Helper()
	for _, envVar := range envVars {
		if envVar.Name != name {
			continue
		}
		if envVar.ValueFrom == nil || envVar.ValueFrom.SecretKeyRef == nil {
			t.Fatalf("%s is missing a SecretKeyRef", name)
		}
		if envVar.ValueFrom.SecretKeyRef.Name != secretName || envVar.ValueFrom.SecretKeyRef.Key != key {
			t.Fatalf("%s secret ref = %s/%s, want %s/%s", name, envVar.ValueFrom.SecretKeyRef.Name, envVar.ValueFrom.SecretKeyRef.Key, secretName, key)
		}
		return
	}
	t.Fatalf("environment variable %s not found", name)
}

func assertVolumeMount(t *testing.T, mounts []corev1.VolumeMount, name, mountPath, subPath string) {
	t.Helper()
	for _, mount := range mounts {
		if mount.Name != name {
			continue
		}
		if mount.MountPath != mountPath {
			t.Fatalf("mountPath for %s = %q, want %q", name, mount.MountPath, mountPath)
		}
		if subPath != "" && mount.SubPath != subPath {
			t.Fatalf("subPath for %s = %q, want %q", name, mount.SubPath, subPath)
		}
		return
	}
	t.Fatalf("volume mount %s not found", name)
}

func assertSecretVolume(t *testing.T, volumes []corev1.Volume, name, secretName string) {
	t.Helper()
	for _, volume := range volumes {
		if volume.Name != name {
			continue
		}
		if volume.Secret == nil || volume.Secret.SecretName != secretName {
			t.Fatalf("secret volume %s = %+v, want secret %q", name, volume.Secret, secretName)
		}
		return
	}
	t.Fatalf("volume %s not found", name)
}

func assertEmptyDirVolume(t *testing.T, volumes []corev1.Volume, name string) {
	t.Helper()
	for _, volume := range volumes {
		if volume.Name != name {
			continue
		}
		if volume.EmptyDir == nil {
			t.Fatalf("volume %s should use emptyDir", name)
		}
		return
	}
	t.Fatalf("volume %s not found", name)
}
