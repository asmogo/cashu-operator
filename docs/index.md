# Cashu Mint Operator

Cashu Mint Operator turns a single `CashuMint` custom resource into a working CDK `mintd` deployment on Kubernetes. Instead of hand-writing Deployments, Services, ConfigMaps, Ingresses, PostgreSQL manifests, backup jobs, and sidecars, you declare the mint once and let the operator reconcile the rest.

## What the operator manages

| Feature in `CashuMint` | Kubernetes resources the operator reconciles |
| --- | --- |
| Core mint deployment | `Deployment`, `ConfigMap`, `Service` |
| SQLite or redb storage | `PersistentVolumeClaim` |
| Auto-provisioned PostgreSQL | `Secret`, `Service`, `StatefulSet` |
| S3 backups for auto-provisioned PostgreSQL | `CronJob` and restore `Job` on demand |
| Ingress and cert-manager TLS | `Ingress`, optional `Certificate` |
| Orchard | Orchard container, `Service`, `Ingress`, `Certificate`, PVC |
| Management RPC TLS | Auto-generated or user-provided TLS `Secret` |
| Prometheus scraping | `PodMonitor` when metrics are enabled |
| gRPC payment processor sidecar | Additional container and shared volumes in the mint pod |

## How to read these docs

| Goal | Read this |
| --- | --- |
| Get a mint running quickly | [Getting started](getting-started.md) |
| Pick the right database, backend, ingress, and production settings | [Deployment guide](deployment-guide.md) |
| Run CDK with an external or sidecar payment processor | [Payment processors](payment-processors.md) |
| Find a manifest to copy | [Sample catalog](samples.md) |
| Check field names and defaults | [API reference](api-reference.md) |
| Move from hand-written manifests to the operator | [Migration guide](migration-guide.md) |
| Debug a rollout or dependency problem | [Troubleshooting](troubleshooting.md) |

## Quick path

Install the operator:

```bash
kubectl apply -f https://github.com/asmogo/cashu-operator/releases/latest/download/install.yaml
```

Create a namespace and deploy the minimal sample:

```bash
kubectl create namespace cashu-mints
kubectl apply -n cashu-mints \
  -f https://raw.githubusercontent.com/asmogo/cashu-operator/main/config/samples/mint_v1alpha1_cashumint_minimal.yaml
```

Wait for the mint to become ready:

```bash
kubectl get cashumints -n cashu-mints -w
```

Port-forward the Service and inspect the mint:

```bash
kubectl -n cashu-mints port-forward svc/cashumint-minimal 8085:8085
curl http://localhost:8085/v1/info
```

The minimal sample is intentionally self-contained: it uses SQLite, `fakeWallet`, and `mintInfo.autoGenerateMnemonic=true` so you can verify the operator without creating prerequisite Secrets first.

## Core ideas

1. **You describe intent in `CashuMint`.** Database engine, payment backend, ingress, backup policy, Orchard, and operational settings all live in one manifest.
2. **The operator owns the generated resources.** It keeps them in sync, rolls Deployments on config changes, and updates status conditions as dependencies become ready.
3. **Secrets stay in Secrets.** Sensitive inputs such as mnemonics, LND macaroons, LNBits keys, Redis URLs, PostgreSQL URLs, and payment processor credentials are always referenced by name/key instead of being embedded in the CR.

## Where to go next

- Start from the [sample catalog](samples.md) if you already know which backend you want.
- Read the [payment processor guide](payment-processors.md) if your mint uses `grpcProcessor`.
- Use the [deployment guide](deployment-guide.md) before exposing a mint through Ingress or switching to PostgreSQL.
