package generators

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

const orchardCertificateHost = "orchard.example.com"

func TestGenerateOrchardCertificate_Disabled(t *testing.T) {
	scheme := testScheme(t)

	tests := []struct {
		name string
		mint *mintv1alpha1.CashuMint
	}{
		{
			name: "nil orchard",
			mint: baseMint("orchard-cert-no-config"),
		},
		{
			name: "orchard disabled",
			mint: func() *mintv1alpha1.CashuMint {
				mint := baseMint("orchard-cert-disabled")
				mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{Enabled: false}
				return mint
			}(),
		},
		{
			name: "ingress nil",
			mint: func() *mintv1alpha1.CashuMint {
				mint := baseMint("orchard-cert-no-ingress")
				mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{Enabled: true}
				return mint
			}(),
		},
		{
			name: "ingress disabled",
			mint: func() *mintv1alpha1.CashuMint {
				mint := baseMint("orchard-cert-ingress-disabled")
				mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
					Enabled: true,
					Ingress: &mintv1alpha1.IngressConfig{Enabled: false},
				}
				return mint
			}(),
		},
		{
			name: "tls nil",
			mint: func() *mintv1alpha1.CashuMint {
				mint := baseMint("orchard-cert-no-tls")
				mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
					Enabled: true,
					Ingress: &mintv1alpha1.IngressConfig{Enabled: true, Host: orchardCertificateHost},
				}
				return mint
			}(),
		},
		{
			name: "tls disabled",
			mint: func() *mintv1alpha1.CashuMint {
				mint := baseMint("orchard-cert-tls-disabled")
				mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
					Enabled: true,
					Ingress: &mintv1alpha1.IngressConfig{
						Enabled: true,
						Host:    orchardCertificateHost,
						TLS:     &mintv1alpha1.IngressTLSConfig{Enabled: false},
					},
				}
				return mint
			}(),
		},
		{
			name: "cert-manager nil",
			mint: func() *mintv1alpha1.CashuMint {
				mint := baseMint("orchard-cert-no-cert-manager")
				mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
					Enabled: true,
					Ingress: &mintv1alpha1.IngressConfig{
						Enabled: true,
						Host:    orchardCertificateHost,
						TLS:     &mintv1alpha1.IngressTLSConfig{Enabled: true},
					},
				}
				return mint
			}(),
		},
		{
			name: "cert-manager disabled",
			mint: func() *mintv1alpha1.CashuMint {
				mint := baseMint("orchard-cert-cert-manager-disabled")
				mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
					Enabled: true,
					Ingress: &mintv1alpha1.IngressConfig{
						Enabled: true,
						Host:    orchardCertificateHost,
						TLS: &mintv1alpha1.IngressTLSConfig{
							Enabled:     true,
							CertManager: &mintv1alpha1.CertManagerConfig{Enabled: false},
						},
					},
				}
				return mint
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := GenerateOrchardCertificate(tt.mint, scheme)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cert != nil {
				t.Fatalf("cert = %+v, want nil", cert)
			}
		})
	}
}

func TestGenerateOrchardCertificate_FullConfig(t *testing.T) {
	mint := baseMint("orchard-cert")
	mint.UID = types.UID("orchard-cert-uid")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled: true,
		Ingress: &mintv1alpha1.IngressConfig{
			Enabled: true,
			Host:    orchardCertificateHost,
			Annotations: map[string]string{
				"example.com/custom": trueStr,
			},
			TLS: &mintv1alpha1.IngressTLSConfig{
				Enabled:    true,
				SecretName: "orchard-custom-tls",
				CertManager: &mintv1alpha1.CertManagerConfig{
					Enabled:    true,
					IssuerName: letsencryptProd,
					IssuerKind: "Issuer",
				},
			},
		},
	}

	cert, err := GenerateOrchardCertificate(mint, testScheme(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected Certificate")
	}
	if cert.Name != orchardResourceName(mint) {
		t.Fatalf("name = %q, want %q", cert.Name, orchardResourceName(mint))
	}
	if cert.Spec.SecretName != "orchard-custom-tls" {
		t.Fatalf("secretName = %q, want orchard-custom-tls", cert.Spec.SecretName)
	}
	if len(cert.Spec.DNSNames) != 1 || cert.Spec.DNSNames[0] != orchardCertificateHost {
		t.Fatalf("dnsNames = %v, want %s", cert.Spec.DNSNames, orchardCertificateHost)
	}
	if cert.Spec.IssuerRef.Name != letsencryptProd || cert.Spec.IssuerRef.Kind != "Issuer" {
		t.Fatalf("issuerRef = %+v, want %s/Issuer", cert.Spec.IssuerRef, letsencryptProd)
	}
	assertLabelsContain(t, cert.Labels, "app.kubernetes.io/component", orchardStr)
	if cert.Annotations["example.com/custom"] != trueStr {
		t.Fatalf("annotations = %+v, want custom annotation", cert.Annotations)
	}
	if len(cert.OwnerReferences) != 1 || cert.OwnerReferences[0].Name != mint.Name {
		t.Fatalf("ownerReferences = %+v, want owner %q", cert.OwnerReferences, mint.Name)
	}
}

func TestGenerateOrchardCertificate_Defaults(t *testing.T) {
	mint := baseMint("orchard-cert-defaults")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled: true,
		Ingress: &mintv1alpha1.IngressConfig{
			Enabled: true,
			Host:    orchardCertificateHost,
			TLS: &mintv1alpha1.IngressTLSConfig{
				Enabled: true,
				CertManager: &mintv1alpha1.CertManagerConfig{
					Enabled:    true,
					IssuerName: letsencryptProd,
				},
			},
		},
	}

	cert, err := GenerateOrchardCertificate(mint, testScheme(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected Certificate")
	}
	if cert.Spec.SecretName != orchardResourceName(mint)+"-tls" {
		t.Fatalf("secretName = %q, want %q", cert.Spec.SecretName, orchardResourceName(mint)+"-tls")
	}
	if cert.Spec.IssuerRef.Kind != mintv1alpha1.DefaultClusterIssuerKind {
		t.Fatalf("issuerKind = %q, want %q", cert.Spec.IssuerRef.Kind, mintv1alpha1.DefaultClusterIssuerKind)
	}
}

func TestGenerateOrchardCertificate_ReturnsControllerReferenceError(t *testing.T) {
	mint := baseMint("orchard-cert-error")
	mint.Spec.Orchard = &mintv1alpha1.OrchardConfig{
		Enabled: true,
		Ingress: &mintv1alpha1.IngressConfig{
			Enabled: true,
			Host:    orchardCertificateHost,
			TLS: &mintv1alpha1.IngressTLSConfig{
				Enabled: true,
				CertManager: &mintv1alpha1.CertManagerConfig{
					Enabled:    true,
					IssuerName: letsencryptProd,
				},
			},
		},
	}

	cert, err := GenerateOrchardCertificate(mint, runtime.NewScheme())
	if err == nil {
		t.Fatal("expected controller reference error")
	}
	if cert != nil {
		t.Fatalf("cert = %+v, want nil on error", cert)
	}
}
