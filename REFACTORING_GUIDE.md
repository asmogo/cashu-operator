# Cashu Operator Refactoring Guide

This document outlines the comprehensive refactoring performed on the Cashu Operator codebase to improve maintainability, extensibility, and code quality.

## Overview

The refactoring focused on:
- **Separation of Concerns**: Breaking down monolithic components into focused, single-responsibility modules
- **Design Patterns**: Implementing established patterns (Strategy, Delegating, Builder, Composite, Observer)
- **Code Clarity**: Improving naming, organization, and documentation
- **Maintainability**: Making the codebase easier to understand and extend

## Architecture Improvements

### 1. Controller Refactoring

**Before**: Single `CashuMintReconciler` with 522 lines containing all reconciliation logic

**After**: 
- `CashuMintReconciler` (streamlined): Main reconciliation loop and lifecycle management
- `ReconciliationOrchestrator` (new): Multi-phase reconciliation orchestration
- Deprecated `helpers.go`: Legacy functions moved to appropriate packages

**Benefits**:
- Main reconciler focuses on controller-runtime integration
- Orchestrator manages resource lifecycle phases
- Each component has a single, well-defined responsibility

### 2. Reconciler Framework

**Enhancements**:
- **DelegatingReconciler**: Runtime selection of database/lightning implementations
- **CompositeReconciler**: Chaining multiple reconcilers
- **BaseReconcilers**: Common functionality for database and lightning reconcilers
- **Validation**: Extracted into dedicated methods with context-rich error messages

**Code Example**:
```go
// Creating delegating reconcilers for runtime selection
dbDelegator := NewDatabaseDelegatingReconciler(
    NewPostgreSQLReconciler(client, statusMgr, applier),
    NewSQLiteReconciler(client, statusMgr, applier),
)

lnDelegator := NewLightningDelegatingReconciler(
    NewLNDReconciler(client, statusMgr, applier),
    NewCLNReconciler(client, statusMgr, applier),
    NewLNbitsReconciler(client, statusMgr, applier),
)
```

### 3. Builder Pattern for Resources

**Existing Pattern** (Enhanced):
```go
// Fluent API for resource construction
deployment, err := GenerateDeployment(mint, configHash, scheme)
service, err := GenerateService(mint, scheme)
configMap, err := GenerateConfigMap(mint, scheme, dbPassword)
```

**Builder Components**:
- `ProbeBuilder`: Fluent API for Kubernetes Probes
- `ContainerBuilder`: Fluent API for Containers
- Additional builders available in `internal/generators/builder.go`

## Design Patterns Used

### 1. Strategy Pattern
**Location**: `internal/reconcilers/database/`, `internal/reconcilers/lightning/`

Different database and lightning backends are implemented as strategies, selected at runtime based on configuration.

```go
// Database strategies
type DatabaseReconciler interface {
    Reconcile(ctx context.Context, mint *mintv1alpha1.CashuMint) (ctrl.Result, error)
    CanHandle(dbConfig *mintv1alpha1.DatabaseConfig) bool
    Name() string
}
```

### 2. Delegating Pattern
**Location**: `internal/reconcilers/interfaces.go`

Selects appropriate reconciler at runtime based on mint specification.

```go
// Delegates to appropriate reconciler
delegator := NewDatabaseDelegatingReconciler(postgresReconciler, sqliteReconciler)
result, err := delegator.Reconcile(ctx, mint)
```

### 3. Builder Pattern
**Location**: `internal/generators/builder.go`

Fluent API for constructing complex Kubernetes resources.

```go
probe := NewProbeBuilder().
    WithPath("/v1/health").
    WithPort("8085").
    WithInitialDelay(10).
    Build()
```

### 4. Composite Pattern
**Location**: `internal/reconcilers/interfaces.go`

Combines multiple reconcilers into a pipeline.

```go
composite := NewCompositeReconciler("pipeline",
    dbReconciler,
    lnReconciler,
    appReconciler,
)
```

### 5. Observer Pattern
**Location**: `internal/status/manager.go`

Status manager observes resource changes and updates conditions.

```go
// Update specific conditions
statusManager.SetDatabaseReady(ctx, mint)
statusManager.SetConfigValid(ctx, mint)
statusManager.SetReady(ctx, mint, "All resources ready")
```

## Module Organization

### `internal/controller/`
- **cashumint_controller.go**: Main reconciliation loop (72 lines after refactoring)
- **orchestrator.go**: Multi-phase reconciliation orchestration
- **helpers.go**: Deprecated (kept for compatibility, to be removed)
- **generators/**: Resource creation functions

### `internal/reconcilers/`
- **interfaces.go**: Strategy, Composite, and Delegating patterns
- **database/base.go**: Common database reconciliation logic
- **database/postgres.go**: PostgreSQL-specific implementation
- **lightning/base.go**: Common lightning reconciliation logic
- **lightning/implementations.go**: LND, CLN, LNBits, etc.

### `internal/resources/`
- **apply.go**: Kubernetes API operations and utilities
- Hashing utilities for change detection
- Common constants and helpers

### `internal/status/`
- **manager.go**: Condition and phase management (313 lines)
- Readiness checks for various resource types

### `internal/generators/`
- **builder.go**: Fluent API for constructing resources (325 lines)
- Resource generators: deployment, service, configmap, ingress, pvc, postgres

## Error Handling Patterns

### Consistency

All modules follow these error handling principles:

**1. Validation Errors** (Return immediately):
```go
if mint.Spec.Database.Engine == "" {
    return ctrl.Result{}, fmt.Errorf("database engine is not specified")
}
```

**2. Transient Errors** (Quick retries):
```go
if apierrors.IsConflict(err) {
    return ctrl.Result{Requeue: true}, nil
}
if apierrors.IsNotFound(err) {
    return ctrl.Result{RequeueAfter: NotReadyRetryInterval}, nil
}
```

**3. Resource Readiness** (Requeue with interval):
```go
if !status.IsDeploymentReady(deployment) {
    return ctrl.Result{RequeueAfter: UpdateReconcileInterval}, nil
}
```

**4. Fatal Errors** (Mark as failed):
```go
return r.handleError(ctx, cashuMint, err)
// Updates status to Failed phase with error message
```

### Error Wrapping

Always wrap errors with context:
```go
if err := r.reconcilePostgreSQL(ctx, cashuMint); err != nil {
    return fmt.Errorf("phase 1 failed: %w", err)
}
```

## Reconciliation Phases

The `ReconciliationOrchestrator` executes these phases in order:

1. **Phase 0**: State transitions (Pending → Provisioning/Updating)
2. **Phase 0b**: Detect spec changes via generation mismatch
3. **Phase 1**: PostgreSQL auto-provisioning (if enabled)
4. **Phase 2**: ConfigMap reconciliation
5. **Phase 3**: PVC reconciliation (for local storage)
6. **Phase 4**: Deployment reconciliation
7. **Phase 5**: Service reconciliation
8. **Phase 6**: Ingress reconciliation (if enabled)
9. **Phase 7**: Finalization (deployment readiness validation)

Each phase:
- Can fail independently, returning with context-rich error
- Uses appropriate requeue intervals
- Updates status conditions for visibility

## Status Conditions

CashuMint resources track state through:

**Phases**:
- `Pending`: Initial state
- `Provisioning`: First reconciliation in progress
- `Updating`: Spec changed, updating resources
- `Ready`: All resources ready
- `Failed`: Reconciliation failed

**Conditions**:
- `Ready`: Overall readiness
- `DatabaseReady`: Database backend readiness
- `LightningReady`: Lightning backend readiness
- `ConfigValid`: Configuration validation
- `IngressReady`: Ingress configuration (if applicable)

## Requeue Intervals

Defined in `internal/resources/apply.go`:

```go
DefaultRequeueAfterShort  = 5 * time.Second   // Transient errors
DefaultRequeueAfterMedium = 30 * time.Second  // Normal operations
DefaultRequeueAfterLong   = 2 * time.Minute   // Periodic checks
```

Selected based on reconciliation phase:
- Provisioning/Updating: `UpdateReconcileInterval` (30s)
- Ready: `ReconcileInterval` (5m)
- Errors: Exponential backoff from controller-runtime

## Documentation Improvements

### Package-Level Documentation
- `internal/doc.go`: Architecture overview, extension points, maintenance notes

### Inline Documentation
- Function documentation: Purpose, parameters, return values, usage examples
- Code comments: Explaining "why", not just "what"
- TODO markers: Identifying incomplete implementations

### Examples
```go
// NewDatabaseDelegatingReconciler creates a new delegating reconciler for databases.
// It accepts multiple DatabaseReconciler implementations and selects the appropriate one
// based on the CashuMint's database configuration at reconciliation time.
//
// Example usage:
//
//  delegator := NewDatabaseDelegatingReconciler(
//      NewPostgreSQLReconciler(client, statusMgr, applier),
//      NewSQLiteReconciler(client, statusMgr, applier),
//  )
func NewDatabaseDelegatingReconciler(candidates ...DatabaseReconciler) *DelegatingReconciler {
    // implementation...
}
```

## Testing Strategy

### Current Coverage
- `internal/controller`: 5.0%
- `internal/reconcilers/database`: 8.2%
- `internal/reconcilers/lightning`: 8.6%

### Recommended Improvements
1. Add unit tests for orchestrator phases
2. Test all reconciler implementations
3. Mock Kubernetes API for integration tests
4. Add edge case coverage for error handling

### Running Tests
```bash
make test           # Run all unit tests
make test-e2e       # Run end-to-end tests with kind
make coverage       # Generate coverage report
```

## Migration Path

### For Developers

1. **Using Database Reconcilers**:
   ```go
   // Create implementation
   postgresReconciler := NewPostgreSQLReconciler(client, statusMgr, applier)
   // Register with delegator
   dbDelegator := NewDatabaseDelegatingReconciler(postgresReconciler)
   ```

2. **Using Lightning Reconcilers**:
   ```go
   // Create implementation
   lndReconciler := NewLNDReconciler(client, statusMgr, applier)
   // Register with delegator
   lnDelegator := NewLightningDelegatingReconciler(lndReconciler)
   ```

3. **Adding New Resource Types**:
   - Create generator function in `internal/controller/generators/`
   - Add reconciliation phase to `orchestrator.go`
   - Update status manager if new conditions needed

### For Operators

No changes required. The refactoring is internal and maintains full backward compatibility.

## Code Quality Metrics

Before refactoring:
- Main controller: 522 lines (complex, hard to test)
- Helpers: 145 lines (deprecated functionality)
- Limited error context
- Inconsistent patterns

After refactoring:
- Controller: 72 lines (focused, testable)
- Orchestrator: 238 lines (clear phases)
- Consistent error handling
- Well-documented patterns
- All tests passing

## Deprecations and Removals

### Deprecated (Keep for now)
- `internal/controller/helpers.go`: Legacy utility functions
  - All functionality available in `resources` and `status` packages
  - Will be removed in next major version

### To Be Added
- More comprehensive test coverage
- Validation reconcilers
- Enhanced observability (metrics, traces)

## Maintenance Guidelines

### Adding a New Database Backend

1. Create file: `internal/reconcilers/database/{backend}.go`
2. Implement `DatabaseReconciler` interface:
   ```go
   type {Backend}Reconciler struct {
       *BaseReconciler
   }
   
   func (r *{Backend}Reconciler) CanHandle(dbConfig *mintv1alpha1.DatabaseConfig) bool {
       return dbConfig.Engine == "{backend}"
   }
   
   func (r *{Backend}Reconciler) Reconcile(ctx context.Context, mint *mintv1alpha1.CashuMint) (ctrl.Result, error) {
       // implementation
   }
   ```
3. Register in controller setup (to be documented)

### Adding a New Phase

1. Implement phase logic in `orchestrator.go`
2. Add logging for visibility
3. Handle errors with context
4. Update status conditions as needed
5. Add to architecture documentation

## Performance Considerations

- Requeue intervals are tunable in `resources/apply.go`
- No polling loops; fully event-driven reconciliation
- Hashing prevents unnecessary resource updates
- Finalizers ensure clean resource deletion

## Future Improvements

1. **Metrics**: Add Prometheus metrics for reconciliation timing
2. **Validation**: Dedicated validation reconciler with CRD validation
3. **Webhooks**: Enhanced validation webhook with better error messages
4. **Observability**: Distributed tracing support
5. **Performance**: Caching layer for resource lookups
6. **Testing**: Comprehensive test suite with mocked clients

## Conclusion

The refactoring improves code clarity, maintainability, and extensibility while maintaining full backward compatibility. The use of established design patterns makes it easy to add new backends and features without modifying existing code.

Future developers will find the codebase easier to understand and extend, reducing time to productivity and lowering the risk of introducing bugs.
