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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
	"github.com/asmogo/cashu-operator/internal/resources"
	"github.com/asmogo/cashu-operator/internal/status"
)

// LNDReconciler implements lightning reconciliation for LND backend.
type LNDReconciler struct {
	*BaseReconciler
}

// NewLNDReconciler creates a new LND lightning reconciler.
func NewLNDReconciler(
	c client.Client,
	statusMgr *status.Manager,
	applier *resources.Applier,
) *LNDReconciler {
	return &LNDReconciler{
		BaseReconciler: &BaseReconciler{
			Client:        c,
			StatusManager: statusMgr,
			Applier:       applier,
		},
	}
}

// Name returns the reconciler name for logging.
func (lr *LNDReconciler) Name() string {
	return "LND"
}

// CanHandle returns true if the lightning config specifies LND.
func (lr *LNDReconciler) CanHandle(lnConfig *mintv1alpha1.LightningConfig) bool {
	return lnConfig != nil && lnConfig.Backend == "lnd"
}

// Reconcile implements the main reconciliation logic for LND.
func (lr *LNDReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get lightning configuration
	if mint.Spec.Lightning.Backend != "lnd" {
		return ctrl.Result{}, fmt.Errorf("LND reconciler called for non-lnd backend")
	}

	logger.Info("Using LND lightning backend", "mint", mint.Name)

	// TODO: Implement LND-specific reconciliation logic
	// This could include:
	// - Validating LND connection parameters
	// - Checking LND node health
	// - Creating required ConfigMaps/Secrets

	logger.Info("LND lightning backend reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}

// CLNReconciler implements lightning reconciliation for Core Lightning (CLN) backend.
type CLNReconciler struct {
	*BaseReconciler
}

// NewCLNReconciler creates a new Core Lightning (CLN) reconciler.
func NewCLNReconciler(
	c client.Client,
	statusMgr *status.Manager,
	applier *resources.Applier,
) *CLNReconciler {
	return &CLNReconciler{
		BaseReconciler: &BaseReconciler{
			Client:        c,
			StatusManager: statusMgr,
			Applier:       applier,
		},
	}
}

// Name returns the reconciler name for logging.
func (cr *CLNReconciler) Name() string {
	return "CoreLightning"
}

// CanHandle returns true if the lightning config specifies CLN.
func (cr *CLNReconciler) CanHandle(lnConfig *mintv1alpha1.LightningConfig) bool {
	return lnConfig != nil && lnConfig.Backend == "cln"
}

// Reconcile implements the main reconciliation logic for CLN.
func (cr *CLNReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get lightning configuration
	if mint.Spec.Lightning.Backend != "cln" {
		return ctrl.Result{}, fmt.Errorf("CLN reconciler called for non-cln backend")
	}

	logger.Info("Using Core Lightning (CLN) backend", "mint", mint.Name)

	// TODO: Implement CLN-specific reconciliation logic
	// This could include:
	// - Validating CLN connection parameters
	// - Checking CLN node health
	// - Creating required ConfigMaps/Secrets

	logger.Info("Core Lightning (CLN) backend reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}

// LNbitsReconciler implements lightning reconciliation for LNbits backend.
type LNbitsReconciler struct {
	*BaseReconciler
}

// NewLNbitsReconciler creates a new LNbits lightning reconciler.
func NewLNbitsReconciler(
	c client.Client,
	statusMgr *status.Manager,
	applier *resources.Applier,
) *LNbitsReconciler {
	return &LNbitsReconciler{
		BaseReconciler: &BaseReconciler{
			Client:        c,
			StatusManager: statusMgr,
			Applier:       applier,
		},
	}
}

// Name returns the reconciler name for logging.
func (lr *LNbitsReconciler) Name() string {
	return "LNbits"
}

// CanHandle returns true if the lightning config specifies LNbits.
func (lr *LNbitsReconciler) CanHandle(lnConfig *mintv1alpha1.LightningConfig) bool {
	return lnConfig != nil && lnConfig.Backend == "lnbits"
}

// Reconcile implements the main reconciliation logic for LNbits.
func (lr *LNbitsReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get lightning configuration
	if mint.Spec.Lightning.Backend != "lnbits" {
		return ctrl.Result{}, fmt.Errorf("LNbits reconciler called for non-lnbits backend")
	}

	logger.Info("Using LNbits lightning backend", "mint", mint.Name)

	// TODO: Implement LNbits-specific reconciliation logic
	// This could include:
	// - Validating LNbits API credentials
	// - Checking LNbits API availability
	// - Creating required ConfigMaps/Secrets

	logger.Info("LNbits lightning backend reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}
