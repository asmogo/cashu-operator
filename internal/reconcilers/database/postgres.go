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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
	"github.com/asmogo/cashu-operator/internal/resources"
	"github.com/asmogo/cashu-operator/internal/status"
)

// PostgreSQLReconciler implements database reconciliation for PostgreSQL.
// It handles both external PostgreSQL connections and auto-provisioned instances.
type PostgreSQLReconciler struct {
	*BaseReconciler
}

// NewPostgreSQLReconciler creates a new PostgreSQL database reconciler.
func NewPostgreSQLReconciler(
	c client.Client,
	statusMgr *status.Manager,
	applier *resources.Applier,
) *PostgreSQLReconciler {
	return &PostgreSQLReconciler{
		BaseReconciler: &BaseReconciler{
			Client:        c,
			StatusManager: statusMgr,
			Applier:       applier,
		},
	}
}

// Name returns the reconciler name for logging.
func (pr *PostgreSQLReconciler) Name() string {
	return "PostgreSQL"
}

// CanHandle returns true if the database config specifies PostgreSQL.
func (pr *PostgreSQLReconciler) CanHandle(dbConfig *mintv1alpha1.DatabaseConfig) bool {
	return dbConfig != nil && dbConfig.Engine == "postgres"
}

// Reconcile implements the main reconciliation logic for PostgreSQL databases.
// It handles both external connections and auto-provisioning scenarios.
func (pr *PostgreSQLReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get database configuration
	if mint.Spec.Database.Engine != "postgres" {
		return ctrl.Result{}, fmt.Errorf("PostgreSQL reconciler called for non-postgres database")
	}

	pgConfig := mint.Spec.Database.Postgres
	if pgConfig == nil {
		err := fmt.Errorf("PostgreSQL configuration is missing")
		logger.Error(err, "Database configuration invalid", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	// If auto-provisioning is enabled, reconcile the PostgreSQL instance
	if pgConfig.AutoProvision {
		logger.Info("Auto-provisioning PostgreSQL database", "mint", mint.Name)
		// TODO: Implement auto-provisioning logic
		// This would involve creating StatefulSet, PVC, Service, etc.
	} else {
		// For external PostgreSQL, just validate the configuration exists
		logger.Info("Using external PostgreSQL database", "mint", mint.Name)
		if pgConfig.URL == "" && pgConfig.URLSecretRef == nil {
			return ctrl.Result{}, fmt.Errorf("PostgreSQL URL or URLSecretRef must be provided")
		}
	}

	// Mark database as ready
	if err := pr.StatusManager.SetDatabaseReady(ctx, mint); err != nil {
		logger.Error(err, "Failed to update database ready status", "mint", mint.Name)
		return pr.RequeueAfterShort()
	}

	logger.Info("PostgreSQL database reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}

// SQLiteReconciler implements database reconciliation for SQLite.
// SQLite is a file-based database that doesn't require provisioning.
type SQLiteReconciler struct {
	*BaseReconciler
}

// NewSQLiteReconciler creates a new SQLite database reconciler.
func NewSQLiteReconciler(
	c client.Client,
	statusMgr *status.Manager,
	applier *resources.Applier,
) *SQLiteReconciler {
	return &SQLiteReconciler{
		BaseReconciler: &BaseReconciler{
			Client:        c,
			StatusManager: statusMgr,
			Applier:       applier,
		},
	}
}

// Name returns the reconciler name for logging.
func (sr *SQLiteReconciler) Name() string {
	return "SQLite"
}

// CanHandle returns true if the database config specifies SQLite.
func (sr *SQLiteReconciler) CanHandle(dbConfig *mintv1alpha1.DatabaseConfig) bool {
	return dbConfig != nil && dbConfig.Engine == "sqlite"
}

// Reconcile implements the main reconciliation logic for SQLite databases.
// SQLite requires minimal setup as it's file-based.
func (sr *SQLiteReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get SQLite configuration
	if mint.Spec.Database.Engine != "sqlite" {
		return ctrl.Result{}, fmt.Errorf("SQLite reconciler called for non-sqlite database")
	}

	sqliteConfig := mint.Spec.Database.SQLite
	if sqliteConfig == nil {
		err := fmt.Errorf("SQLite configuration is missing")
		logger.Error(err, "Database configuration invalid", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	logger.Info("SQLite database is ready", "dataDir", sqliteConfig.DataDir, "mint", mint.Name)

	// Mark database as ready (SQLite doesn't need to wait for resources)
	if err := sr.StatusManager.SetDatabaseReady(ctx, mint); err != nil {
		logger.Error(err, "Failed to update database ready status", "mint", mint.Name)
		return sr.RequeueAfterShort()
	}

	return ctrl.Result{}, nil
}
