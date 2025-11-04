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

package status

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// Manager handles all status-related operations for CashuMint resources.
// It provides a clean interface for updating conditions and phases without
// cluttering the main controller logic.
type Manager struct {
	client client.Client
}

// NewManager creates a new status manager.
func NewManager(c client.Client) *Manager {
	return &Manager{client: c}
}

// SetCondition updates or adds a condition to the CashuMint status.
func (m *Manager) SetCondition(ctx context.Context, mint *mintv1alpha1.CashuMint, condition metav1.Condition) error {
	meta.SetStatusCondition(&mint.Status.Conditions, condition)
	return m.Update(ctx, mint)
}

// SetPhase updates the phase and updates the status.
func (m *Manager) SetPhase(ctx context.Context, mint *mintv1alpha1.CashuMint, phase mintv1alpha1.MintPhase) error {
	mint.Status.Phase = phase
	return m.Update(ctx, mint)
}

// SetReady marks the mint as ready with the given message.
func (m *Manager) SetReady(ctx context.Context, mint *mintv1alpha1.CashuMint, message string) error {
	mint.Status.Phase = mintv1alpha1.MintPhaseReady
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: mint.Generation,
		Reason:             "Ready",
		Message:            message,
	})
	return m.Update(ctx, mint)
}

// SetError marks the mint as failed with the given error message.
func (m *Manager) SetError(ctx context.Context, mint *mintv1alpha1.CashuMint, err error) error {
	mint.Status.Phase = mintv1alpha1.MintPhaseFailed
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: mint.Generation,
		Reason:             "ReconciliationFailed",
		Message:            err.Error(),
	})
	return m.Update(ctx, mint)
}

// SetProvisioning marks the mint as provisioning.
func (m *Manager) SetProvisioning(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	mint.Status.Phase = mintv1alpha1.MintPhaseProvisioning
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: mint.Generation,
		Reason:             "Provisioning",
		Message:            "Starting resource provisioning",
	})
	return m.Update(ctx, mint)
}

// SetUpdating marks the mint as updating.
func (m *Manager) SetUpdating(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	mint.Status.Phase = mintv1alpha1.MintPhaseUpdating
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: mint.Generation,
		Reason:             "Updating",
		Message:            "Updating resources due to spec change",
	})
	return m.Update(ctx, mint)
}

// SetDatabaseReady marks the database as ready.
func (m *Manager) SetDatabaseReady(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeDatabaseReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: mint.Generation,
		Reason:             "DatabaseReady",
		Message:            "Database is ready",
	})
	return m.Update(ctx, mint)
}

// SetDatabaseNotReady marks the database as not ready.
func (m *Manager) SetDatabaseNotReady(ctx context.Context, mint *mintv1alpha1.CashuMint, reason, message string) error {
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeDatabaseReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: mint.Generation,
		Reason:             reason,
		Message:            message,
	})
	return m.Update(ctx, mint)
}

// SetConfigValid marks the configuration as valid.
func (m *Manager) SetConfigValid(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeConfigValid,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: mint.Generation,
		Reason:             "ConfigurationValid",
		Message:            "Configuration is valid and applied",
	})
	return m.Update(ctx, mint)
}

// SetIngressReady marks the ingress as ready.
func (m *Manager) SetIngressReady(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeIngressReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: mint.Generation,
		Reason:             "IngressReady",
		Message:            "Ingress is ready and accessible",
	})
	return m.Update(ctx, mint)
}

// SetIngressNotReady marks the ingress as not ready.
func (m *Manager) SetIngressNotReady(ctx context.Context, mint *mintv1alpha1.CashuMint, reason, message string) error {
	meta.SetStatusCondition(&mint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeIngressReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: mint.Generation,
		Reason:             reason,
		Message:            message,
	})
	return m.Update(ctx, mint)
}

// UpdateResourceNames updates the names of managed resources.
func (m *Manager) UpdateResourceNames(ctx context.Context, mint *mintv1alpha1.CashuMint, deployment, service, configmap, ingress string) error {
	mint.Status.DeploymentName = deployment
	mint.Status.ServiceName = service
	mint.Status.ConfigMapName = configmap
	mint.Status.IngressName = ingress
	return m.Update(ctx, mint)
}

// UpdateDeploymentStatus updates deployment-specific status fields.
func (m *Manager) UpdateDeploymentStatus(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	deployment := &appsv1.Deployment{}
	deploymentKey := client.ObjectKey{Name: mint.Name, Namespace: mint.Namespace}
	if err := m.client.Get(ctx, deploymentKey, deployment); err == nil {
		mint.Status.ReadyReplicas = deployment.Status.ReadyReplicas
		mint.Status.DeploymentName = deployment.Name
	} else if !apierrors.IsNotFound(err) {
		logger.Error(err, "Failed to get Deployment for status update")
	}

	return m.Update(ctx, mint)
}

// UpdateIngressStatus updates ingress-specific status fields and URL.
func (m *Manager) UpdateIngressStatus(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	if mint.Spec.Ingress == nil || !mint.Spec.Ingress.Enabled {
		return nil
	}

	ingress := &networkingv1.Ingress{}
	ingressKey := client.ObjectKey{Name: mint.Name, Namespace: mint.Namespace}
	if err := m.client.Get(ctx, ingressKey, ingress); err == nil {
		mint.Status.IngressName = ingress.Name

		// Set URL based on ingress
		if len(ingress.Status.LoadBalancer.Ingress) > 0 {
			if len(ingress.Spec.TLS) > 0 {
				mint.Status.URL = "https://" + mint.Spec.Ingress.Host
			} else {
				mint.Status.URL = "http://" + mint.Spec.Ingress.Host
			}

			return m.SetIngressReady(ctx, mint)
		}

		return m.SetIngressNotReady(ctx, mint, "IngressNotReady", "Waiting for ingress to be assigned")
	} else if !apierrors.IsNotFound(err) {
		logger.Error(err, "Failed to get Ingress for status update")
	}

	return m.Update(ctx, mint)
}

// UpdateObservedGeneration updates the observed generation.
func (m *Manager) UpdateObservedGeneration(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	mint.Status.ObservedGeneration = mint.Generation
	return m.Update(ctx, mint)
}

// Update persists status changes to the API server.
func (m *Manager) Update(ctx context.Context, mint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)
	if err := m.client.Status().Update(ctx, mint); err != nil {
		logger.Error(err, "Failed to update CashuMint status")
		return fmt.Errorf("failed to update status: %w", err)
	}
	return nil
}

// IsDeploymentReady checks if a Deployment is ready.
func IsDeploymentReady(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}

	if deployment.Spec.Replicas == nil {
		return false
	}

	desiredReplicas := *deployment.Spec.Replicas

	return deployment.Status.ReadyReplicas == desiredReplicas &&
		deployment.Status.UpdatedReplicas == desiredReplicas &&
		deployment.Status.AvailableReplicas == desiredReplicas
}

// IsStatefulSetReady checks if a StatefulSet is ready.
func IsStatefulSetReady(sts *appsv1.StatefulSet) bool {
	if sts == nil {
		return false
	}

	if sts.Spec.Replicas == nil {
		return false
	}

	desiredReplicas := *sts.Spec.Replicas

	return sts.Status.ReadyReplicas == desiredReplicas &&
		sts.Status.CurrentReplicas == desiredReplicas &&
		sts.Status.UpdatedReplicas == desiredReplicas
}

// IsServiceReady checks if a Service is ready.
func IsServiceReady(service *corev1.Service) bool {
	if service == nil {
		return false
	}

	// For ClusterIP and NodePort services, they're ready once created
	if service.Spec.Type == corev1.ServiceTypeClusterIP ||
		service.Spec.Type == corev1.ServiceTypeNodePort {
		return service.Spec.ClusterIP != ""
	}

	// For LoadBalancer services, check if external IP is assigned
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		return len(service.Status.LoadBalancer.Ingress) > 0
	}

	return true
}

// IsIngressReady checks if an Ingress is ready.
func IsIngressReady(ingress *networkingv1.Ingress) bool {
	if ingress == nil {
		return false
	}

	return len(ingress.Status.LoadBalancer.Ingress) > 0
}

// IsPVCBound checks if a PVC is bound.
func IsPVCBound(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc == nil {
		return false
	}
	return pvc.Status.Phase == corev1.ClaimBound
}
