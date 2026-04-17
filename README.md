# Cashu Mint Operator

[![Release](https://img.shields.io/github/v/release/asmogo/cashu-operator)](https://github.com/asmogo/cashu-operator/releases)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![CI](https://github.com/asmogo/cashu-operator/actions/workflows/test.yml/badge.svg)](https://github.com/asmogo/cashu-operator/actions/workflows/test.yml)
[![Docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://asmogo.github.io/cashu-operator/)
[![codecov](https://codecov.io/gh/asmogo/cashu-operator/branch/main/graph/badge.svg)](https://codecov.io/gh/asmogo/cashu-operator)

A Kubernetes operator for running [CDK `mintd`](https://github.com/cashubtc/cdk) with a single `CashuMint` custom resource. The operator manages the mint Deployment, generated config, Services, optional PostgreSQL, backups, ingress/TLS, Orchard, metrics, and sidecars so users can operate a Cashu mint with standard Kubernetes workflows instead of hand-managed YAML.

## Documentation

- **Docs site:** <https://asmogo.github.io/cashu-operator/>
- **Overview:** [`docs/index.md`](docs/index.md)
- **Getting started:** [`docs/getting-started.md`](docs/getting-started.md)
- **Deployment guide:** [`docs/deployment-guide.md`](docs/deployment-guide.md)
- **Payment processors:** [`docs/payment-processors.md`](docs/payment-processors.md)
- **Sample catalog:** [`docs/samples.md`](docs/samples.md)
- **API reference:** [`docs/api-reference.md`](docs/api-reference.md)
- **Troubleshooting:** [`docs/troubleshooting.md`](docs/troubleshooting.md)

## What the operator manages

- `Deployment`, `ConfigMap`, and `Service` for every mint
- PVCs for SQLite, redb, and Orchard state
- Auto-provisioned PostgreSQL `Secret`, `Service`, and `StatefulSet`
- S3 backup `CronJob` and restore `Job` for auto-provisioned PostgreSQL
- `Ingress` and optional cert-manager `Certificate`
- Optional Orchard companion resources
- Optional management RPC TLS Secret generation
- Optional `PodMonitor` when Prometheus metrics are enabled
- Optional gRPC payment processor sidecars and LDK node sidecars

## Quick start

Install the latest release:

```bash
kubectl apply -f https://github.com/asmogo/cashu-operator/releases/latest/download/install.yaml
```

Deploy the minimal sample:

```bash
kubectl create namespace cashu-mints
kubectl apply -n cashu-mints \
  -f https://raw.githubusercontent.com/asmogo/cashu-operator/main/config/samples/mint_v1alpha1_cashumint_minimal.yaml
kubectl get cashumints -n cashu-mints -w
```

Access the mint:

```bash
kubectl -n cashu-mints port-forward svc/cashumint-minimal 8085:8085
curl http://localhost:8085/v1/info
```

## Sample manifests

| Scenario | Manifest |
| --- | --- |
| Minimal quick start (`fakeWallet` + SQLite) | [`mint_v1alpha1_cashumint_minimal.yaml`](config/samples/mint_v1alpha1_cashumint_minimal.yaml) |
| Annotated starter template | [`mint_v1alpha1_cashumint.yaml`](config/samples/mint_v1alpha1_cashumint.yaml) |
| Auto-provisioned PostgreSQL | [`mint_v1alpha1_cashumint_postgres_auto.yaml`](config/samples/mint_v1alpha1_cashumint_postgres_auto.yaml) |
| External PostgreSQL | [`mint_v1alpha1_cashumint_external_postgres.yaml`](config/samples/mint_v1alpha1_cashumint_external_postgres.yaml) |
| LND backend | [`mint_v1alpha1_cashumint_lnd.yaml`](config/samples/mint_v1alpha1_cashumint_lnd.yaml) |
| LNBits backend | [`mint_v1alpha1_cashumint_lnbits.yaml`](config/samples/mint_v1alpha1_cashumint_lnbits.yaml) |
| CLN backend | [`mint_v1alpha1_cashumint_cln.yaml`](config/samples/mint_v1alpha1_cashumint_cln.yaml) |
| External gRPC payment processor | [`mint_v1alpha1_cashumint_grpc_processor_external.yaml`](config/samples/mint_v1alpha1_cashumint_grpc_processor_external.yaml) |
| Spark/Breez or Stripe sidecar processors | [`mint_v1alpha1_cashumint_spark_breez.yaml`](config/samples/mint_v1alpha1_cashumint_spark_breez.yaml), [`mint_v1alpha1_cashumint_stripe_processor.yaml`](config/samples/mint_v1alpha1_cashumint_stripe_processor.yaml) |
| Orchard | [`mint_v1alpha1_cashumint_orchard_sqlite.yaml`](config/samples/mint_v1alpha1_cashumint_orchard_sqlite.yaml), [`mint_v1alpha1_cashumint_orchard_postgres.yaml`](config/samples/mint_v1alpha1_cashumint_orchard_postgres.yaml) |
| Auth, Redis cache, limits, metrics | [`mint_v1alpha1_cashumint_auth_httpcache.yaml`](config/samples/mint_v1alpha1_cashumint_auth_httpcache.yaml) |
| LDK node sidecar | [`mint_v1alpha1_cashumint_ldk_node.yaml`](config/samples/mint_v1alpha1_cashumint_ldk_node.yaml) |
| Production-style reference | [`mint_v1alpha1_cashumint_production.yaml`](config/samples/mint_v1alpha1_cashumint_production.yaml) |

## Development

```bash
make build            # Build controller binary
make run              # Run controller locally against the active kubeconfig
make test             # Run unit tests
make test-e2e         # Run end-to-end tests
make lint             # Run golangci-lint
make build-installer  # Generate dist/install.yaml
```

## License

Apache 2.0 -- see [LICENSE](LICENSE).
