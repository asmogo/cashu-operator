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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGenerateManagementRPCTLSSecret_Disabled(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("management-rpc-disabled")
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{Enabled: true}

	secret, err := GenerateManagementRPCTLSSecret(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != nil {
		t.Fatalf("secret = %+v, want nil", secret)
	}
}

func TestGenerateManagementRPCTLSSecret_CreatesBundle(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("management-rpc-enabled")
	mint.UID = types.UID("management-rpc-enabled-uid")
	mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{
		Enabled:      true,
		TLSSecretRef: &corev1.LocalObjectReference{Name: "custom-management-rpc-tls"},
	}

	secret, err := GenerateManagementRPCTLSSecret(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret == nil {
		t.Fatal("expected Secret")
	}
	if secret.Name != "custom-management-rpc-tls" {
		t.Fatalf("name = %q, want custom-management-rpc-tls", secret.Name)
	}
	assertLabelsContain(t, secret.Labels, "app.kubernetes.io/instance", mint.Name)
	assertLabelsContain(t, secret.Labels, "app.kubernetes.io/component", "management-rpc-tls")
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].Name != mint.Name {
		t.Fatalf("unexpected owner refs: %+v", secret.OwnerReferences)
	}
	for _, key := range []string{"ca.pem", "server.pem", "server.key", "client.pem", "client.key"} {
		if _, ok := secret.Data[key]; !ok {
			t.Fatalf("expected secret data to contain %q", key)
		}
	}

	caCert := mustParseCertificatePEM(t, secret.Data["ca.pem"])
	serverCert := mustParseCertificatePEM(t, secret.Data["server.pem"])
	clientCert := mustParseCertificatePEM(t, secret.Data["client.pem"])
	serverKey := mustParseRSAPrivateKeyPEM(t, secret.Data["server.key"])
	clientKey := mustParseRSAPrivateKeyPEM(t, secret.Data["client.key"])

	if !caCert.IsCA {
		t.Fatal("expected CA certificate")
	}
	if err := caCert.CheckSignatureFrom(caCert); err != nil {
		t.Fatalf("expected self-signed CA certificate: %v", err)
	}
	if err := serverCert.CheckSignatureFrom(caCert); err != nil {
		t.Fatalf("expected server certificate to be signed by CA: %v", err)
	}
	if err := clientCert.CheckSignatureFrom(caCert); err != nil {
		t.Fatalf("expected client certificate to be signed by CA: %v", err)
	}
	if serverKey.N.BitLen() != 2048 {
		t.Fatalf("server key size = %d, want 2048", serverKey.N.BitLen())
	}
	if clientKey.N.BitLen() != 2048 {
		t.Fatalf("client key size = %d, want 2048", clientKey.N.BitLen())
	}
	if len(serverCert.DNSNames) != 1 || serverCert.DNSNames[0] != "localhost" {
		t.Fatalf("server DNS names = %v, want [localhost]", serverCert.DNSNames)
	}
	if len(serverCert.IPAddresses) != 1 || serverCert.IPAddresses[0].String() != mintv1alpha1.DefaultLoopbackHost {
		t.Fatalf("server IPs = %v, want [%s]", serverCert.IPAddresses, mintv1alpha1.DefaultLoopbackHost)
	}
	if len(serverCert.ExtKeyUsage) != 1 || serverCert.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Fatalf("server ext key usage = %v, want ServerAuth", serverCert.ExtKeyUsage)
	}
	if len(clientCert.ExtKeyUsage) != 1 || clientCert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Fatalf("client ext key usage = %v, want ClientAuth", clientCert.ExtKeyUsage)
	}
}

func TestNewTLSSerialNumber(t *testing.T) {
	first, err := newTLSSerialNumber()
	if err != nil {
		t.Fatalf("unexpected error generating first serial: %v", err)
	}
	second, err := newTLSSerialNumber()
	if err != nil {
		t.Fatalf("unexpected error generating second serial: %v", err)
	}

	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	if first.Cmp(limit) >= 0 {
		t.Fatalf("first serial bit length exceeds 128 bits: %s", first.String())
	}
	if second.Cmp(limit) >= 0 {
		t.Fatalf("second serial bit length exceeds 128 bits: %s", second.String())
	}
	if first.Cmp(second) == 0 {
		t.Fatal("expected independently generated serial numbers to differ")
	}
}

func mustParseCertificatePEM(t *testing.T, data []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}
	return cert
}

func mustParseRSAPrivateKeyPEM(t *testing.T, data []byte) *rsa.PrivateKey {
	t.Helper()
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("failed to decode private key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse RSA private key: %v", err)
	}
	return key
}
