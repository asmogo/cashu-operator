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

package reconcilers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// Reconciler defines the interface for a resource reconciler.
// This design pattern allows for easy extension with new reconcilers
// for different resources or backends without modifying the main controller.
type Reconciler interface {
	// Reconcile performs the reconciliation logic for a specific resource.
	// It should return a reconciliation result and any error that occurred.
	Reconcile(ctx context.Context, mint *mintv1alpha1.CashuMint) (ctrl.Result, error)

	// Name returns the name of this reconciler for logging purposes.
	Name() string
}

// DatabaseReconciler defines the interface for database-specific reconciliation logic.
// Different database backends (PostgreSQL, SQLite, etc.) implement this interface.
type DatabaseReconciler interface {
	Reconciler

	// CanHandle returns true if this reconciler can handle the given database configuration.
	CanHandle(dbConfig *mintv1alpha1.DatabaseConfig) bool
}

// LightningReconciler defines the interface for Lightning backend-specific reconciliation.
// Different Lightning backends (LND, CLN, LNBits, etc.) implement this interface.
type LightningReconciler interface {
	Reconciler

	// CanHandle returns true if this reconciler can handle the given Lightning configuration.
	CanHandle(lnConfig *mintv1alpha1.LightningConfig) bool
}

// CompositeReconciler manages multiple reconcilers and executes them sequentially.
// It's useful for orchestrating complex reconciliation workflows with multiple steps.
type CompositeReconciler struct {
	reconcilers []Reconciler
	name        string
}

// NewCompositeReconciler creates a new composite reconciler.
func NewCompositeReconciler(name string, reconcilers ...Reconciler) *CompositeReconciler {
	return &CompositeReconciler{
		reconcilers: reconcilers,
		name:        name,
	}
}

// Reconcile executes all sub-reconcilers in sequence.
func (cr *CompositeReconciler) Reconcile(ctx context.Context, mint *mintv1alpha1.CashuMint) (ctrl.Result, error) {
	for _, reconciler := range cr.reconcilers {
		result, err := reconciler.Reconcile(ctx, mint)
		if err != nil {
			return ctrl.Result{}, err
		}
		// If any reconciler requests a requeue, propagate it
		if result.Requeue || result.RequeueAfter > 0 {
			return result, nil
		}
	}
	return ctrl.Result{}, nil
}

// Name returns the name of the composite reconciler.
func (cr *CompositeReconciler) Name() string {
	return cr.name
}

// DelegatingReconciler finds and delegates to the appropriate reconciler based on configuration.
// This pattern is useful for selecting implementations at runtime.
type DelegatingReconciler struct {
	candidates []interface{} // Can be DatabaseReconciler or LightningReconciler
	name       string
}

// NewDatabaseDelegatingReconciler creates a new delegating reconciler for databases.
func NewDatabaseDelegatingReconciler(candidates ...DatabaseReconciler) *DelegatingReconciler {
	delegateCandidates := make([]interface{}, len(candidates))
	for i, c := range candidates {
		delegateCandidates[i] = c
	}
	return &DelegatingReconciler{
		candidates: delegateCandidates,
		name:       "DatabaseDelegate",
	}
}

// NewLightningDelegatingReconciler creates a new delegating reconciler for Lightning.
func NewLightningDelegatingReconciler(candidates ...LightningReconciler) *DelegatingReconciler {
	delegateCandidates := make([]interface{}, len(candidates))
	for i, c := range candidates {
		delegateCandidates[i] = c
	}
	return &DelegatingReconciler{
		candidates: delegateCandidates,
		name:       "LightningDelegate",
	}
}

// Name returns the name of the delegating reconciler.
func (dr *DelegatingReconciler) Name() string {
	return dr.name
}
