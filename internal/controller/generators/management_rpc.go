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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

const managementRPCTLSVolumeName = "management-rpc-tls"

func GenerateManagementRPCTLSSecret(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*corev1.Secret, error) {
	if !mintv1alpha1.ManagementRPCTLSEnabled(&mint.Spec) {
		return nil, nil
	}

	bundle, err := generateManagementRPCTLSBundle()
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mintv1alpha1.ManagementRPCTLSSecretName(&mint.Spec, mint.Name),
			Namespace: mint.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "cashu-mint",
				"app.kubernetes.io/instance":   mint.Name,
				"app.kubernetes.io/component":  "management-rpc-tls",
				"app.kubernetes.io/managed-by": "cashu-operator",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: bundle,
	}

	if err := controllerutil.SetControllerReference(mint, secret, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return secret, nil
}

func generateManagementRPCTLSBundle() (map[string][]byte, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate management RPC CA key: %w", err)
	}

	now := time.Now().UTC()
	caSerial, err := newTLSSerialNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to generate management RPC CA serial number: %w", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			CommonName:   "cashu-management-rpc-ca",
			Organization: []string{"cashu-operator"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(3650 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create management RPC CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse management RPC CA certificate: %w", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate management RPC server key: %w", err)
	}
	serverSerial, err := newTLSSerialNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to generate management RPC server serial number: %w", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			CommonName:   "cashu-management-rpc-server",
			Organization: []string{"cashu-operator"},
		},
		NotBefore:   now.Add(-5 * time.Minute),
		NotAfter:    now.Add(825 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP(mintv1alpha1.DefaultLoopbackHost)},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create management RPC server certificate: %w", err)
	}

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate management RPC client key: %w", err)
	}
	clientSerial, err := newTLSSerialNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to generate management RPC client serial number: %w", err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: clientSerial,
		Subject: pkix.Name{
			CommonName:   "cashu-management-rpc-client",
			Organization: []string{"cashu-operator"},
		},
		NotBefore:   now.Add(-5 * time.Minute),
		NotAfter:    now.Add(825 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create management RPC client certificate: %w", err)
	}

	return map[string][]byte{
		"ca.pem":     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		"server.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}),
		"server.key": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}),
		"client.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}),
		"client.key": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}),
	}, nil
}

func newTLSSerialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, err
	}
	return serial, nil
}
