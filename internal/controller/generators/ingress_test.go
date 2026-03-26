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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGenerateIngress_Disabled(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("no-ingress")

	ingress, err := GenerateIngress(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ingress != nil {
		t.Errorf("expected nil for disabled ingress, got %v", ingress)
	}
}

func TestGenerateIngress_DisabledExplicit(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("disabled-ingress")
	mint.Spec.Ingress = &mintv1alpha1.IngressConfig{Enabled: false}

	ingress, err := GenerateIngress(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ingress != nil {
		t.Errorf("expected nil for disabled ingress, got %v", ingress)
	}
}

func TestGenerateIngress_Basic(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("basic-ingress")
	mint.Spec.Ingress = &mintv1alpha1.IngressConfig{
		Enabled: true,
		Host:    "mint.example.com",
	}

	ingress, err := GenerateIngress(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ingress == nil {
		t.Fatal("expected Ingress, got nil")
	}
	if ingress.Name != "basic-ingress" {
		t.Errorf("name = %q, want %q", ingress.Name, "basic-ingress")
	}
	if ingress.Spec.IngressClassName == nil || *ingress.Spec.IngressClassName != "nginx" {
		t.Errorf("ingressClassName = %v, want 'nginx'", ingress.Spec.IngressClassName)
	}
	if len(ingress.Spec.Rules) != 1 {
		t.Fatalf("rules count = %d, want 1", len(ingress.Spec.Rules))
	}
	if ingress.Spec.Rules[0].Host != "mint.example.com" {
		t.Errorf("host = %q, want %q", ingress.Spec.Rules[0].Host, "mint.example.com")
	}
	// Default annotations
	if ingress.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"] != "true" {
		t.Error("missing default ssl-redirect annotation")
	}
}

func TestGenerateIngress_CustomClassName(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("custom-class")
	mint.Spec.Ingress = &mintv1alpha1.IngressConfig{
		Enabled:   true,
		Host:      "mint.example.com",
		ClassName: "traefik",
	}

	ingress, _ := GenerateIngress(mint, scheme)
	if *ingress.Spec.IngressClassName != "traefik" {
		t.Errorf("ingressClassName = %v, want 'traefik'", *ingress.Spec.IngressClassName)
	}
}

func TestGenerateIngress_TLSEnabled(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("tls-ingress")
	mint.Spec.Ingress = &mintv1alpha1.IngressConfig{
		Enabled: true,
		Host:    "mint.example.com",
		TLS: &mintv1alpha1.IngressTLSConfig{
			Enabled:    true,
			SecretName: "my-tls-secret",
		},
	}

	ingress, _ := GenerateIngress(mint, scheme)
	if len(ingress.Spec.TLS) != 1 {
		t.Fatalf("TLS count = %d, want 1", len(ingress.Spec.TLS))
	}
	if ingress.Spec.TLS[0].SecretName != "my-tls-secret" {
		t.Errorf("TLS secret = %q, want %q", ingress.Spec.TLS[0].SecretName, "my-tls-secret")
	}
	if len(ingress.Spec.TLS[0].Hosts) != 1 || ingress.Spec.TLS[0].Hosts[0] != "mint.example.com" {
		t.Errorf("TLS hosts = %v, want [mint.example.com]", ingress.Spec.TLS[0].Hosts)
	}
}

func TestGenerateIngress_TLSDefaultSecretName(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("auto-tls")
	mint.Spec.Ingress = &mintv1alpha1.IngressConfig{
		Enabled: true,
		Host:    "mint.example.com",
		TLS:     &mintv1alpha1.IngressTLSConfig{Enabled: true},
	}

	ingress, _ := GenerateIngress(mint, scheme)
	if ingress.Spec.TLS[0].SecretName != "auto-tls-tls" {
		t.Errorf("TLS secret = %q, want %q", ingress.Spec.TLS[0].SecretName, "auto-tls-tls")
	}
}

func TestGenerateIngress_CertManagerAnnotations(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("certmgr-ing")
	mint.Spec.Ingress = &mintv1alpha1.IngressConfig{
		Enabled: true,
		Host:    "mint.example.com",
		TLS: &mintv1alpha1.IngressTLSConfig{
			Enabled: true,
			CertManager: &mintv1alpha1.CertManagerConfig{
				Enabled:    true,
				IssuerName: "letsencrypt-prod",
				IssuerKind: "ClusterIssuer",
			},
		},
	}

	ingress, _ := GenerateIngress(mint, scheme)
	if ingress.Annotations["cert-manager.io/issuer"] != "letsencrypt-prod" {
		t.Errorf("cert-manager issuer annotation missing")
	}
	if ingress.Annotations["cert-manager.io/issuer-kind"] != "ClusterIssuer" {
		t.Errorf("cert-manager issuer-kind annotation missing")
	}
}

func TestGenerateIngress_CustomAnnotationsOverride(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "custom-ann", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo:       mintv1alpha1.MintInfo{URL: "http://test.local"},
			Database:       mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
			Ingress: &mintv1alpha1.IngressConfig{
				Enabled: true,
				Host:    "mint.example.com",
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/ssl-redirect": "false",
					"custom-key": "custom-value",
				},
			},
		},
	}

	ingress, _ := GenerateIngress(mint, scheme)
	// User annotation should override default
	if ingress.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"] != "false" {
		t.Error("user annotation should override default")
	}
	if ingress.Annotations["custom-key"] != "custom-value" {
		t.Error("custom annotation missing")
	}
}

func TestGenerateIngress_CustomPort(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("port-ingress")
	mint.Spec.MintInfo.ListenPort = 9999
	mint.Spec.Ingress = &mintv1alpha1.IngressConfig{Enabled: true, Host: "mint.example.com"}

	ingress, _ := GenerateIngress(mint, scheme)
	backend := ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	if backend.Port.Number != 9999 {
		t.Errorf("backend port = %d, want 9999", backend.Port.Number)
	}
}
