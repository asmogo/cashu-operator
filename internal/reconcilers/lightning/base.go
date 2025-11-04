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

package lightning

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
	"github.com/asmogo/cashu-operator/internal/resources"
	"github.com/asmogo/cashu-operator/internal/status"
)

// BaseReconciler provides common functionality for all lightning reconcilers.
// Reconcilers should embed this struct to inherit shared reconciliation logic.
type BaseReconciler struct {
	Client        client.Client
	StatusManager *status.Manager
	Applier       *resources.Applier
}

// ReconcileConfig handles common configuration reconciliation tasks:
// - Creating/updating ConfigMaps
// - Creating/updating Secrets
// - Recording config hashes for detecting changes
func (br *BaseReconciler) ReconcileConfig(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
	configMap *corev1.ConfigMap,
	secret *corev1.Secret,
) error {
	// Apply ConfigMap with owner reference
	if configMap != nil {
		if err := br.Applier.ApplyWithOwner(ctx, mint, configMap); err != nil {
			return fmt.Errorf("failed to apply ConfigMap: %w", err)
		}
	}

	// Apply Secret with owner reference
	if secret != nil {
		if err := br.Applier.ApplyWithOwner(ctx, mint, secret); err != nil {
			return fmt.Errorf("failed to apply Secret: %w", err)
		}
	}

	return nil
}

// ReconcileDeployment handles deployment creation/updates and readiness checks.
// It applies the deployment and waits for it to be ready.
func (br *BaseReconciler) ReconcileDeployment(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
	deployment *appsv1.Deployment,
) error {
	// Apply deployment with owner reference
	if err := br.Applier.ApplyWithOwner(ctx, mint, deployment); err != nil {
		return fmt.Errorf("failed to apply Deployment: %w", err)
	}

	// Check if deployment is ready
	namespacedName := types.NamespacedName{
		Namespace: deployment.Namespace,
		Name:      deployment.Name,
	}

	// Fetch current deployment state
	currentDeployment := &appsv1.Deployment{}
	if err := br.Client.Get(ctx, namespacedName, currentDeployment); err != nil {
		return fmt.Errorf("failed to get Deployment: %w", err)
	}

	// Check readiness
	if !status.IsDeploymentReady(currentDeployment) {
		return fmt.Errorf("deployment %s/%s is not ready yet", deployment.Namespace, deployment.Name)
	}

	return nil
}

// ReconcileService handles service creation/updates and readiness checks.
func (br *BaseReconciler) ReconcileService(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
	service *corev1.Service,
) error {
	// Apply service with owner reference
	if err := br.Applier.ApplyWithOwner(ctx, mint, service); err != nil {
		return fmt.Errorf("failed to apply Service: %w", err)
	}

	// Check if service is ready
	namespacedName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	// Fetch current service state
	currentService := &corev1.Service{}
	if err := br.Client.Get(ctx, namespacedName, currentService); err != nil {
		return fmt.Errorf("failed to get Service: %w", err)
	}

	// Check readiness
	if !status.IsServiceReady(currentService) {
		return fmt.Errorf("service %s/%s is not ready yet", service.Namespace, service.Name)
	}

	return nil
}

// GetNamespacedName returns a NamespacedName for the given resource name.
func (br *BaseReconciler) GetNamespacedName(namespace, name string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}

// RequeueAfterShort returns a requeue result with a short delay.
// Useful for retrying failed operations quickly.
func (br *BaseReconciler) RequeueAfterShort() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: resources.DefaultRequeueAfterShort}, nil
}

// RequeueAfterLong returns a requeue result with a long delay.
// Useful for checking status periodically.
func (br *BaseReconciler) RequeueAfterLong() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: resources.DefaultRequeueAfterLong}, nil
}
