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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGenerateService_Default(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("svc-mint")

	svc, err := GenerateService(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Name != "svc-mint" {
		t.Errorf("name = %q, want %q", svc.Name, "svc-mint")
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("type = %v, want ClusterIP", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("ports count = %d, want 1", len(svc.Spec.Ports))
	}
	if svc.Spec.Ports[0].Name != "api" {
		t.Errorf("port name = %q, want %q", svc.Spec.Ports[0].Name, "api")
	}
	if svc.Spec.Ports[0].Port != 8085 {
		t.Errorf("port = %d, want 8085", svc.Spec.Ports[0].Port)
	}
}

func TestGenerateService_CustomPortAndType(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("custom-svc")
	mint.Spec.MintInfo.ListenPort = 9090
	mint.Spec.Service = &mintv1alpha1.ServiceConfig{
		Type:           corev1.ServiceTypeLoadBalancer,
		LoadBalancerIP: "1.2.3.4",
		Annotations:    map[string]string{"custom": "annotation"},
	}

	svc, err := GenerateService(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("type = %v, want LoadBalancer", svc.Spec.Type)
	}
	if svc.Spec.Ports[0].Port != 9090 {
		t.Errorf("port = %d, want 9090", svc.Spec.Ports[0].Port)
	}
	if svc.Spec.LoadBalancerIP != "1.2.3.4" {
		t.Errorf("loadBalancerIP = %q, want %q", svc.Spec.LoadBalancerIP, "1.2.3.4")
	}
	if svc.Annotations["custom"] != "annotation" {
		t.Errorf("annotation missing")
	}
}

func TestGenerateService_LDKNodePorts(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("ldk-svc")
	mint.Spec.LDKNode = &mintv1alpha1.LDKNodeConfig{
		Enabled:       true,
		Port:          8090,
		WebserverPort: 8888,
	}

	svc, err := GenerateService(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// api + ldk + ldk-webserver = 3 ports
	if len(svc.Spec.Ports) != 3 {
		t.Fatalf("ports count = %d, want 3", len(svc.Spec.Ports))
	}

	portNames := map[string]bool{}
	for _, p := range svc.Spec.Ports {
		portNames[p.Name] = true
	}
	for _, name := range []string{"api", "ldk", "ldk-webserver"} {
		if !portNames[name] {
			t.Errorf("missing port %q", name)
		}
	}
}

func TestGenerateService_ManagementRPCPort(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("mgmt-svc")
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{
		Enabled: true,
		Port:    8086,
	}

	svc, err := GenerateService(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.Spec.Ports) != 2 {
		t.Fatalf("ports count = %d, want 2", len(svc.Spec.Ports))
	}

	found := false
	for _, p := range svc.Spec.Ports {
		if p.Name == "management" {
			found = true
			if p.Port != 8086 {
				t.Errorf("management port = %d, want 8086", p.Port)
			}
		}
	}
	if !found {
		t.Error("management port not found")
	}
}

func TestGenerateService_OwnerReference(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "ref-svc", Namespace: "default", UID: "test-uid"},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo:       mintv1alpha1.MintInfo{URL: "http://test.local"},
			Database:       mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
		},
	}

	svc, err := GenerateService(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.OwnerReferences) == 0 {
		t.Error("expected owner reference to be set")
	}
}
