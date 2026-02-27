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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

const DefaultPaymentProcessorPort int32 = 50051

func PaymentProcessorResourceName(mintName, processorName string) string {
	return fmt.Sprintf("%s-processor-%s", mintName, processorName)
}

func FindPaymentProcessorByName(
	mint *mintv1alpha1.CashuMint,
	name string,
) (*mintv1alpha1.PaymentProcessorSpec, bool) {
	for i := range mint.Spec.PaymentProcessors {
		if mint.Spec.PaymentProcessors[i].Name == name {
			return &mint.Spec.PaymentProcessors[i], true
		}
	}
	return nil, false
}

func EffectivePaymentProcessorPort(processor *mintv1alpha1.PaymentProcessorSpec) int32 {
	if processor != nil && processor.Port != 0 {
		return processor.Port
	}
	return DefaultPaymentProcessorPort
}

func PaymentProcessorServiceAddress(
	mint *mintv1alpha1.CashuMint,
	processor *mintv1alpha1.PaymentProcessorSpec,
) string {
	return fmt.Sprintf(
		"%s.%s.svc.cluster.local",
		PaymentProcessorResourceName(mint.Name, processor.Name),
		mint.Namespace,
	)
}

func GeneratePaymentProcessorDeployment(
	mint *mintv1alpha1.CashuMint,
	processor *mintv1alpha1.PaymentProcessorSpec,
	scheme *runtime.Scheme,
) (*appsv1.Deployment, error) {
	if processor == nil {
		return nil, nil
	}

	replicas := int32(1)
	if processor.Replicas != nil {
		replicas = *processor.Replicas
	}

	imagePullPolicy := processor.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = corev1.PullIfNotPresent
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-payment-processor",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "payment-processor",
		"app.kubernetes.io/processor":  processor.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      PaymentProcessorResourceName(mint.Name, processor.Name),
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: mint.Spec.ImagePullSecrets,
					NodeSelector:     mint.Spec.NodeSelector,
					Tolerations:      mint.Spec.Tolerations,
					Affinity:         mint.Spec.Affinity,
					Containers: []corev1.Container{
						{
							Name:            "processor",
							Image:           processor.Image,
							ImagePullPolicy: imagePullPolicy,
							Command:         processor.Command,
							Args:            processor.Args,
							Env:             processor.Env,
							Resources: func() corev1.ResourceRequirements {
								if processor.Resources != nil {
									return *processor.Resources
								}
								return corev1.ResourceRequirements{}
							}(),
							Ports: []corev1.ContainerPort{
								{
									Name:          "grpc",
									ContainerPort: EffectivePaymentProcessorPort(processor),
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(mint, deployment, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return deployment, nil
}

func GeneratePaymentProcessorService(
	mint *mintv1alpha1.CashuMint,
	processor *mintv1alpha1.PaymentProcessorSpec,
	scheme *runtime.Scheme,
) (*corev1.Service, error) {
	if processor == nil {
		return nil, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-payment-processor",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "payment-processor",
		"app.kubernetes.io/processor":  processor.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      PaymentProcessorResourceName(mint.Name, processor.Name),
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       EffectivePaymentProcessorPort(processor),
					TargetPort: intstr.FromString("grpc"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(mint, service, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return service, nil
}
