package generators

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGenerateOrchardContainer_GeneratorDefaults(t *testing.T) {
	mint := baseMint("orchard-generator-defaults")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled:           true,
		SetupKeySecretRef: orchardSecretRef("orchard-setup", "setup-key"),
	}

	container := GenerateOrchardContainer(mint)

	if container.Image != mintv1alpha1.DefaultOrchardImage(mintv1alpha1.DatabaseEngineSQLite) {
		t.Fatalf("image = %q, want sqlite default", container.Image)
	}
	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Fatalf("ImagePullPolicy = %v, want IfNotPresent", container.ImagePullPolicy)
	}
	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != orchardPortDefault {
		t.Fatalf("ports = %+v, want %d", container.Ports, orchardPortDefault)
	}
	if actual := container.Resources.Requests[corev1.ResourceCPU]; !actual.Equal(resource.MustParse("100m")) {
		t.Fatalf("cpu request = %s, want 100m", actual.String())
	}
	if actual := container.Resources.Requests[corev1.ResourceMemory]; !actual.Equal(resource.MustParse("128Mi")) {
		t.Fatalf("memory request = %s, want 128Mi", actual.String())
	}
}

func TestGenerateOrchardContainer_UsesCustomResourcesAndSecurityContext(t *testing.T) {
	runAsUser := int64(1000)
	mint := baseMint("orchard-custom-runtime")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled:           true,
		SetupKeySecretRef: orchardSecretRef("orchard-setup", "setup-key"),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("750m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		},
		ContainerSecurityContext: &corev1.SecurityContext{
			RunAsUser:                &runAsUser,
			RunAsNonRoot:             boolPtr(true),
			AllowPrivilegeEscalation: boolPtr(false),
		},
	}

	container := GenerateOrchardContainer(mint)

	if actual := container.Resources.Requests[corev1.ResourceCPU]; !actual.Equal(resource.MustParse("250m")) {
		t.Fatalf("cpu request = %s, want 250m", actual.String())
	}
	if actual := container.Resources.Limits[corev1.ResourceMemory]; !actual.Equal(resource.MustParse("1Gi")) {
		t.Fatalf("memory limit = %s, want 1Gi", actual.String())
	}
	if container.SecurityContext != mint.Spec.Orchard.ContainerSecurityContext {
		t.Fatal("expected custom security context to be reused")
	}
}

func TestGenerateOrchardMintDatabaseEnvVar_ReturnsNilWhenPostgresConfigMissing(t *testing.T) {
	mint := baseMint("orchard-db-nil")
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine: mintv1alpha1.DatabaseEnginePostgres,
	}

	if env := generateOrchardMintDatabaseEnvVar(mint); env != nil {
		t.Fatalf("env = %+v, want nil", env)
	}
}

func TestGenerateOrchardMintDatabaseEnvVar_ReturnsNilWhenPostgresURLCannotBeResolved(t *testing.T) {
	mint := baseMint("orchard-db-empty")
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine:   mintv1alpha1.DatabaseEnginePostgres,
		Postgres: &mintv1alpha1.PostgresConfig{},
	}

	if env := generateOrchardMintDatabaseEnvVar(mint); env != nil {
		t.Fatalf("env = %+v, want nil", env)
	}
}

func TestGenerateOrchardVolumes_WithLightningKeyAndCASecrets(t *testing.T) {
	mint := baseMint("orchard-volumes-lightning-tls")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled: true,
		Lightning: &mintv1alpha1.OrchardLightningConfig{
			KeySecretRef: orchardSecretRef("lightning-key", "client.key"),
			CASecretRef:  orchardSecretRef("lightning-ca", "ca.pem"),
		},
	}

	volumes := GenerateOrchardVolumes(mint)
	assertSecretVolume(t, volumes, orchardLightningKeyVolumeName, "lightning-key")
	assertSecretVolume(t, volumes, orchardLightningCAVolumeName, "lightning-ca")
}

func TestGenerateOrchardPVC_Disabled(t *testing.T) {
	pvc, err := GenerateOrchardPVC(baseMint("orchard-pvc-disabled"), testScheme(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pvc != nil {
		t.Fatalf("pvc = %+v, want nil", pvc)
	}
}

func TestGenerateOrchardPVC_DefaultStorageSettings(t *testing.T) {
	mint := baseMint("orchard-pvc-defaults")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{Enabled: true}

	pvc, err := GenerateOrchardPVC(mint, testScheme(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pvc == nil {
		t.Fatal("expected PVC")
	}
	if actual := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; !actual.Equal(resource.MustParse(mintv1alpha1.DefaultStorageSize)) {
		t.Fatalf("storage request = %s, want %s", actual.String(), mintv1alpha1.DefaultStorageSize)
	}
	if pvc.Spec.StorageClassName != nil {
		t.Fatalf("StorageClassName = %v, want nil", pvc.Spec.StorageClassName)
	}
}

func TestGenerateOrchardPVC_ReturnsControllerReferenceError(t *testing.T) {
	mint := baseMint("orchard-pvc-error")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{Enabled: true}

	pvc, err := GenerateOrchardPVC(mint, runtime.NewScheme())
	if err == nil {
		t.Fatal("expected controller reference error")
	}
	if pvc != nil {
		t.Fatalf("pvc = %+v, want nil on error", pvc)
	}
}

func TestGenerateOrchardService_Disabled(t *testing.T) {
	service, err := GenerateOrchardService(baseMint("orchard-service-disabled"), testScheme(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service != nil {
		t.Fatalf("service = %+v, want nil", service)
	}
}

func TestGenerateOrchardService_ReturnsControllerReferenceError(t *testing.T) {
	mint := baseMint("orchard-service-error")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{Enabled: true}

	service, err := GenerateOrchardService(mint, runtime.NewScheme())
	if err == nil {
		t.Fatal("expected controller reference error")
	}
	if service != nil {
		t.Fatalf("service = %+v, want nil on error", service)
	}
}

func TestGenerateOrchardIngress_ReturnsControllerReferenceError(t *testing.T) {
	mint := baseMint("orchard-ingress-error")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled: true,
		Ingress: &mintv1alpha1.IngressConfig{
			Enabled: true,
			Host:    "orchard.example.com",
		},
	}

	ingress, err := GenerateOrchardIngress(mint, runtime.NewScheme())
	if err == nil {
		t.Fatal("expected controller reference error")
	}
	if ingress != nil {
		t.Fatalf("ingress = %+v, want nil on error", ingress)
	}
}

func TestOrchardPort_DefaultsWhenOrchardMissing(t *testing.T) {
	if port := orchardPort(baseMint("orchard-port-default")); port != orchardPortDefault {
		t.Fatalf("port = %d, want %d", port, orchardPortDefault)
	}
}

func TestOrchardMintAPI_UsesCustomValue(t *testing.T) {
	mint := baseMint("orchard-api")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Mint: &mintv1alpha1.OrchardMintConfig{
			API: "https://mint-api.example.com",
		},
	}

	if api := orchardMintAPI(mint); api != "https://mint-api.example.com" {
		t.Fatalf("api = %q, want custom API", api)
	}
}

func TestOrchardMintRPCPort_UsesManagementRPCPort(t *testing.T) {
	mint := baseMint("orchard-rpc-port-management")
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{Port: 9443}
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Mint: &mintv1alpha1.OrchardMintConfig{
			RPC: &mintv1alpha1.OrchardMintRPCConfig{},
		},
	}

	if port := orchardMintRPCPort(mint); port != 9443 {
		t.Fatalf("port = %d, want 9443", port)
	}
}

func TestOrchardMintRPCPort_UsesOrchardRPCPort(t *testing.T) {
	mint := baseMint("orchard-rpc-port-custom")
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{Port: 9443}
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Mint: &mintv1alpha1.OrchardMintConfig{
			RPC: &mintv1alpha1.OrchardMintRPCConfig{Port: 9555},
		},
	}

	if port := orchardMintRPCPort(mint); port != 9555 {
		t.Fatalf("port = %d, want 9555", port)
	}
}
