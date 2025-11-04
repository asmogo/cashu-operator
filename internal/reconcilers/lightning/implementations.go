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
// LND (Lightning Network Daemon) is a full-featured Lightning implementation.
// This method validates LND configuration and prepares the mint to use LND as its backend.
func (lr *LNDReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate lightning backend matches
	if mint.Spec.Lightning.Backend != "lnd" {
		return ctrl.Result{}, fmt.Errorf("LND reconciler called for non-lnd backend, got backend: %s", mint.Spec.Lightning.Backend)
	}

	// Validate LND configuration is present
	lndConfig := mint.Spec.Lightning.LND
	if lndConfig == nil {
		err := fmt.Errorf("LND configuration is missing but backend is set to 'lnd'")
		logger.Error(err, "Invalid lightning configuration", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	// Validate LND configuration
	if err := lr.validateLNDConfig(lndConfig); err != nil {
		logger.Error(err, "LND configuration validation failed", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	logger.Info("Using LND lightning backend", "mint", mint.Name, "lnd-address", lndConfig.Address)

	// TODO: Implement LND-specific reconciliation logic:
	// - Create ConfigMaps/Secrets for LND connection details
	// - Validate LND connectivity and credentials
	// - Handle TLS certificates if configured
	// - Check LND node health and readiness

	logger.Info("LND lightning backend reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}

// validateLNDConfig validates the LND configuration.
// Ensures required connection parameters are provided.
func (lr *LNDReconciler) validateLNDConfig(lndConfig *mintv1alpha1.LNDConfig) error {
	if lndConfig.Address == "" {
		return fmt.Errorf("LND address must be specified")
	}
	// Additional validation can be added here as needed
	// Examples: address format validation, certificate path checks, etc.
	return nil
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

// Reconcile implements the main reconciliation logic for CLN (Core Lightning).
// CLN is a BOLT#1-compliant Lightning implementation optimized for resource efficiency.
// This method validates CLN configuration and prepares the mint to use CLN as its backend.
func (cr *CLNReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate lightning backend matches
	if mint.Spec.Lightning.Backend != "cln" {
		return ctrl.Result{}, fmt.Errorf("CLN reconciler called for non-cln backend, got backend: %s", mint.Spec.Lightning.Backend)
	}

	// Validate CLN configuration is present
	clnConfig := mint.Spec.Lightning.CLN
	if clnConfig == nil {
		err := fmt.Errorf("CLN configuration is missing but backend is set to 'cln'")
		logger.Error(err, "Invalid lightning configuration", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	// Validate CLN configuration
	if err := cr.validateCLNConfig(clnConfig); err != nil {
		logger.Error(err, "CLN configuration validation failed", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	logger.Info("Using Core Lightning (CLN) backend", "mint", mint.Name, "rpc-path", clnConfig.RPCPath)

	// TODO: Implement CLN-specific reconciliation logic:
	// - Create ConfigMaps/Secrets for CLN connection details
	// - Validate CLN RPC socket accessibility
	// - Mount RPC socket volume if needed
	// - Check CLN node health and readiness

	logger.Info("Core Lightning (CLN) backend reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}

// validateCLNConfig validates the CLN configuration.
// Ensures required RPC path is specified.
func (cr *CLNReconciler) validateCLNConfig(clnConfig *mintv1alpha1.CLNConfig) error {
	if clnConfig.RPCPath == "" {
		return fmt.Errorf("CLN RPC path must be specified")
	}
	return nil
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
// LNbits is a lightweight wallet management system that provides an HTTP API for Lightning operations.
// This method validates LNbits configuration and prepares the mint to use LNbits as its backend.
func (lr *LNbitsReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate lightning backend matches
	if mint.Spec.Lightning.Backend != "lnbits" {
		return ctrl.Result{}, fmt.Errorf("LNbits reconciler called for non-lnbits backend, got backend: %s", mint.Spec.Lightning.Backend)
	}

	// Validate LNbits configuration is present
	lnbitsConfig := mint.Spec.Lightning.LNBits
	if lnbitsConfig == nil {
		err := fmt.Errorf("LNbits configuration is missing but backend is set to 'lnbits'")
		logger.Error(err, "Invalid lightning configuration", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	// Validate LNbits configuration
	if err := lr.validateLNbitsConfig(lnbitsConfig); err != nil {
		logger.Error(err, "LNbits configuration validation failed", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	logger.Info("Using LNbits lightning backend", "mint", mint.Name, "api-url", lnbitsConfig.API)

	// TODO: Implement LNbits-specific reconciliation logic:
	// - Create Secrets for API keys (admin and invoice)
	// - Validate LNbits API connectivity and credentials
	// - Check LNbits wallet health and readiness
	// - Handle RetroAPI compatibility if needed

	logger.Info("LNbits lightning backend reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}

// validateLNbitsConfig validates the LNbits configuration.
// Ensures required API credentials and URL are specified.
func (lr *LNbitsReconciler) validateLNbitsConfig(lnbitsConfig *mintv1alpha1.LNBitsConfig) error {
	if lnbitsConfig.API == "" {
		return fmt.Errorf("LNbits API URL must be specified")
	}
	if lnbitsConfig.AdminAPIKeySecretRef.Name == "" {
		return fmt.Errorf("LNbits admin API key secret reference must be specified")
	}
	if lnbitsConfig.InvoiceAPIKeySecretRef.Name == "" {
		return fmt.Errorf("LNbits invoice API key secret reference must be specified")
	}
	return nil
}
