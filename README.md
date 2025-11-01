# Cashu Mint Kubernetes Operator

The Cashu Mint Operator provides full lifecycle automation for running [CDK mintd](https://github.com/cashubtc/cdk) inside Kubernetes. It translates declarative [`CashuMint`](config/crd/bases/mint.cashu.asmogo.github.io_cashumints.yaml) custom resources into a complete mint deployment with database, Lightning backend, ingress, TLS, and optional LDK sidecar. The operator is designed for production environments that require repeatable deployments, configuration drift detection, and continuous health reconciliation.

## Key Highlights

- **Production-ready automation** &mdash; continuous reconciliation, rolling updates, and status conditions.
- **Flexible backends** &mdash; PostgreSQL (auto-provisioned or external), SQLite, redb, and Lightning providers including LND, CLN, LNBits, FakeWallet, and gRPC Processor.
- **Integrated ingress and TLS** &mdash; native cert-manager support and customizable networking.
- **Operational visibility** &mdash; detailed status conditions (`Ready`, `DatabaseReady`, `LightningReady`, `ConfigValid`, `IngressReady`) surfaced on every `CashuMint`.

## Architecture Overview

- Custom Resource Definition and validation logic: [`cashumint_types.go`](api/v1alpha1/cashumint_types.go), [`cashumint_webhook.go`](api/v1alpha1/cashumint_webhook.go)
- Controller implementation: [`cashumint_controller.go`](internal/controller/cashumint_controller.go)
- Resource generators: [`internal/controller/generators`](internal/controller/generators/deployment.go)
- Detailed design: [`OPERATOR_ARCHITECTURE.md`](OPERATOR_ARCHITECTURE.md)

See [Deployment Guide](docs/deployment-guide.md) for backend selection and sizing guidance.

## Features

- Automated deployment, upgrades, and reconciliation of Cashu mints.
- Multiple database backends:
  - PostgreSQL with operator-managed auto-provisioning.
  - External PostgreSQL with Secret-based credentials.
  - Embedded SQLite or redb stores for testing or single-node setups.
- Lightning backend integrations: LND, CLN, LNBits, FakeWallet (testing), gRPC Processor.
- Optional LDK node sidecar with configurable Bitcoin network connectivity.
- Ingress and TLS management with cert-manager automation.
- Config-driven rolling updates triggered by spec changes or config hash delta.
- Rich status reporting with per-subsystem readiness conditions and observed state.

## Prerequisites

| Component       | Requirement                                                      |
|-----------------|------------------------------------------------------------------|
| Kubernetes      | v1.25 or newer (webhooks, server-side apply, networking APIs)    |
| kubectl         | v1.25 or newer                                                   |
| cert-manager    | Optional, required for automatic TLS certificate issuance        |
| Helm            | Optional, for packaging or alternative deployment workflows      |
| Go toolchain    | Go 1.22+ (for building/running locally)                          |

## Installation

```bash
# 1. Install or update CRDs
make install

# 2. Deploy the operator manager (uses Kustomize overlays)
make deploy IMG=ghcr.io/<org>/cashu-operator:<tag>

# Optional: manually deploy using pre-built manifests
# kubectl apply -k config/default

# 3. Verify controller pods are healthy
kubectl -n cashu-operator-system get pods
```

> **Note**  
> The default `make deploy` workflow patches the manager Deployments with the image tag provided through `IMG`. Ensure your Kubernetes context has sufficient privileges (cluster-admin recommended during installation).

## Quick Start

1. **Create a namespace and secrets (if needed).**

   ```bash
   kubectl create namespace cashu-mints
   ```

2. **Apply the minimal sample using FakeWallet + SQLite.**

   ```bash
   kubectl -n cashu-mints apply -f config/samples/mint_v1alpha1_cashumint_minimal.yaml
   ```

3. **Inspect status and conditions.**

   ```bash
   kubectl -n cashu-mints get cashumints
   kubectl -n cashu-mints describe cashumint cashumint-minimal
   ```

4. **Access the mint endpoint.**

   - Port-forward the service:

     ```bash
     kubectl -n cashu-mints port-forward svc/cashumint-minimal 8085:8085
     curl http://localhost:8085/v1/info
     ```

   - For ingress-enabled deployments, refer to the production sample at [`mint_v1alpha1_cashumint_production.yaml`](config/samples/mint_v1alpha1_cashumint_production.yaml).

## Usage Examples

Sample CR manifests are available under [`config/samples`](config/samples/kustomization.yaml):

| Scenario                                  | Manifest                                                                                   |
|-------------------------------------------|--------------------------------------------------------------------------------------------|
| Minimal testing (FakeWallet + SQLite)     | [`mint_v1alpha1_cashumint_minimal.yaml`](config/samples/mint_v1alpha1_cashumint_minimal.yaml) |
| Auto-provisioned PostgreSQL               | [`mint_v1alpha1_cashumint_postgres_auto.yaml`](config/samples/mint_v1alpha1_cashumint_postgres_auto.yaml) |
| External PostgreSQL                       | [`mint_v1alpha1_cashumint_external_postgres.yaml`](config/samples/mint_v1alpha1_cashumint_external_postgres.yaml) |
| LND Lightning backend                     | [`mint_v1alpha1_cashumint_lnd.yaml`](config/samples/mint_v1alpha1_cashumint_lnd.yaml) |
| Full production example with TLS + gRPC   | [`mint_v1alpha1_cashumint_production.yaml`](config/samples/mint_v1alpha1_cashumint_production.yaml) |

## Configuration Reference

The full schema (including defaults and validation) is documented in:

- [API Reference](docs/api-reference.md)
- Generated CRD manifest: [`mint.cashu.asmogo.github.io_cashumints.yaml`](config/crd/bases/mint.cashu.asmogo.github.io_cashumints.yaml)

For backend planning, TLS setup, and sizing guidelines, see the [Deployment Guide](docs/deployment-guide.md). Migration procedures from manual deployments are covered in the [Migration Guide](docs/migration-guide.md). Common issues and debug commands are summarized in the [Troubleshooting Guide](docs/troubleshooting.md).

## Development

### Build and Test Locally

```bash
# Compile controller binary
make build

# Run controller against the active kubeconfig
make run

# Execute unit tests and controller-runtime envtest suite
make test

# Optional: run lint checks
make lint
```

### End-to-End Testing

```
make test-e2e
```

This target provisions a kind cluster (if missing), installs CRDs, deploys the operator, and executes integration tests from [`test/e2e`](test/e2e/e2e_test.go).

### Contributing

1. Fork and clone the repository.
2. Install Go 1.22+, Docker (optional), and controller-runtime tooling (`make controller-gen` acquires binaries).
3. Use feature branches with descriptive names (`feature/add-cln-guide`).
4. Run `make lint`, `make test`, and update documentation prior to opening a pull request.
5. Ensure new CR fields include validation markers and necessary webhook logic.

Refer to the architecture design at [`OPERATOR_ARCHITECTURE.md`](OPERATOR_ARCHITECTURE.md) before implementing significant changes.

## Frequently Used Make Targets

| Target          | Description                                      |
|-----------------|--------------------------------------------------|
| `make install`  | Apply CRDs to the current cluster                |
| `make deploy`   | Deploy controller-manager via Kustomize          |
| `make undeploy` | Remove controller-manager resources              |
| `make build-installer` | Generate `dist/install.yaml` bundle       |
| `make run`      | Run controller locally with your kubeconfig      |
| `make test`     | Run unit tests                                   |
| `make test-e2e` | Execute kind-based end-to-end suite              |

## License & Credits

- Licensed under the Apache 2.0 License. See [`LICENSE`](LICENSE) for details.
- Maintained by the asmogo/cashu-operator community.
- Leverages [Kubebuilder](https://book.kubebuilder.io) scaffolding and controller-runtime libraries.
