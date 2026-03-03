# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes operator that provides full lifecycle automation for running CDK mintd (Cashu mints) inside Kubernetes. The operator translates declarative `CashuMint` custom resources into complete mint deployments with database, Lightning backend, ingress, TLS, and optional LDK sidecar support.

Built with Kubebuilder/controller-runtime and designed for production environments requiring repeatable deployments, configuration drift detection, and continuous health reconciliation.

## Development Commands

### Building and Running

```bash
# Build the controller binary
make build

# Run controller locally against active kubeconfig
make run

# Build Docker image
make docker-build IMG=<image-name>:<tag>

# Push Docker image
make docker-push IMG=<image-name>:<tag>
```

### Code Generation

```bash
# Generate manifests (CRDs, RBAC, Webhook configs)
make manifests

# Generate DeepCopy methods
make generate

# Format code
make fmt

# Run static analysis
make vet

# Run linter
make lint

# Run linter with auto-fixes
make lint-fix
```

### Testing

```bash
# Run unit tests (excludes e2e)
make test

# Run e2e tests (provisions kind cluster automatically)
make test-e2e

# Clean up e2e test cluster
make cleanup-test-e2e

# The test target runs: manifests, generate, fmt, vet, then tests with coverage
# E2E tests use Kind cluster named 'cashu-operator-test-e2e' by default
```

### Deployment

```bash
# Install CRDs to current cluster
make install

# Deploy operator to current cluster
make deploy IMG=<image-name>:<tag>

# Undeploy operator from current cluster
make undeploy

# Uninstall CRDs from current cluster
make uninstall

# Generate consolidated install.yaml bundle
make build-installer
# Output: dist/install.yaml
```

## Architecture

### Core Components

**API Types** (`api/v1alpha1/`):
- `cashumint_types.go`: CRD schema with comprehensive validation markers for database, Lightning backends (LND, CLN, LNBits, FakeWallet, gRPC Processor), ingress, TLS, authentication, and operational settings
- `cashumint_webhook.go`: Admission webhook for validation and defaulting logic

**Controller** (`internal/controller/`):
- `cashumint_controller.go`: Main reconciliation loop with phased resource management:
  1. PostgreSQL auto-provisioning (if enabled)
  2. ConfigMap reconciliation
  3. PVC reconciliation (for SQLite/redb)
  4. Deployment reconciliation with config-hash based rolling updates
  5. Service reconciliation
  6. Ingress reconciliation (if enabled)
- `helpers.go`: Utility functions for resource application and status checks

**Resource Generators** (`internal/controller/generators/`):
- `deployment.go`: Deployment with mint container and optional LDK sidecar
- `service.go`: Service with configurable type (ClusterIP/NodePort/LoadBalancer)
- `ingress.go`: Ingress with cert-manager integration
- `configmap.go`: ConfigMap with TOML configuration for CDK mintd
- `postgres.go`: PostgreSQL StatefulSet, Service, and Secret for auto-provisioning
- `pvc.go`: PersistentVolumeClaim for local storage backends

### Reconciliation Strategy

The controller uses a phased reconciliation approach with status conditions tracking:
- **Phase tracking**: Pending → Provisioning → Ready/Updating/Failed
- **Status conditions**: `Ready`, `DatabaseReady`, `LightningReady`, `ConfigValid`, `IngressReady`
- **Config-driven updates**: ConfigMap hash annotation triggers rolling Deployment updates
- **Requeue intervals**: 5 minutes (normal), 30 seconds (updating), 10 seconds (not ready)
- **Finalizer**: Handles cleanup on CashuMint deletion

### Database Backends

- **PostgreSQL**: Auto-provisioned (StatefulSet) or external (Secret-based credentials)
- **SQLite**: Embedded with PVC for persistence
- **redb**: Embedded with PVC for persistence

Auto-provisioned PostgreSQL creates: Secret (with generated password preserved across reconciliations), Service, StatefulSet with configurable storage and resources.

### Lightning Backends

Supported backends with backend-specific configuration:
- **LND**: gRPC with macaroon and TLS certificate authentication
- **CLN**: Unix socket RPC path
- **LNBits**: HTTP API with admin and invoice API keys
- **FakeWallet**: Testing backend with configurable delays
- **gRPC Processor**: Custom gRPC payment processor with optional TLS

### Sidecar Support

Optional LDK node sidecar with configuration for:
- Bitcoin network (mainnet/testnet/signet/regtest)
- Chain source (Esplora/Bitcoin RPC)
- Gossip source (P2P/RGS)
- Storage (local path or PostgreSQL)
- P2P and management ports

## Key Implementation Patterns

### Resource Application
- Uses `applyResource()` helper with server-side apply semantics
- Sets owner references for garbage collection
- Handles updates to immutable fields by recreation where needed

### Configuration Hash
- ConfigMap data is hashed and added as pod annotation: `config-hash`
- Forces rolling update when configuration changes
- Calculated in `helpers.go:calculateConfigHash()`

### Status Management
- Phase transitions: Pending → Provisioning → Ready/Updating/Failed
- Conditions use standard metav1.Condition with typed condition types (constants in `cashumint_types.go`)
- Status subresource updated after each reconciliation
- ObservedGeneration tracks spec vs status alignment

### Secret References
- Sensitive data (mnemonics, passwords, API keys) always via SecretKeySelector
- Never stored directly in spec
- Auto-provisioned PostgreSQL Secret preserves existing passwords across reconciliations

### Ingress and TLS
- Optional cert-manager integration via annotations
- TLS secret auto-generated or user-provided
- Ingress class configurable (default: nginx)
- Status URL updated based on ingress readiness

## Testing Guidelines

### Unit Tests
- Controller tests in `internal/controller/cashumint_controller_test.go`
- Use envtest for simulating Kubernetes API server
- Test files should use controller-runtime's Ginkgo/Gomega setup

### E2E Tests
- Located in `test/e2e/`
- Automatically provision Kind cluster
- Test full operator deployment and CashuMint lifecycle
- Clean up cluster after test completion

### Test Data
- Sample CRs in `config/samples/`:
  - `mint_v1alpha1_cashumint_minimal.yaml`: FakeWallet + SQLite
  - `mint_v1alpha1_cashumint_postgres_auto.yaml`: Auto-provisioned PostgreSQL
  - `mint_v1alpha1_cashumint_external_postgres.yaml`: External PostgreSQL
  - `mint_v1alpha1_cashumint_lnd.yaml`: LND Lightning backend
  - `mint_v1alpha1_cashumint_production.yaml`: Full production setup with TLS

## Common Patterns

### Adding New CRD Fields
1. Add field to `api/v1alpha1/cashumint_types.go` with kubebuilder markers
2. Update webhook validation in `api/v1alpha1/cashumint_webhook.go` if needed
3. Run `make manifests generate` to update generated code
4. Update relevant generator in `internal/controller/generators/`
5. Update controller reconciliation logic if needed
6. Update sample manifests in `config/samples/`

### Adding New Lightning Backend
1. Add backend type to `LightningConfig.Backend` enum in `cashumint_types.go`
2. Define backend-specific config struct (e.g., `MyBackendConfig`)
3. Add optional field to `LightningConfig`
4. Update `generators/configmap.go` to generate TOML config for new backend
5. Add webhook validation for required fields
6. Update documentation and add sample manifest

### Config Hash Calculation
When changing ConfigMap generation:
- Hash is calculated from ConfigMap.Data in `helpers.go:calculateConfigHash()`
- Hash triggers pod restart when config changes
- Ensure sensitive data uses Secret mounts, not ConfigMap, to avoid hash exposure

## Project Dependencies

- **Go**: 1.24.5
- **Kubernetes**: v1.34.0 (minimum v1.25 required for webhooks, server-side apply)
- **controller-runtime**: v0.22.1
- **Ginkgo/Gomega**: For testing
- **Optional**: cert-manager for TLS automation

## Important File Locations

- **CRD manifests**: `config/crd/bases/mint.cashu.asmogo.github.io_cashumints.yaml`
- **Kustomize overlays**: `config/default/kustomization.yaml`
- **Sample CRs**: `config/samples/`
- **Main entry point**: `cmd/main.go`
- **Documentation**: `docs/` (api-reference.md, deployment-guide.md, troubleshooting.md, migration-guide.md)

## Workflow for Changes

1. Modify API types and run `make manifests generate`
2. Update controller logic and generators
3. Run `make fmt vet lint` to ensure code quality
4. Run `make test` for unit tests
5. Test locally with `make run` or `make deploy IMG=...`
6. Run `make test-e2e` before committing significant changes
7. Update sample manifests if API changed
8. Update documentation if user-facing behavior changed
