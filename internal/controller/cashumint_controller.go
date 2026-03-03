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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
	"github.com/asmogo/cashu-operator/internal/controller/generators"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

const (
	// Reconciliation intervals
	ReconcileInterval       = 5 * time.Minute
	UpdateReconcileInterval = 30 * time.Second
	NotReadyRetryInterval   = 10 * time.Second

	// Finalizer name
	cashuMintFinalizer = "mint.cashu.asmogo.github.io/finalizer"
)

// CashuMintReconciler reconciles a CashuMint object
type CashuMintReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mint.cashu.asmogo.github.io,resources=cashumints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mint.cashu.asmogo.github.io,resources=cashumints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mint.cashu.asmogo.github.io,resources=cashumints/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CashuMintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the CashuMint instance
	cashuMint := &mintv1alpha1.CashuMint{}
	if err := r.Get(ctx, req.NamespacedName, cashuMint); err != nil {
		if apierrors.IsNotFound(err) {
			// Resource not found, likely deleted
			logger.Info("CashuMint resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get CashuMint")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling CashuMint", "name", cashuMint.Name, "namespace", cashuMint.Namespace)

	// Handle deletion with finalizer
	if !cashuMint.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, cashuMint)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(cashuMint, cashuMintFinalizer) {
		controllerutil.AddFinalizer(cashuMint, cashuMintFinalizer)
		if err := r.Update(ctx, cashuMint); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if needed
	if cashuMint.Status.Phase == "" {
		cashuMint.Status.Phase = mintv1alpha1.MintPhasePending
		if err := r.Status().Update(ctx, cashuMint); err != nil {
			logger.Error(err, "Failed to update status to Pending")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Main reconciliation logic
	result, err := r.reconcileResources(ctx, cashuMint)
	if err != nil {
		return r.handleError(ctx, cashuMint, err)
	}

	// Update status
	if err := r.updateStatus(ctx, cashuMint); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return result, nil
}

// handleDeletion handles the deletion of CashuMint resources
func (r *CashuMintReconciler) handleDeletion(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(cashuMint, cashuMintFinalizer) {
		// Perform cleanup logic here
		logger.Info("Performing cleanup for CashuMint", "name", cashuMint.Name)

		// TODO: Add cleanup logic (e.g., external database cleanup, notifications)

		// Remove finalizer
		controllerutil.RemoveFinalizer(cashuMint, cashuMintFinalizer)
		if err := r.Update(ctx, cashuMint); err != nil {
			logger.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcileResources handles the main reconciliation of all resources
func (r *CashuMintReconciler) reconcileResources(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Update phase to Provisioning if currently Pending
	if cashuMint.Status.Phase == mintv1alpha1.MintPhasePending {
		cashuMint.Status.Phase = mintv1alpha1.MintPhaseProvisioning
		meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
			Type:               mintv1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cashuMint.Generation,
			Reason:             "Provisioning",
			Message:            "Starting resource provisioning",
		})
	}

	// Check if spec has changed (generation mismatch)
	if cashuMint.Status.ObservedGeneration != 0 && cashuMint.Status.ObservedGeneration != cashuMint.Generation {
		logger.Info("Spec changed, updating resources")
		cashuMint.Status.Phase = mintv1alpha1.MintPhaseUpdating
		meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
			Type:               mintv1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cashuMint.Generation,
			Reason:             "Updating",
			Message:            "Updating resources due to spec change",
		})
	}

	// Phase 1: Reconcile PostgreSQL auto-provisioning (if needed)
	if cashuMint.Spec.Database.Postgres != nil && cashuMint.Spec.Database.Postgres.AutoProvision {
		logger.Info("Reconciling auto-provisioned PostgreSQL")
		if err := r.reconcilePostgreSQL(ctx, cashuMint); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile PostgreSQL: %w", err)
		}
	}

	// Phase 2: Reconcile backup resources (if enabled)
	if cashuMint.Spec.Backup != nil && cashuMint.Spec.Backup.Enabled {
		logger.Info("Reconciling backup resources")
		if err := r.reconcileBackup(ctx, cashuMint); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile backup resources: %w", err)
		}
	}

	// Phase 3: Reconcile ConfigMap
	logger.Info("Reconciling ConfigMap")
	if err := r.reconcileConfigMap(ctx, cashuMint); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile ConfigMap: %w", err)
	}

	// Phase 4: Reconcile PVC (for SQLite/redb)
	if cashuMint.Spec.Database.Engine == mintv1alpha1.DatabaseEngineSQLite || cashuMint.Spec.Database.Engine == mintv1alpha1.DatabaseEngineRedb {
		logger.Info("Reconciling PVC for local database")
		if err := r.reconcilePVC(ctx, cashuMint); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile PVC: %w", err)
		}
	}

	// Phase 4: Reconcile Deployment
	logger.Info("Reconciling Deployment")
	if err := r.reconcileDeployment(ctx, cashuMint); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile Deployment: %w", err)
	}

	// Phase 5: Reconcile Service
	logger.Info("Reconciling Service")
	if err := r.reconcileService(ctx, cashuMint); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile Service: %w", err)
	}

	// Phase 6: Reconcile Ingress (if enabled)
	if cashuMint.Spec.Ingress != nil && cashuMint.Spec.Ingress.Enabled {
		logger.Info("Reconciling Ingress")
		if err := r.reconcileIngress(ctx, cashuMint); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile Ingress: %w", err)
		}

		// Phase 7: Reconcile Certificate (if cert-manager is enabled)
		if cashuMint.Spec.Ingress.TLS != nil && cashuMint.Spec.Ingress.TLS.Enabled &&
			cashuMint.Spec.Ingress.TLS.CertManager != nil && cashuMint.Spec.Ingress.TLS.CertManager.Enabled {
			logger.Info("Reconciling Certificate")
			if err := r.reconcileCertificate(ctx, cashuMint); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to reconcile Certificate: %w", err)
			}
		}
	}

	// Check deployment readiness and update phase
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: cashuMint.Name, Namespace: cashuMint.Namespace}, deployment); err == nil {
		if isDeploymentReady(deployment) {
			cashuMint.Status.Phase = mintv1alpha1.MintPhaseReady
			meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
				Type:               mintv1alpha1.ConditionTypeReady,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: cashuMint.Generation,
				Reason:             "ReconciliationComplete",
				Message:            "All resources reconciled successfully",
			})
		} else {
			meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
				Type:               mintv1alpha1.ConditionTypeReady,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: cashuMint.Generation,
				Reason:             "DeploymentNotReady",
				Message:            "Waiting for deployment to be ready",
			})
		}
	}

	// Determine requeue interval
	if cashuMint.Status.Phase == mintv1alpha1.MintPhaseUpdating || cashuMint.Status.Phase == mintv1alpha1.MintPhaseProvisioning {
		return ctrl.Result{RequeueAfter: UpdateReconcileInterval}, nil
	}

	return ctrl.Result{RequeueAfter: ReconcileInterval}, nil
}

// handleError handles reconciliation errors
func (r *CashuMintReconciler) handleError(ctx context.Context, cashuMint *mintv1alpha1.CashuMint, err error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Handle specific error types
	if apierrors.IsConflict(err) {
		// Conflict: retry immediately
		logger.Info("Conflict detected, retrying")
		return ctrl.Result{Requeue: true}, nil
	}

	if apierrors.IsNotFound(err) {
		// Dependency not found: fast retry
		logger.Info("Dependency not found, retrying after delay")
		return ctrl.Result{RequeueAfter: NotReadyRetryInterval}, nil
	}

	// Update status with error
	cashuMint.Status.Phase = mintv1alpha1.MintPhaseFailed
	meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: cashuMint.Generation,
		Reason:             "ReconciliationFailed",
		Message:            err.Error(),
	})

	if updateErr := r.Status().Update(ctx, cashuMint); updateErr != nil {
		logger.Error(updateErr, "Failed to update status after error")
	}

	// Return error for exponential backoff
	return ctrl.Result{}, err
}

// updateStatus re-fetches the CashuMint from the API server (to get the latest
// resourceVersion) and applies the desired status derived from the current
// cluster state. Using a fresh fetch avoids the "object has been modified"
// conflict that occurs when reconcileResources already mutated Status in memory.
func (r *CashuMintReconciler) updateStatus(ctx context.Context, desired *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	// Re-fetch to get the current resourceVersion.
	current := &mintv1alpha1.CashuMint{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(desired), current); err != nil {
		return fmt.Errorf("failed to re-fetch CashuMint for status update: %w", err)
	}

	// Copy the status we computed during reconciliation onto the fresh object.
	current.Status = desired.Status

	// Observe current cluster resources and update status fields.

	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: current.Name, Namespace: current.Namespace}, deployment); err == nil {
		current.Status.ReadyReplicas = deployment.Status.ReadyReplicas
		current.Status.DeploymentName = deployment.Name
	} else if !apierrors.IsNotFound(err) {
		logger.Error(err, "Failed to get Deployment for status update")
	}

	service := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: current.Name, Namespace: current.Namespace}, service); err == nil {
		current.Status.ServiceName = service.Name
	} else if !apierrors.IsNotFound(err) {
		logger.Error(err, "Failed to get Service for status update")
	}

	if current.Spec.Ingress != nil && current.Spec.Ingress.Enabled {
		ingress := &networkingv1.Ingress{}
		if err := r.Get(ctx, client.ObjectKey{Name: current.Name, Namespace: current.Namespace}, ingress); err == nil {
			current.Status.IngressName = ingress.Name
			if len(ingress.Status.LoadBalancer.Ingress) > 0 {
				if len(ingress.Spec.TLS) > 0 {
					current.Status.URL = "https://" + current.Spec.Ingress.Host
				} else {
					current.Status.URL = "http://" + current.Spec.Ingress.Host
				}
				meta.SetStatusCondition(&current.Status.Conditions, metav1.Condition{
					Type:               mintv1alpha1.ConditionTypeIngressReady,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: current.Generation,
					Reason:             "IngressReady",
					Message:            "Ingress is ready and accessible",
				})
			} else {
				meta.SetStatusCondition(&current.Status.Conditions, metav1.Condition{
					Type:               mintv1alpha1.ConditionTypeIngressReady,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: current.Generation,
					Reason:             "IngressNotReady",
					Message:            "Waiting for ingress to be assigned",
				})
			}
		} else if !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to get Ingress for status update")
		}
	}

	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{Name: current.Name + "-config", Namespace: current.Namespace}, configMap); err == nil {
		current.Status.ConfigMapName = configMap.Name
	} else if !apierrors.IsNotFound(err) {
		logger.Error(err, "Failed to get ConfigMap for status update")
	}

	current.Status.ObservedGeneration = current.Generation

	if err := r.Status().Update(ctx, current); err != nil {
		// Conflicts here are harmless — the next reconcile will try again.
		if apierrors.IsConflict(err) {
			logger.Info("Status update conflict, will retry on next reconcile")
			return nil
		}
		logger.Error(err, "Failed to update CashuMint status")
		return err
	}

	return nil
}

// reconcilePostgreSQL reconciles PostgreSQL auto-provisioning resources
func (r *CashuMintReconciler) reconcilePostgreSQL(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	// The postgres secret is created once and never updated — the password must
	// not change after Postgres initialises its data directory with it.
	secretName := cashuMint.Name + "-postgres-secret"
	existingSecret := &corev1.Secret{}
	secretErr := r.Get(ctx, client.ObjectKey{
		Namespace: cashuMint.Namespace,
		Name:      secretName,
	}, existingSecret)
	if secretErr != nil && !apierrors.IsNotFound(secretErr) {
		return fmt.Errorf("failed to get postgres secret: %w", secretErr)
	}

	if apierrors.IsNotFound(secretErr) {
		// Secret does not exist yet — generate and create it.
		secret, err := generators.GeneratePostgresSecret(cashuMint, "")
		if err != nil {
			return fmt.Errorf("failed to generate PostgreSQL secret: %w", err)
		}
		if secret != nil {
			if err := r.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create PostgreSQL secret: %w", err)
			}
			logger.Info("PostgreSQL secret created", "secret", secret.Name)
		}
	} else {
		logger.Info("PostgreSQL secret already exists, keeping existing password", "secret", secretName)
	}

	// Generate PostgreSQL Service
	service, err := generators.GeneratePostgresService(cashuMint, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate PostgreSQL service: %w", err)
	}
	if service != nil {
		if err := applyResource(ctx, r.Client, service); err != nil {
			return fmt.Errorf("failed to apply PostgreSQL service: %w", err)
		}
		logger.Info("PostgreSQL service reconciled", "service", service.Name)
	}

	// Generate PostgreSQL StatefulSet
	sts, err := generators.GeneratePostgresStatefulSet(cashuMint, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate PostgreSQL StatefulSet: %w", err)
	}
	if sts != nil {
		if err := applyResource(ctx, r.Client, sts); err != nil {
			return fmt.Errorf("failed to apply PostgreSQL StatefulSet: %w", err)
		}
		logger.Info("PostgreSQL StatefulSet reconciled", "statefulset", sts.Name)

		// Check if StatefulSet is ready
		if isStatefulSetReady(sts) {
			meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
				Type:               mintv1alpha1.ConditionTypeDatabaseReady,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: cashuMint.Generation,
				Reason:             "PostgreSQLReady",
				Message:            "PostgreSQL database is ready",
			})
		} else {
			meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
				Type:               mintv1alpha1.ConditionTypeDatabaseReady,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: cashuMint.Generation,
				Reason:             "PostgreSQLNotReady",
				Message:            "Waiting for PostgreSQL to be ready",
			})
		}
	}

	return nil
}

// reconcileBackup reconciles backup resources for PostgreSQL
func (r *CashuMintReconciler) reconcileBackup(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	cronJob, err := generators.GeneratePostgresBackupCronJob(cashuMint, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate backup CronJob: %w", err)
	}
	if cronJob == nil {
		meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
			Type:               mintv1alpha1.ConditionTypeBackupReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cashuMint.Generation,
			Reason:             "BackupNotReconciled",
			Message:            "Backup is enabled but no backup resources were generated",
		})
		return nil
	}

	if err := applyResource(ctx, r.Client, cronJob); err != nil {
		return fmt.Errorf("failed to apply backup CronJob: %w", err)
	}
	logger.Info("Backup CronJob reconciled", "cronjob", cronJob.Name)

	conditionReason := "BackupScheduleReady"
	conditionMessage := fmt.Sprintf("Backup CronJob %s reconciled", cronJob.Name)

	restoreJob, err := generators.GeneratePostgresRestoreJob(cashuMint, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate restore Job: %w", err)
	}
	if restoreJob != nil {
		if err := applyResource(ctx, r.Client, restoreJob); err != nil {
			return fmt.Errorf("failed to apply restore Job: %w", err)
		}
		logger.Info("Restore Job reconciled", "job", restoreJob.Name)
		conditionReason = "RestoreJobReconciled"
		conditionMessage = fmt.Sprintf("Restore Job %s reconciled for requested backup object", restoreJob.Name)
	}

	meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeBackupReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cashuMint.Generation,
		Reason:             conditionReason,
		Message:            conditionMessage,
	})

	return nil
}

// reconcileConfigMap reconciles the ConfigMap containing config.toml
func (r *CashuMintReconciler) reconcileConfigMap(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	// Fetch the postgres password if auto-provisioned.
	// reconcilePostgreSQL runs before this, so the secret should already exist.
	// If it doesn't exist yet, return an error to requeue rather than writing
	// an empty password into config.toml (which would cause auth failures in postgres).
	var dbPassword string
	if cashuMint.Spec.Database.Engine == mintv1alpha1.DatabaseEnginePostgres &&
		cashuMint.Spec.Database.Postgres != nil &&
		cashuMint.Spec.Database.Postgres.AutoProvision {
		secretName := cashuMint.Name + "-postgres-secret"
		secret := &corev1.Secret{}
		if err := r.Get(ctx, client.ObjectKey{
			Namespace: cashuMint.Namespace,
			Name:      secretName,
		}, secret); err != nil {
			return fmt.Errorf("postgres secret %s not ready yet: %w", secretName, err)
		}
		dbPassword = string(secret.Data["password"])
		if dbPassword == "" {
			return fmt.Errorf("postgres secret %s exists but password key is empty", secretName)
		}
	}

	configMap, err := generators.GenerateConfigMap(cashuMint, r.Scheme, dbPassword)
	if err != nil {
		return fmt.Errorf("failed to generate ConfigMap: %w", err)
	}

	if err := applyResource(ctx, r.Client, configMap); err != nil {
		return fmt.Errorf("failed to apply ConfigMap: %w", err)
	}

	logger.Info("ConfigMap reconciled", "configmap", configMap.Name)

	meta.SetStatusCondition(&cashuMint.Status.Conditions, metav1.Condition{
		Type:               mintv1alpha1.ConditionTypeConfigValid,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cashuMint.Generation,
		Reason:             "ConfigurationValid",
		Message:            "Configuration is valid and applied",
	})

	return nil
}

// reconcilePVC reconciles the PersistentVolumeClaim for local storage
func (r *CashuMintReconciler) reconcilePVC(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	pvc, err := generators.GeneratePVC(cashuMint, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate PVC: %w", err)
	}

	if pvc != nil {
		if err := applyResource(ctx, r.Client, pvc); err != nil {
			return fmt.Errorf("failed to apply PVC: %w", err)
		}
		logger.Info("PVC reconciled", "pvc", pvc.Name)
	}

	return nil
}

// reconcileDeployment reconciles the Deployment for the mint
func (r *CashuMintReconciler) reconcileDeployment(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	// Get ConfigMap to calculate hash
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      cashuMint.Name + "-config",
		Namespace: cashuMint.Namespace,
	}, configMap); err != nil {
		return fmt.Errorf("failed to get ConfigMap for hash calculation: %w", err)
	}

	configHash := calculateConfigHash(configMap)

	deployment, err := generators.GenerateDeployment(cashuMint, configHash, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate Deployment: %w", err)
	}

	if err := applyResource(ctx, r.Client, deployment); err != nil {
		return fmt.Errorf("failed to apply Deployment: %w", err)
	}

	logger.Info("Deployment reconciled", "deployment", deployment.Name, "configHash", configHash)

	return nil
}

// reconcileService reconciles the Service for the mint
func (r *CashuMintReconciler) reconcileService(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	service, err := generators.GenerateService(cashuMint, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate Service: %w", err)
	}

	if err := applyResource(ctx, r.Client, service); err != nil {
		return fmt.Errorf("failed to apply Service: %w", err)
	}

	logger.Info("Service reconciled", "service", service.Name)

	return nil
}

// reconcileIngress reconciles the Ingress for the mint
func (r *CashuMintReconciler) reconcileIngress(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	ingress, err := generators.GenerateIngress(cashuMint, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate Ingress: %w", err)
	}

	if ingress != nil {
		if err := applyResource(ctx, r.Client, ingress); err != nil {
			return fmt.Errorf("failed to apply Ingress: %w", err)
		}
		logger.Info("Ingress reconciled", "ingress", ingress.Name)
	}

	return nil
}

// reconcileCertificate reconciles the Certificate for the mint
func (r *CashuMintReconciler) reconcileCertificate(ctx context.Context, cashuMint *mintv1alpha1.CashuMint) error {
	logger := log.FromContext(ctx)

	cert, err := generators.GenerateCertificate(cashuMint, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to generate Certificate: %w", err)
	}

	if cert != nil {
		if err := applyResource(ctx, r.Client, cert); err != nil {
			return fmt.Errorf("failed to apply Certificate: %w", err)
		}
		logger.Info("Certificate reconciled", "certificate", cert.Name)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CashuMintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mintv1alpha1.CashuMint{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Secret{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&certmanagerv1.Certificate{}).
		Named("cashumint").
		Complete(r)
}
