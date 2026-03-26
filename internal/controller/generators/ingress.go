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

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// GenerateIngress creates an Ingress for the Cashu mint
func GenerateIngress(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*networkingv1.Ingress, error) {
	if mint.Spec.Ingress == nil || !mint.Spec.Ingress.Enabled {
		return nil, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	pathTypePrefix := networkingv1.PathTypePrefix
	ingressClassName := mint.Spec.Ingress.ClassName
	if ingressClassName == "" {
		ingressClassName = "nginx"
	}

	// Default annotations for nginx ingress
	annotations := map[string]string{
		"nginx.ingress.kubernetes.io/ssl-redirect":      "true",
		"nginx.ingress.kubernetes.io/backend-protocol":  "HTTP",
		"nginx.ingress.kubernetes.io/enable-cors":       "true",
		"nginx.ingress.kubernetes.io/cors-allow-origin": "*",
	}

	// Add cert-manager annotations if enabled
	if mint.Spec.Ingress.TLS != nil && mint.Spec.Ingress.TLS.Enabled &&
		mint.Spec.Ingress.TLS.CertManager != nil && mint.Spec.Ingress.TLS.CertManager.Enabled {

		issuerKind := mint.Spec.Ingress.TLS.CertManager.IssuerKind
		if issuerKind == "" {
			const defaultClusterIssuer = "ClusterIssuer"
			issuerKind = defaultClusterIssuer
		}

		annotations["cert-manager.io/issuer"] = mint.Spec.Ingress.TLS.CertManager.IssuerName
		annotations["cert-manager.io/issuer-kind"] = issuerKind
	}

	// Merge custom annotations (user annotations override defaults)
	if mint.Spec.Ingress.Annotations != nil {
		for k, v := range mint.Spec.Ingress.Annotations {
			annotations[k] = v
		}
	}

	listenPort := mint.Spec.MintInfo.ListenPort
	if listenPort == 0 {
		listenPort = 8085
	}

	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        mint.Name,
			Namespace:   mint.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: mint.Spec.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathTypePrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: mint.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: listenPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Add TLS configuration if enabled
	if mint.Spec.Ingress.TLS != nil && mint.Spec.Ingress.TLS.Enabled {
		secretName := mint.Spec.Ingress.TLS.SecretName
		if secretName == "" {
			secretName = mint.Name + "-tls"
		}

		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{mint.Spec.Ingress.Host},
				SecretName: secretName,
			},
		}
	}

	if err := controllerutil.SetControllerReference(mint, ingress, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return ingress, nil
}
