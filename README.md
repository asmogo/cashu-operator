# Cashu Mint Operator

[![Release](https://img.shields.io/github/v/release/asmogo/cashu-operator)](https://github.com/asmogo/cashu-operator/releases)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![CI](https://github.com/asmogo/cashu-operator/actions/workflows/test.yml/badge.svg)](https://github.com/asmogo/cashu-operator/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/asmogo/cashu-operator/branch/main/graph/badge.svg)](https://codecov.io/gh/asmogo/cashu-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/asmogo/cashu-operator)](https://goreportcard.com/report/github.com/asmogo/cashu-operator)

A Kubernetes operator for running [CDK mintd](https://github.com/cashubtc/cdk) (Cashu mints) with full lifecycle automation -- database provisioning, Lightning backend configuration, ingress, TLS, and rolling updates from a single `CashuMint` custom resource.

## Features

- Declarative `CashuMint` custom resource with continuous reconciliation
- Database backends: auto-provisioned PostgreSQL, external PostgreSQL, SQLite, redb
- Lightning backends: LND, CLN, LNBits, FakeWallet, gRPC Processor
- Optional LDK node sidecar with configurable Bitcoin network connectivity
- Ingress and TLS management with cert-manager integration
- Config-driven rolling updates on spec changes
- Rich status conditions: `Ready`, `DatabaseReady`, `LightningReady`, `ConfigValid`, `IngressReady`

## Quick Start

### Install the operator

```bash
kubectl apply -f https://github.com/asmogo/cashu-operator/releases/latest/download/install.yaml
```

### Deploy your first mint

```bash
kubectl create namespace cashu-mints
kubectl apply -n cashu-mints -f https://raw.githubusercontent.com/asmogo/cashu-operator/main/config/samples/mint_v1alpha1_cashumint_minimal.yaml
```

### Verify

```bash
kubectl get cashumints -n cashu-mints
```

### Access the mint

```bash
kubectl -n cashu-mints port-forward svc/cashumint-minimal 8085:8085
curl http://localhost:8085/v1/info
```

## Prerequisites

| Component | Requirement |
|-----------|-------------|
| Kubernetes | v1.25+ |
| kubectl | v1.25+ |
| cert-manager | Optional -- required for automatic TLS certificate issuance |

## Configuration

The `CashuMint` custom resource exposes the full CDK mintd configuration surface. See the [API Reference](docs/api-reference.md) for the complete spec.

### Sample manifests

| Scenario | Manifest |
|----------|----------|
| Minimal testing (FakeWallet + SQLite) | [`cashumint_minimal.yaml`](config/samples/mint_v1alpha1_cashumint_minimal.yaml) |
| Auto-provisioned PostgreSQL | [`cashumint_postgres_auto.yaml`](config/samples/mint_v1alpha1_cashumint_postgres_auto.yaml) |
| External PostgreSQL | [`cashumint_external_postgres.yaml`](config/samples/mint_v1alpha1_cashumint_external_postgres.yaml) |
| LND Lightning backend | [`cashumint_lnd.yaml`](config/samples/mint_v1alpha1_cashumint_lnd.yaml) |
| Production with TLS + gRPC | [`cashumint_production.yaml`](config/samples/mint_v1alpha1_cashumint_production.yaml) |

## Uninstall

```bash
kubectl delete -f https://github.com/asmogo/cashu-operator/releases/latest/download/install.yaml
```

## Development

### Local development with Tilt + k3d

The recommended local workflow uses the dedicated k3d cluster defined in [`ctlptl-config.yaml`](ctlptl-config.yaml).

```bash
make tilt-up
```

That command creates or updates a dedicated `cashu-operator-dev` k3d cluster with a local registry, switches `kubectl` to `k3d-cashu-operator-dev`, and starts Tilt against that context.

Once Tilt is running:

1. `codegen` regenerates manifests and deepcopy code when the API changes.
2. `unit-tests` is available as a manual Tilt resource for `make test`.
3. `demo-mint` is available as a manual Tilt resource to apply a minimal `CashuMint` from `config/dev/`.

To inspect the demo mint after triggering `demo-mint`:

```bash
kubectl -n cashu-mints port-forward svc/cashumint-minimal 8085:8085
curl http://localhost:8085/v1/info
```

To stop the local loop:

```bash
make tilt-down
make dev-cluster-delete
```

```bash
make build          # Build controller binary
make run            # Run controller locally against active kubeconfig
make dev-cluster-up # Create/update the dedicated local k3d dev cluster
make tilt-up        # Start Tilt against the local k3d dev cluster
make tilt-down      # Stop Tilt and remove Tilt-managed resources
make dev-reset      # Tear down Tilt-managed resources and delete the dev cluster
make test           # Run unit tests
make test-e2e       # Run end-to-end tests (provisions Kind cluster)
make lint           # Run linter
make manifests      # Regenerate CRDs, RBAC, webhook configs
make generate       # Regenerate DeepCopy methods
make build-installer # Generate dist/install.yaml
```

## Documentation

- [API Reference](docs/api-reference.md)
- [Deployment Guide](docs/deployment-guide.md)
- [Migration Guide](docs/migration-guide.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Operator Architecture](OPERATOR_ARCHITECTURE.md)

## Contributing

1. Fork and clone the repository
2. Create a feature branch
3. Run `make lint test` before opening a PR
4. Ensure new CRD fields include validation markers and webhook logic

## License

Apache 2.0 -- see [LICENSE](LICENSE) for details.
