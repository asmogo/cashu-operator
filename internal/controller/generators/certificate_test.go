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

const letsencryptProd = "letsencrypt-prod"

func TestGenerateCertificate_Disabled(t *testing.T) {
	scheme := testScheme(t)

	tests := []struct {
		name string
		mint *mintv1alpha1.CashuMint
	}{
		{
			name: "nil ingress",
			mint: baseMint("no-ing"),
		},
		{
			name: "ingress disabled",
			mint: func() *mintv1alpha1.CashuMint {
				m := baseMint("disabled-ing")
				m.Spec.Ingress = &mintv1alpha1.IngressConfig{Enabled: false}
				return m
			}(),
		},
		{
			name: "TLS nil",
			mint: func() *mintv1alpha1.CashuMint {
				m := baseMint("no-tls")
				m.Spec.Ingress = &mintv1alpha1.IngressConfig{Enabled: true, Host: "h.com"}
				return m
			}(),
		},
		{
			name: "TLS disabled",
			mint: func() *mintv1alpha1.CashuMint {
				m := baseMint("tls-off")
				m.Spec.Ingress = &mintv1alpha1.IngressConfig{
					Enabled: true, Host: "h.com",
					TLS: &mintv1alpha1.IngressTLSConfig{Enabled: false},
				}
				return m
			}(),
		},
		{
			name: "cert-manager nil",
			mint: func() *mintv1alpha1.CashuMint {
				m := baseMint("no-cm")
				m.Spec.Ingress = &mintv1alpha1.IngressConfig{
					Enabled: true, Host: "h.com",
					TLS: &mintv1alpha1.IngressTLSConfig{Enabled: true},
				}
				return m
			}(),
		},
		{
			name: "cert-manager disabled",
			mint: func() *mintv1alpha1.CashuMint {
				m := baseMint("cm-off")
				m.Spec.Ingress = &mintv1alpha1.IngressConfig{
					Enabled: true, Host: "h.com",
					TLS: &mintv1alpha1.IngressTLSConfig{
						Enabled:     true,
						CertManager: &mintv1alpha1.CertManagerConfig{Enabled: false},
					},
				}
				return m
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := GenerateCertificate(tt.mint, scheme)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cert != nil {
				t.Errorf("expected nil Certificate, got %v", cert)
			}
		})
	}
}

func TestGenerateCertificate_FullConfig(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-mint", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo:       mintv1alpha1.MintInfo{URL: "http://test.local"},
			Database:       mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
			Ingress: &mintv1alpha1.IngressConfig{
				Enabled: true,
				Host:    "mint.example.com",
				TLS: &mintv1alpha1.IngressTLSConfig{
					Enabled:    true,
					SecretName: "custom-tls-secret",
					CertManager: &mintv1alpha1.CertManagerConfig{
						Enabled:    true,
						IssuerName: letsencryptProd,
						IssuerKind: "Issuer",
					},
				},
			},
		},
	}

	cert, err := GenerateCertificate(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected Certificate, got nil")
	}
	if cert.Name != "cert-mint" {
		t.Errorf("name = %q, want %q", cert.Name, "cert-mint")
	}
	if cert.Spec.SecretName != "custom-tls-secret" {
		t.Errorf("secretName = %q, want %q", cert.Spec.SecretName, "custom-tls-secret")
	}
	if len(cert.Spec.DNSNames) != 1 || cert.Spec.DNSNames[0] != "mint.example.com" {
		t.Errorf("dnsNames = %v, want [mint.example.com]", cert.Spec.DNSNames)
	}
	if cert.Spec.IssuerRef.Name != letsencryptProd {
		t.Errorf("issuerRef.name = %q, want %q", cert.Spec.IssuerRef.Name, letsencryptProd)
	}
	if cert.Spec.IssuerRef.Kind != "Issuer" {
		t.Errorf("issuerRef.kind = %q, want %q", cert.Spec.IssuerRef.Kind, "Issuer")
	}
}

func TestGenerateCertificate_DefaultIssuerKind(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "def-cert", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo:       mintv1alpha1.MintInfo{URL: "http://test.local"},
			Database:       mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
			Ingress: &mintv1alpha1.IngressConfig{
				Enabled: true, Host: "mint.local",
				TLS: &mintv1alpha1.IngressTLSConfig{
					Enabled: true,
					CertManager: &mintv1alpha1.CertManagerConfig{
						Enabled:    true,
						IssuerName: "my-issuer",
					},
				},
			},
		},
	}

	cert, _ := GenerateCertificate(mint, scheme)
	if cert.Spec.IssuerRef.Kind != "ClusterIssuer" {
		t.Errorf("default issuerKind = %q, want ClusterIssuer", cert.Spec.IssuerRef.Kind)
	}
	// Default secret name
	if cert.Spec.SecretName != "def-cert-tls" {
		t.Errorf("default secret = %q, want %q", cert.Spec.SecretName, "def-cert-tls")
	}
}
