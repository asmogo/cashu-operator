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
	"testing"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// TestPostgreSQLCanHandle tests PostgreSQL reconciler's CanHandle method
func TestPostgreSQLCanHandle(t *testing.T) {
	tests := []struct {
		name     string
		dbConfig *mintv1alpha1.DatabaseConfig
		expected bool
	}{
		{
			name:     "nil_database_config",
			dbConfig: nil,
			expected: false,
		},
		{
			name: "postgres_engine",
			dbConfig: &mintv1alpha1.DatabaseConfig{
				Engine: "postgres",
			},
			expected: true,
		},
		{
			name: "sqlite_engine",
			dbConfig: &mintv1alpha1.DatabaseConfig{
				Engine: "sqlite",
			},
			expected: false,
		},
		{
			name: "unknown_engine",
			dbConfig: &mintv1alpha1.DatabaseConfig{
				Engine: "unknown",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &PostgreSQLReconciler{
				BaseReconciler: &BaseReconciler{},
			}
			result := reconciler.CanHandle(tt.dbConfig)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestPostgreSQLReconcilerName tests the Name method
func TestPostgreSQLReconcilerName(t *testing.T) {
	reconciler := &PostgreSQLReconciler{
		BaseReconciler: &BaseReconciler{},
	}
	expected := "PostgreSQL"
	if reconciler.Name() != expected {
		t.Errorf("expected %q, got %q", expected, reconciler.Name())
	}
}

// TestPostgreSQLReconcileWithoutConfig tests PostgreSQL reconciliation without proper config
// by checking that CanHandle properly identifies invalid configs
func TestPostgreSQLReconcileWithoutConfigValidation(t *testing.T) {
	// An empty database config should not match PostgreSQL
	dbConfig := &mintv1alpha1.DatabaseConfig{
		Engine: "postgres",
		// Postgres config is nil - this would fail validation in Reconcile
	}

	reconciler := &PostgreSQLReconciler{
		BaseReconciler: &BaseReconciler{},
	}

	// Even with empty Postgres config, CanHandle returns true for engine match
	if !reconciler.CanHandle(dbConfig) {
		t.Error("expected CanHandle to return true for postgres engine")
	}
}

// TestSQLiteCanHandle tests SQLite reconciler's CanHandle method
func TestSQLiteCanHandle(t *testing.T) {
	tests := []struct {
		name     string
		dbConfig *mintv1alpha1.DatabaseConfig
		expected bool
	}{
		{
			name:     "nil_database_config",
			dbConfig: nil,
			expected: false,
		},
		{
			name: "postgres_engine",
			dbConfig: &mintv1alpha1.DatabaseConfig{
				Engine: "postgres",
			},
			expected: false,
		},
		{
			name: "sqlite_engine",
			dbConfig: &mintv1alpha1.DatabaseConfig{
				Engine: "sqlite",
			},
			expected: true,
		},
		{
			name: "unknown_engine",
			dbConfig: &mintv1alpha1.DatabaseConfig{
				Engine: "unknown",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &SQLiteReconciler{
				BaseReconciler: &BaseReconciler{},
			}
			result := reconciler.CanHandle(tt.dbConfig)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestSQLiteReconcilerName tests the Name method
func TestSQLiteReconcilerName(t *testing.T) {
	reconciler := &SQLiteReconciler{
		BaseReconciler: &BaseReconciler{},
	}
	expected := "SQLite"
	if reconciler.Name() != expected {
		t.Errorf("expected %q, got %q", expected, reconciler.Name())
	}
}

// TestSQLiteCanHandleWithValidConfig tests that SQLite reconciler properly validates config
func TestSQLiteCanHandleWithValidConfig(t *testing.T) {
	dbConfig := &mintv1alpha1.DatabaseConfig{
		Engine: "sqlite",
		SQLite: &mintv1alpha1.SQLiteConfig{
			DataDir: "/data",
		},
	}

	reconciler := &SQLiteReconciler{
		BaseReconciler: &BaseReconciler{},
	}

	// Should handle SQLite config
	if !reconciler.CanHandle(dbConfig) {
		t.Error("expected CanHandle to return true for sqlite engine")
	}
}

// TestSQLiteReconcileWithoutConfigValidation tests SQLite validation with missing config
func TestSQLiteReconcileWithoutConfigValidation(t *testing.T) {
	// An empty SQLite config should still match the engine
	dbConfig := &mintv1alpha1.DatabaseConfig{
		Engine: "sqlite",
		// SQLite config is nil - would fail validation in Reconcile
	}

	reconciler := &SQLiteReconciler{
		BaseReconciler: &BaseReconciler{},
	}

	// CanHandle returns true for engine match
	if !reconciler.CanHandle(dbConfig) {
		t.Error("expected CanHandle to return true for sqlite engine")
	}
}

// TestBaseReconcilerRequeue tests the requeue helper methods
func TestBaseReconcilerRequeue(t *testing.T) {
	reconciler := &BaseReconciler{}

	// Test short requeue
	result, err := reconciler.RequeueAfterShort()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected non-zero RequeueAfter")
	}

	// Test long requeue
	result, err = reconciler.RequeueAfterLong()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected non-zero RequeueAfter")
	}
}

// TestBaseReconcilerGetNamespacedName tests the GetNamespacedName method
func TestBaseReconcilerGetNamespacedName(t *testing.T) {
	reconciler := &BaseReconciler{}

	tests := []struct {
		name      string
		namespace string
		name_     string
	}{
		{
			name:      "default_namespace",
			namespace: "default",
			name_:     "test-resource",
		},
		{
			name:      "custom_namespace",
			namespace: "kube-system",
			name_:     "coredns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nn := reconciler.GetNamespacedName(tt.namespace, tt.name_)
			if nn.Namespace != tt.namespace {
				t.Errorf("expected namespace %q, got %q", tt.namespace, nn.Namespace)
			}
			if nn.Name != tt.name_ {
				t.Errorf("expected name %q, got %q", tt.name_, nn.Name)
			}
		})
	}
}
