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
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// GenerateCertificate creates a Certificate for the Cashu mint
func GenerateCertificate(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*certmanagerv1.Certificate, error) {
	if mint.Spec.Ingress == nil || !mint.Spec.Ingress.Enabled {
		return nil, nil
	}

	// Only generate certificate if TLS is enabled and cert-manager is configured
	if mint.Spec.Ingress.TLS == nil || !mint.Spec.Ingress.TLS.Enabled ||
		mint.Spec.Ingress.TLS.CertManager == nil || !mint.Spec.Ingress.TLS.CertManager.Enabled {
		return nil, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	secretName := mint.Spec.Ingress.TLS.SecretName
	if secretName == "" {
		secretName = mint.Name + "-tls"
	}

	issuerKind := mint.Spec.Ingress.TLS.CertManager.IssuerKind
	if issuerKind == "" {
		issuerKind = "ClusterIssuer"
	}

	// Default duration: 90 days (2160h)
	duration := 2160 * time.Hour
	// Default renewBefore: 15 days (360h)
	renewBefore := 360 * time.Hour

	cert := &certmanagerv1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cert-manager.io/v1",
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        mint.Name,
			Namespace:   mint.Namespace,
			Labels:      labels,
			Annotations: mint.Spec.Ingress.Annotations,
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: secretName,
			DNSNames:   []string{mint.Spec.Ingress.Host},
			IssuerRef: certmanagermeta.ObjectReference{
				Name:  mint.Spec.Ingress.TLS.CertManager.IssuerName,
				Kind:  issuerKind,
				Group: "cert-manager.io",
			},
			Duration:    &metav1.Duration{Duration: duration},
			RenewBefore: &metav1.Duration{Duration: renewBefore},
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
			},
		},
	}

	if err := controllerutil.SetControllerReference(mint, cert, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return cert, nil
}
