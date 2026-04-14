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

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// GeneratePodMonitor creates a PodMonitor for the Cashu mint metrics endpoint.
func GeneratePodMonitor(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*monitoringv1.PodMonitor, error) {
	if mint.Spec.Prometheus == nil || !mint.Spec.Prometheus.Enabled {
		return nil, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	podMonitor := &monitoringv1.PodMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       "PodMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mint.Name,
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{
					Port: ptr.To("metrics"),
					Path: "/metrics",
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(mint, podMonitor, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return podMonitor, nil
}
