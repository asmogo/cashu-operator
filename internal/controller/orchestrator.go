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

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
	"github.com/asmogo/cashu-operator/internal/resources"
	"github.com/asmogo/cashu-operator/internal/status"
)

// ReconciliationOrchestrator orchestrates the multi-phase reconciliation process.
// It breaks down the complex reconciliation into distinct phases, making the
// code more maintainable and testable.
//
// Reconciliation phases in order:
// 1. Phase synchronization (pending -> provisioning/updating)
// 2. PostgreSQL auto-provisioning (if enabled)
// 3. ConfigMap reconciliation
// 4. PersistentVolumeClaim reconciliation (for local storage)
// 5. Deployment reconciliation
// 6. Service reconciliation
// 7. Ingress reconciliation (if enabled)
// 8. Status validation and finalization
type ReconciliationOrchestrator struct {
	client         client.Client
	statusManager  *status.Manager
	applier        *resources.Applier
	reconciliators map[string]ResourceReconciliator
}

// ResourceReconciliator defines the interface for resource-specific reconciliation.
type ResourceReconciliator interface {
	// Reconcile performs the reconciliation for this specific resource type.
	// It should return true if the resource is ready, false otherwise.
	Reconcile(ctx context.Context, mint *mintv1alpha1.CashuMint) error

	// ShouldReconcile determines if this resource should be reconciled for the given mint.
	ShouldReconcile(mint *mintv1alpha1.CashuMint) bool

	// Name returns the name of this reconciliator for logging.
	Name() string
}

// NewReconciliationOrchestrator creates a new reconciliation orchestrator.
func NewReconciliationOrchestrator(
	c client.Client,
	statusMgr *status.Manager,
	applier *resources.Applier,
) *ReconciliationOrchestrator {
	return &ReconciliationOrchestrator{
		client:        c,
		statusManager: statusMgr,
		applier:       applier,
	}
}

// Orchestrate performs the complete reconciliation orchestration.
// It executes all reconciliation phases in the correct order and returns
// the appropriate requeue result.
func (ro *ReconciliationOrchestrator) Orchestrate(
	ctx context.Context,
	cashuMint *mintv1alpha1.CashuMint,
) error {
	logger := log.FromContext(ctx)

	// Phase 0: Update phase if transitioning from Pending
	if cashuMint.Status.Phase == mintv1alpha1.MintPhasePending {
		if err := ro.statusManager.SetProvisioning(ctx, cashuMint); err != nil {
			return fmt.Errorf("failed to set provisioning phase: %w", err)
		}
	}

	// Phase 0b: Check for spec changes
	if cashuMint.Status.ObservedGeneration != 0 && cashuMint.Status.ObservedGeneration != cashuMint.Generation {
		logger.Info("Spec changed, transitioning to updating phase")
		if err := ro.statusManager.SetUpdating(ctx, cashuMint); err != nil {
			return fmt.Errorf("failed to set updating phase: %w", err)
		}
	}

	// Phase 1: PostgreSQL auto-provisioning
	if cashuMint.Spec.Database.Postgres != nil && cashuMint.Spec.Database.Postgres.AutoProvision {
		logger.Info("Phase 1: Reconciling auto-provisioned PostgreSQL")
		if err := ro.reconcilePostgreSQL(ctx, cashuMint); err != nil {
			return fmt.Errorf("phase 1 failed: %w", err)
		}
	}

	// Phase 2: ConfigMap
	logger.Info("Phase 2: Reconciling ConfigMap")
	if err := ro.reconcileConfigMap(ctx, cashuMint); err != nil {
		return fmt.Errorf("phase 2 failed: %w", err)
	}

	// Phase 3: PVC for local storage
	if isLocalStorageEngine(cashuMint.Spec.Database.Engine) {
		logger.Info("Phase 3: Reconciling PVC for local database")
		if err := ro.reconcilePVC(ctx, cashuMint); err != nil {
			return fmt.Errorf("phase 3 failed: %w", err)
		}
	}

	// Phase 4: Deployment
	logger.Info("Phase 4: Reconciling Deployment")
	if err := ro.reconcileDeployment(ctx, cashuMint); err != nil {
		return fmt.Errorf("phase 4 failed: %w", err)
	}

	// Phase 5: Service
	logger.Info("Phase 5: Reconciling Service")
	if err := ro.reconcileService(ctx, cashuMint); err != nil {
		return fmt.Errorf("phase 5 failed: %w", err)
	}

	// Phase 6: Ingress
	if cashuMint.Spec.Ingress != nil && cashuMint.Spec.Ingress.Enabled {
		logger.Info("Phase 6: Reconciling Ingress")
		if err := ro.reconcileIngress(ctx, cashuMint); err != nil {
			return fmt.Errorf("phase 6 failed: %w", err)
		}
	}

	// Phase 7: Finalization - check deployment readiness
	logger.Info("Phase 7: Validating deployment readiness")
	if err := ro.validateDeploymentReadiness(ctx, cashuMint); err != nil {
		// Not an error condition - just means we're not ready yet
		logger.Info("Deployment not ready yet, will requeue")
		meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
			Type:               mintv1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cashuMint.Generation,
			Reason:             "DeploymentNotReady",
			Message:            "Waiting for deployment to be ready",
		})
	} else {
		// All resources are ready
		if err := ro.statusManager.SetReady(ctx, cashuMint, "All resources reconciled successfully"); err != nil {
			logger.Error(err, "Failed to set ready phase")
		}
	}

	return nil
}

// isLocalStorageEngine returns true if the database engine uses local storage.
func isLocalStorageEngine(engine string) bool {
	return engine == "sqlite" || engine == "redb"
}

// reconcilePostgreSQL reconciles PostgreSQL auto-provisioning resources.
func (ro *ReconciliationOrchestrator) reconcilePostgreSQL(
	ctx context.Context,
	cashuMint *mintv1alpha1.CashuMint,
) error {
	logger := log.FromContext(ctx)

	// Fetch or generate the database secret
	secret := &corev1.Secret{}
	err := ro.applier.GetOrError(ctx,
		resources.CreateObjectKey(cashuMint.Namespace, cashuMint.Name+"-postgres-secret"),
		secret)
	if err == nil {
		logger.Info("Using existing postgres password from secret")
	} else {
		logger.Info("Generating new postgres password")
	}

	// TODO: Generate and apply PostgreSQL resources (secret, service, statefulset)
	// This is currently handled in the main controller's reconcilePostgreSQL method

	return nil
}

// reconcileConfigMap reconciles the ConfigMap containing mint configuration.
func (ro *ReconciliationOrchestrator) reconcileConfigMap(
	ctx context.Context,
	cashuMint *mintv1alpha1.CashuMint,
) error {
	// TODO: Implement ConfigMap reconciliation logic
	// This is currently handled in the main controller's reconcileConfigMap method
	return nil
}

// reconcilePVC reconciles the PersistentVolumeClaim for local storage.
func (ro *ReconciliationOrchestrator) reconcilePVC(
	ctx context.Context,
	cashuMint *mintv1alpha1.CashuMint,
) error {
	// TODO: Implement PVC reconciliation logic
	// This is currently handled in the main controller's reconcilePVC method
	return nil
}

// reconcileDeployment reconciles the mint Deployment.
func (ro *ReconciliationOrchestrator) reconcileDeployment(
	ctx context.Context,
	cashuMint *mintv1alpha1.CashuMint,
) error {
	// TODO: Implement Deployment reconciliation logic
	// This is currently handled in the main controller's reconcileDeployment method
	return nil
}

// reconcileService reconciles the mint Service.
func (ro *ReconciliationOrchestrator) reconcileService(
	ctx context.Context,
	cashuMint *mintv1alpha1.CashuMint,
) error {
	// TODO: Implement Service reconciliation logic
	// This is currently handled in the main controller's reconcileService method
	return nil
}

// reconcileIngress reconciles the mint Ingress (if enabled).
func (ro *ReconciliationOrchestrator) reconcileIngress(
	ctx context.Context,
	cashuMint *mintv1alpha1.CashuMint,
) error {
	// TODO: Implement Ingress reconciliation logic
	// This is currently handled in the main controller's reconcileIngress method
	return nil
}

// validateDeploymentReadiness checks if the mint Deployment is ready.
func (ro *ReconciliationOrchestrator) validateDeploymentReadiness(
	ctx context.Context,
	cashuMint *mintv1alpha1.CashuMint,
) error {
	deployment := &appsv1.Deployment{}
	if err := ro.client.Get(ctx, client.ObjectKey{
		Name:      cashuMint.Name,
		Namespace: cashuMint.Namespace,
	}, deployment); err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	if !status.IsDeploymentReady(deployment) {
		return fmt.Errorf("deployment %s/%s is not ready", deployment.Namespace, deployment.Name)
	}

	return nil
}
