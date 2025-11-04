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

package database

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

// BaseReconciler provides common functionality for all database reconcilers.
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

// ReconcileStatefulSet handles stateful set creation/updates and readiness checks.
func (br *BaseReconciler) ReconcileStatefulSet(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
	statefulSet *appsv1.StatefulSet,
) error {
	// Apply StatefulSet with owner reference
	if err := br.Applier.ApplyWithOwner(ctx, mint, statefulSet); err != nil {
		return fmt.Errorf("failed to apply StatefulSet: %w", err)
	}

	// Check if stateful set is ready
	namespacedName := types.NamespacedName{
		Namespace: statefulSet.Namespace,
		Name:      statefulSet.Name,
	}

	// Fetch current stateful set state
	currentStatefulSet := &appsv1.StatefulSet{}
	if err := br.Client.Get(ctx, namespacedName, currentStatefulSet); err != nil {
		return fmt.Errorf("failed to get StatefulSet: %w", err)
	}

	// Check readiness
	if !status.IsStatefulSetReady(currentStatefulSet) {
		return fmt.Errorf("statefulSet %s/%s is not ready yet", statefulSet.Namespace, statefulSet.Name)
	}

	return nil
}

// ReconcilePVC handles PersistentVolumeClaim creation/updates and binding checks.
func (br *BaseReconciler) ReconcilePVC(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
	pvc *corev1.PersistentVolumeClaim,
) error {
	// Apply PVC with owner reference
	if err := br.Applier.ApplyWithOwner(ctx, mint, pvc); err != nil {
		return fmt.Errorf("failed to apply PVC: %w", err)
	}

	// Check if PVC is bound
	namespacedName := types.NamespacedName{
		Namespace: pvc.Namespace,
		Name:      pvc.Name,
	}

	// Fetch current PVC state
	currentPVC := &corev1.PersistentVolumeClaim{}
	if err := br.Client.Get(ctx, namespacedName, currentPVC); err != nil {
		return fmt.Errorf("failed to get PVC: %w", err)
	}

	// Check if bound
	if !status.IsPVCBound(currentPVC) {
		return fmt.Errorf("pvc %s/%s is not bound yet", pvc.Namespace, pvc.Name)
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
// Use this for quick retries on transient errors (e.g., temporary API failures, network issues).
// Default interval is 5 seconds.
func (br *BaseReconciler) RequeueAfterShort() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: resources.DefaultRequeueAfterShort}, nil
}

// RequeueAfterMedium returns a requeue result with a medium delay.
// Use this for normal operational delays (e.g., waiting for resources to become ready).
// Default interval is 30 seconds.
func (br *BaseReconciler) RequeueAfterMedium() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: resources.DefaultRequeueAfterMedium}, nil
}

// RequeueAfterLong returns a requeue result with a long delay.
// Use this for periodic status checks and normal operation (e.g., checking if a deployment is ready).
// Default interval is 2 minutes.
func (br *BaseReconciler) RequeueAfterLong() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: resources.DefaultRequeueAfterLong}, nil
}
