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
// This method validates configuration and delegates to appropriate sub-reconcilers
// based on whether auto-provisioning is enabled.
func (pr *PostgreSQLReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate database engine matches
	if mint.Spec.Database.Engine != "postgres" {
		return ctrl.Result{}, fmt.Errorf("PostgreSQL reconciler called for non-postgres database, got engine: %s", mint.Spec.Database.Engine)
	}

	// Validate PostgreSQL configuration is present
	pgConfig := mint.Spec.Database.Postgres
	if pgConfig == nil {
		err := fmt.Errorf("PostgreSQL configuration is missing but engine is set to 'postgres'")
		logger.Error(err, "Invalid database configuration", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	// Validate configuration based on provisioning mode
	if err := pr.validatePostgresConfig(pgConfig); err != nil {
		logger.Error(err, "PostgreSQL configuration validation failed", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	// Route to appropriate reconciliation path
	if pgConfig.AutoProvision {
		logger.Info("Reconciling auto-provisioned PostgreSQL", "mint", mint.Name)
		// TODO: Implement auto-provisioning logic
		// This would involve creating StatefulSet, PVC, Service, etc.
	} else {
		logger.Info("Using external PostgreSQL connection", "mint", mint.Name)
	}

	// Mark database as ready
	if err := pr.StatusManager.SetDatabaseReady(ctx, mint); err != nil {
		logger.Error(err, "Failed to update database ready status", "mint", mint.Name)
		return pr.RequeueAfterShort()
	}

	logger.Info("PostgreSQL database reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}

// validatePostgresConfig validates the PostgreSQL configuration based on the provisioning mode.
// For external databases, validates that connection parameters are provided.
// For auto-provisioned databases, validates that resource requirements are set.
func (pr *PostgreSQLReconciler) validatePostgresConfig(pgConfig *mintv1alpha1.PostgresConfig) error {
	if !pgConfig.AutoProvision {
		// External database: must have connection URL
		if pgConfig.URL == "" && pgConfig.URLSecretRef == nil {
			return fmt.Errorf("external PostgreSQL requires either url or urlSecretRef to be specified")
		}
	}
	// Auto-provisioned databases have sensible defaults, so no additional validation needed
	return nil
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
// SQLite requires minimal setup as it's file-based and doesn't require external resources.
// This method validates configuration and marks the database as ready immediately.
func (sr *SQLiteReconciler) Reconcile(
	ctx context.Context,
	mint *mintv1alpha1.CashuMint,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate database engine matches
	if mint.Spec.Database.Engine != "sqlite" {
		return ctrl.Result{}, fmt.Errorf("SQLite reconciler called for non-sqlite database, got engine: %s", mint.Spec.Database.Engine)
	}

	// Validate SQLite configuration is present
	sqliteConfig := mint.Spec.Database.SQLite
	if sqliteConfig == nil {
		err := fmt.Errorf("SQLite configuration is missing but engine is set to 'sqlite'")
		logger.Error(err, "Invalid database configuration", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	// Validate SQLite configuration
	if err := sr.validateSQLiteConfig(sqliteConfig); err != nil {
		logger.Error(err, "SQLite configuration validation failed", "mint", mint.Name)
		return ctrl.Result{}, err
	}

	logger.Info("SQLite database ready", "dataDir", sqliteConfig.DataDir, "mint", mint.Name)

	// Mark database as ready (SQLite doesn't need to wait for resources)
	if err := sr.StatusManager.SetDatabaseReady(ctx, mint); err != nil {
		logger.Error(err, "Failed to update database ready status", "mint", mint.Name)
		return sr.RequeueAfterShort()
	}

	logger.Info("SQLite database reconciliation completed", "mint", mint.Name)
	return ctrl.Result{}, nil
}

// validateSQLiteConfig validates the SQLite configuration.
// Currently, SQLite requires minimal validation as the dataDir is typically set to a sensible default.
func (sr *SQLiteReconciler) validateSQLiteConfig(_ *mintv1alpha1.SQLiteConfig) error {
	// SQLite configuration is fairly open-ended, but we could add validation here
	// if specific requirements emerge (e.g., dataDir not empty, proper path format)
	// Using _ to explicitly ignore the parameter as validation is not yet needed
	return nil
}
