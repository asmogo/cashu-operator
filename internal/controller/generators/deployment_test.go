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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGenerateDeployment_Defaults(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("deploy-mint")

	dep, err := GenerateDeployment(mint, "hash123", scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dep.Name != "deploy-mint" {
		t.Errorf("name = %q, want %q", dep.Name, "deploy-mint")
	}
	if *dep.Spec.Replicas != 1 {
		t.Errorf("replicas = %d, want 1", *dep.Spec.Replicas)
	}
	if dep.Spec.Template.Annotations["config-hash"] != "hash123" {
		t.Errorf("config-hash annotation = %q, want %q", dep.Spec.Template.Annotations["config-hash"], "hash123")
	}
	assertLabelsContain(t, dep.Labels, "app.kubernetes.io/instance", "deploy-mint")
}

func TestGenerateDeployment_CustomReplicas(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("custom-rep")
	mint.Spec.Replicas = int32Ptr(1)

	dep, _ := GenerateDeployment(mint, "h", scheme)
	if *dep.Spec.Replicas != 1 {
		t.Errorf("replicas = %d, want 1", *dep.Spec.Replicas)
	}
}

func TestGenerateDeployment_DefaultImage(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("img-mint")

	dep, _ := GenerateDeployment(mint, "h", scheme)
	containers := dep.Spec.Template.Spec.Containers
	var mintd *corev1.Container
	for i := range containers {
		if containers[i].Name == "mintd" {
			mintd = &containers[i]
		}
	}
	if mintd == nil {
		t.Fatal("mintd container not found")
	}
	if mintd.Image != "ghcr.io/cashubtc/cdk-mintd:latest" {
		t.Errorf("image = %q, want default", mintd.Image)
	}
	if mintd.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("pullPolicy = %v, want IfNotPresent", mintd.ImagePullPolicy)
	}
}

func TestGenerateDeployment_CustomImage(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("custom-img")
	mint.Spec.Image = "my-registry/mintd:v1.0"
	mint.Spec.ImagePullPolicy = corev1.PullAlways

	dep, _ := GenerateDeployment(mint, "h", scheme)
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	if mintd.Image != "my-registry/mintd:v1.0" {
		t.Errorf("image = %q, want custom", mintd.Image)
	}
	if mintd.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("pullPolicy = %v, want Always", mintd.ImagePullPolicy)
	}
}

func TestGenerateDeployment_Probes(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("probe-mint")

	dep, _ := GenerateDeployment(mint, "h", scheme)
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	if mintd.LivenessProbe == nil {
		t.Error("expected liveness probe")
	}
	if mintd.ReadinessProbe == nil {
		t.Error("expected readiness probe")
	}
	if mintd.LivenessProbe.HTTPGet.Path != "/v1/info" {
		t.Errorf("liveness path = %q, want /v1/info", mintd.LivenessProbe.HTTPGet.Path)
	}
}

func TestGenerateDeployment_DefaultPort(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("port-mint")

	dep, _ := GenerateDeployment(mint, "h", scheme)
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	if mintd.Ports[0].ContainerPort != 8085 {
		t.Errorf("port = %d, want 8085", mintd.Ports[0].ContainerPort)
	}
}

func TestGenerateDeployment_PrometheusPorts(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("prom-mint")
	mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true, Port: int32Ptr(9090)}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	if len(mintd.Ports) != 2 {
		t.Fatalf("ports count = %d, want 2", len(mintd.Ports))
	}
	found := false
	for _, p := range mintd.Ports {
		if p.Name == "metrics" && p.ContainerPort == 9090 {
			found = true
		}
	}
	if !found {
		t.Error("metrics port not found")
	}
}

func TestGenerateDeployment_DefaultResources(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("res-mint")

	dep, _ := GenerateDeployment(mint, "h", scheme)
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	cpuReq := mintd.Resources.Requests[corev1.ResourceCPU]
	if !cpuReq.Equal(resource.MustParse("100m")) {
		t.Errorf("default cpu request = %s, want 100m", cpuReq.String())
	}
}

func TestGenerateDeployment_CustomResources(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("cust-res")
	mint.Spec.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
	}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	if !mintd.Resources.Requests[corev1.ResourceCPU].Equal(resource.MustParse("500m")) {
		t.Error("custom resources not applied")
	}
}

func TestGenerateDeployment_DefaultSecurityContext(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("sec-mint")

	dep, _ := GenerateDeployment(mint, "h", scheme)
	podSec := dep.Spec.Template.Spec.SecurityContext
	if podSec == nil || podSec.RunAsNonRoot == nil || !*podSec.RunAsNonRoot {
		t.Error("expected RunAsNonRoot=true in pod security context")
	}
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	if mintd.SecurityContext == nil || *mintd.SecurityContext.AllowPrivilegeEscalation {
		t.Error("expected AllowPrivilegeEscalation=false")
	}
}

func TestGenerateDeployment_CustomSecurityContext(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("cust-sec")
	mint.Spec.PodSecurityContext = &corev1.PodSecurityContext{RunAsUser: int64Ptr(2000)}
	mint.Spec.ContainerSecurityContext = &corev1.SecurityContext{ReadOnlyRootFilesystem: boolPtr(true)}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	if *dep.Spec.Template.Spec.SecurityContext.RunAsUser != 2000 {
		t.Error("custom pod security context not applied")
	}
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	if !*mintd.SecurityContext.ReadOnlyRootFilesystem {
		t.Error("custom container security context not applied")
	}
}

func TestGenerateDeployment_EnvVars(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("env-mint")
	mint.Spec.Logging = &mintv1alpha1.LoggingConfig{Level: "debug", Format: "json"}
	mint.Spec.MintInfo.MnemonicSecretRef = &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "mnemonic-secret"},
		Key:                  "mnemonic",
	}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	envMap := envVarMap(mintd.Env)

	if envMap["CDK_MINTD_WORK_DIR"] != "/data" {
		t.Error("CDK_MINTD_WORK_DIR missing or wrong")
	}
	if envMap["HOME"] != "/data" {
		t.Error("HOME missing or wrong")
	}
	if envMap["RUST_LOG"] != "debug" {
		t.Error("RUST_LOG missing or wrong")
	}
	if envMap["LOG_LEVEL"] != "debug" {
		t.Error("LOG_LEVEL missing or wrong")
	}
	if envMap["LOG_FORMAT"] != "json" {
		t.Error("LOG_FORMAT missing or wrong")
	}

	// Mnemonic should be from secret
	found := false
	for _, e := range mintd.Env {
		if e.Name == "CDK_MINTD_MNEMONIC" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			found = true
		}
	}
	if !found {
		t.Error("CDK_MINTD_MNEMONIC secret ref missing")
	}
}

func TestGenerateDeployment_PostgresURLSecretRef(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("pg-env")
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine: "postgres",
		Postgres: &mintv1alpha1.PostgresConfig{
			URLSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "db-secret"},
				Key:                  "url",
			},
		},
	}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	mintd := findContainer(dep.Spec.Template.Spec.Containers, "mintd")
	found := false
	for _, e := range mintd.Env {
		if e.Name == "CDK_MINTD_DATABASE_URL" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef.Name == "db-secret" {
			found = true
		}
	}
	if !found {
		t.Error("CDK_MINTD_DATABASE_URL secret ref missing")
	}
}

func TestGenerateDeployment_Volumes_SQLite(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("vol-sqlite")

	dep, _ := GenerateDeployment(mint, "h", scheme)
	volumes := dep.Spec.Template.Spec.Volumes

	foundConfig := false
	foundPVC := false
	for _, v := range volumes {
		if v.Name == "config" && v.ConfigMap != nil {
			foundConfig = true
		}
		if v.Name == "data" && v.PersistentVolumeClaim != nil {
			foundPVC = true
		}
	}
	if !foundConfig {
		t.Error("config volume missing")
	}
	if !foundPVC {
		t.Error("PVC data volume missing for sqlite")
	}
}

func TestGenerateDeployment_Volumes_Postgres(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("vol-pg")
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine:   "postgres",
		Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: true},
	}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "data" {
			if v.EmptyDir == nil {
				t.Error("expected emptyDir for postgres data volume")
			}
			return
		}
	}
	t.Error("data volume not found")
}

func TestGenerateDeployment_SidecarProcessor(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("sidecar")
	mint.Spec.PaymentBackend = mintv1alpha1.PaymentBackendConfig{
		GRPCProcessor: &mintv1alpha1.GRPCProcessorConfig{
			Port: 50051,
			SidecarProcessor: &mintv1alpha1.SidecarProcessorConfig{
				Enabled:    true,
				Image:      "processor:latest",
				WorkingDir: "/work",
				Command:    []string{"./processor"},
				Args:       []string{"--port", "50051"},
			},
		},
	}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	sidecar := findContainer(dep.Spec.Template.Spec.Containers, "grpc-processor")
	if sidecar.Image != "processor:latest" {
		t.Errorf("sidecar image = %q, want processor:latest", sidecar.Image)
	}
	if sidecar.Ports[0].ContainerPort != 50051 {
		t.Errorf("sidecar port = %d, want 50051", sidecar.Ports[0].ContainerPort)
	}
}

func TestGenerateDeployment_LDKSidecar(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("ldk-deploy")
	mint.Spec.LDKNode = &mintv1alpha1.LDKNodeConfig{
		Enabled: true, Port: 8090, WebserverPort: 8888,
	}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	ldk := findContainer(dep.Spec.Template.Spec.Containers, "ldk-node")
	if ldk.Image == "" {
		t.Error("ldk-node container should have default image")
	}
}

func TestGenerateDeployment_ImagePullSecrets(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("ips-mint")
	mint.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "my-registry-cred"}}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	if len(dep.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Fatal("expected 1 image pull secret")
	}
	if dep.Spec.Template.Spec.ImagePullSecrets[0].Name != "my-registry-cred" {
		t.Error("image pull secret name mismatch")
	}
}

func TestGenerateDeployment_NodeSelectorAndTolerations(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("sched-mint")
	mint.Spec.NodeSelector = map[string]string{"node-type": "gpu"}
	mint.Spec.Tolerations = []corev1.Toleration{{Key: "special", Operator: corev1.TolerationOpExists}}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	if dep.Spec.Template.Spec.NodeSelector["node-type"] != "gpu" {
		t.Error("node selector not applied")
	}
	if len(dep.Spec.Template.Spec.Tolerations) != 1 {
		t.Error("tolerations not applied")
	}
}

func TestGenerateDeployment_OwnerReference(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "ref-dep", Namespace: "default", UID: "test-uid"},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo:       mintv1alpha1.MintInfo{URL: "http://test.local"},
			Database:       mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
		},
	}

	dep, _ := GenerateDeployment(mint, "h", scheme)
	if len(dep.OwnerReferences) == 0 {
		t.Error("expected owner reference")
	}
}

// findContainer finds a container by name in a slice.
func findContainer(containers []corev1.Container, name string) corev1.Container {
	for _, c := range containers {
		if c.Name == name {
			return c
		}
	}
	return corev1.Container{}
}

// envVarMap converts env vars to a name->value map (only for plain values).
func envVarMap(envs []corev1.EnvVar) map[string]string {
	m := map[string]string{}
	for _, e := range envs {
		m[e.Name] = e.Value
	}
	return m
}
