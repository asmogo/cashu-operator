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

// Package internal contains the core implementation of the Cashu Mint Operator.
//
// Architecture Overview:
//
// The operator follows a layered architecture:
//
// 1. Controller Layer (controller/)
//   - Main reconciliation loop: CashuMintReconciler
//   - Orchestration: ReconciliationOrchestrator (manages multi-phase reconciliation)
//   - Resource generators: Create Kubernetes manifests for all required resources
//   - Deprecated helpers: Legacy utility functions (to be removed)
//
// 2. Reconciler Layer (reconcilers/)
//   - Pluggable reconciliation backends for databases and lightning nodes
//   - Strategy pattern: Database and Lightning reconcilers
//   - Delegating reconcilers: Runtime selection of appropriate implementation
//   - Interfaces: Common contracts for all reconcilers
//
// 3. Resource Management (resources/)
//   - Applier: High-level Kubernetes API operations
//   - Hash utilities: ConfigMap and Secret change detection
//   - Common constants: Requeue intervals and timing configuration
//
// 4. Status Management (status/)
//   - Manager: Coordinate condition and phase updates
//   - Status helpers: Readiness checks for various Kubernetes objects
//
// 5. Generators (controller/generators/)
//   - Resource builders: Fluent API for constructing Kubernetes objects
//   - Generator functions: Create specific resources (Deployment, Service, etc.)
//
// Reconciliation Flow:
//
// The reconciliation process follows these phases:
//
//  1. CashuMintReconciler.Reconcile() - Main entry point
//  2. Fetch CashuMint resource and validate state
//  3. Handle deletions (with finalizers) or initialization
//  4. ReconciliationOrchestrator.Orchestrate() - Multi-phase reconciliation:
//     - Phase sync (Pending → Provisioning/Updating)
//     - PostgreSQL auto-provisioning (if enabled)
//     - ConfigMap reconciliation
//     - PVC reconciliation (for local storage)
//     - Deployment reconciliation
//     - Service reconciliation
//     - Ingress reconciliation (if enabled)
//     - Status validation and finalization
//  5. Update final status and return requeue result
//
// Design Patterns Used:
//
//   - Strategy Pattern: Different reconcilers for databases and lightning nodes
//   - Delegating Pattern: Runtime selection of appropriate reconciler
//   - Builder Pattern: Fluent API for constructing Kubernetes resources
//   - Composite Pattern: Combining multiple reconcilers
//   - Observer Pattern: Status management and condition updates
//
// Extension Points:
//
// To add support for a new database backend:
//  1. Implement DatabaseReconciler interface in reconcilers/database/
//  2. Register with DelegatingReconciler in the controller
//
// To add support for a new lightning backend:
//  1. Implement LightningReconciler interface in reconcilers/lightning/
//  2. Register with DelegatingReconciler in the controller
//
// Error Handling:
//
// The operator uses a consistent error handling strategy:
//   - Validation errors: Return immediately (usually during initialization)
//   - Transient errors (API conflicts, not-found): Quick retries with backoff
//   - Resource readiness errors: Requeue with appropriate interval
//   - Fatal errors: Mark resource as Failed and log details
//
// Status Conditions:
//
// CashuMint resources track their state through a combination of:
//   - Phase: Overall lifecycle stage (Pending, Provisioning, Ready, Failed, etc.)
//   - Conditions: Specific readiness indicators for subsystems:
//   - Ready: Overall resource is ready for use
//   - DatabaseReady: Database backend is ready
//   - LightningReady: Lightning backend is ready
//   - ConfigValid: Configuration is valid
//   - IngressReady: Ingress is configured (if enabled)
//
// Maintenance Notes:
//
// - helpers.go is deprecated; helper functions have been moved to appropriate packages
// - The orchestrator can be extended with additional phases
// - Consider adding validation reconcilers in the future
// - Performance: Requeue intervals are configurable in resources.go
package internal
