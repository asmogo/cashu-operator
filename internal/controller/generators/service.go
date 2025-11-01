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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// GenerateService creates a Service for the Cashu mint
func GenerateService(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*corev1.Service, error) {
	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	serviceType := corev1.ServiceTypeClusterIP
	if mint.Spec.Service != nil && mint.Spec.Service.Type != "" {
		serviceType = mint.Spec.Service.Type
	}

	listenPort := mint.Spec.MintInfo.ListenPort
	if listenPort == 0 {
		listenPort = 8085
	}

	ports := []corev1.ServicePort{
		{
			Name:       "api",
			Port:       listenPort,
			TargetPort: intstr.FromString("api"),
			Protocol:   corev1.ProtocolTCP,
		},
	}

	// Add LDK node ports if enabled
	if mint.Spec.LDKNode != nil && mint.Spec.LDKNode.Enabled {
		ldkPort := mint.Spec.LDKNode.Port
		if ldkPort == 0 {
			ldkPort = 8090
		}
		webserverPort := mint.Spec.LDKNode.WebserverPort
		if webserverPort == 0 {
			webserverPort = 8888
		}

		ports = append(ports,
			corev1.ServicePort{
				Name:       "ldk",
				Port:       ldkPort,
				TargetPort: intstr.FromString("ldk"),
				Protocol:   corev1.ProtocolTCP,
			},
			corev1.ServicePort{
				Name:       "ldk-webserver",
				Port:       webserverPort,
				TargetPort: intstr.FromString("webserver"),
				Protocol:   corev1.ProtocolTCP,
			},
		)
	}

	// Add management RPC port if enabled
	if mint.Spec.ManagementRPC != nil && mint.Spec.ManagementRPC.Enabled {
		mgmtPort := mint.Spec.ManagementRPC.Port
		if mgmtPort == 0 {
			mgmtPort = 8086
		}
		ports = append(ports, corev1.ServicePort{
			Name:       "management",
			Port:       mgmtPort,
			TargetPort: intstr.FromInt(int(mgmtPort)),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	annotations := make(map[string]string)
	if mint.Spec.Service != nil && mint.Spec.Service.Annotations != nil {
		annotations = mint.Spec.Service.Annotations
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        mint.Name,
			Namespace:   mint.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: labels,
			Ports:    ports,
		},
	}

	// Add LoadBalancer specific configuration
	if serviceType == corev1.ServiceTypeLoadBalancer &&
		mint.Spec.Service != nil && mint.Spec.Service.LoadBalancerIP != "" {
		service.Spec.LoadBalancerIP = mint.Spec.Service.LoadBalancerIP
	}

	if err := controllerutil.SetControllerReference(mint, service, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return service, nil
}
