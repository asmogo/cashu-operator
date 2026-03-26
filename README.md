# Cashu Mint Operator

[![Release](https://img.shields.io/github/v/release/asmogo/cashu-operator)](https://github.com/asmogo/cashu-operator/releases)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![CI](https://github.com/asmogo/cashu-operator/actions/workflows/test.yml/badge.svg)](https://github.com/asmogo/cashu-operator/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/asmogo/cashu-operator/branch/main/graph/badge.svg)](https://codecov.io/gh/asmogo/cashu-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/asmogo/cashu-operator)](https://goreportcard.com/report/github.com/asmogo/cashu-operator)

A Kubernetes operator for running [CDK mintd](https://github.com/cashubtc/cdk) (Cashu mints) with full lifecycle automation. Manage database provisioning, Lightning backend configuration, ingress, TLS, and rolling updates from a single declarative `CashuMint` custom resource.

## ✨ Features

- **Declarative Management**: Manage `CashuMint` custom resources with continuous reconciliation.
- **Multiple Database Backends**: Support for auto-provisioned PostgreSQL, external PostgreSQL, SQLite, and redb.
- **Lightning Backends**: Seamless integration with LND, CLN, LNBits, FakeWallet, and gRPC Processor.
- **LDK Node Sidecar**: Optional sidecar with configurable Bitcoin network connectivity.
- **Ingress & TLS**: Automatic management with cert-manager integration for secure endpoints.
- **Zero-Downtime Updates**: Config-driven rolling updates triggered automatically on spec changes.
- **Rich Status Conditions**: Granular operational states including `Ready`, `DatabaseReady`, `LightningReady`, `ConfigValid`, and `IngressReady`.

## 📋 Prerequisites

| Component    | Requirement |
|--------------|-------------|
| **Kubernetes**| v1.25+      |
| **kubectl**  | v1.25+      |
| **cert-manager**| Optional (Required for automatic TLS certificate issuance) |

## 🚀 Quick Start

### 1. Install the Operator

Install the operator and its Custom Resource Definitions (CRDs) into your cluster:

```bash
kubectl apply -f https://github.com/asmogo/cashu-operator/releases/latest/download/install.yaml
```

### 2. Deploy Your First Mint

Create a namespace and apply a minimal test configuration (FakeWallet + SQLite):

```bash
kubectl create namespace cashu-mints
kubectl apply -n cashu-mints -f https://raw.githubusercontent.com/asmogo/cashu-operator/main/config/samples/mint_v1alpha1_cashumint_minimal.yaml
```

### 3. Verify Deployment

Check the status of your newly created mint:

```bash
kubectl get cashumints -n cashu-mints
```

### 4. Access the Mint

Forward the service port to your local machine and query the mint info:

```bash
kubectl -n cashu-mints port-forward svc/cashumint-minimal 8085:8085 &
curl http://localhost:8085/v1/info
```

## ⚙️ Configuration

The `CashuMint` custom resource exposes the full CDK mintd configuration surface. See the [API Reference](docs/api-reference.md) for the complete specification.

### Sample Manifests

| Scenario | Manifest Link |
|----------|---------------|
| Minimal testing (FakeWallet + SQLite) | [`cashumint_minimal.yaml`](config/samples/mint_v1alpha1_cashumint_minimal.yaml) |
| Auto-provisioned PostgreSQL | [`cashumint_postgres_auto.yaml`](config/samples/mint_v1alpha1_cashumint_postgres_auto.yaml) |
| External PostgreSQL | [`cashumint_external_postgres.yaml`](config/samples/mint_v1alpha1_cashumint_external_postgres.yaml) |
| LND Lightning backend | [`cashumint_lnd.yaml`](config/samples/mint_v1alpha1_cashumint_lnd.yaml) |
| Production with TLS + gRPC | [`cashumint_production.yaml`](config/samples/mint_v1alpha1_cashumint_production.yaml) |

## 🗑️ Uninstall

To remove the operator and its CRDs from your cluster:

```bash
kubectl delete -f https://github.com/asmogo/cashu-operator/releases/latest/download/install.yaml
```

## 🛠️ Development

Common `make` targets for local development:

```bash
make build           # Build controller binary
make run             # Run controller locally against active kubeconfig
make test            # Run unit tests
make test-e2e        # Run end-to-end tests (provisions a Kind cluster automatically)
make lint            # Run linter
make manifests       # Regenerate CRDs, RBAC, webhook configs
make generate        # Regenerate DeepCopy methods
make build-installer # Generate consolidated dist/install.yaml
```

## 📚 Documentation

- [API Reference](docs/api-reference.md)
- [Deployment Guide](docs/deployment-guide.md)
- [Migration Guide](docs/migration-guide.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Operator Architecture](OPERATOR_ARCHITECTURE.md)

## 🤝 Contributing

1. Fork and clone the repository.
2. Create a new feature branch.
3. Make your changes, ensuring new CRD fields include validation markers and webhook logic.
4. Run `make lint test` to ensure code quality.
5. Open a Pull Request.

## 📄 License

Apache 2.0 -- see the [LICENSE](LICENSE) file for details.
