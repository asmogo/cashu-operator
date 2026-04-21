# Getting started

This guide walks through the fastest path to a working mint and explains what the operator creates along the way.

## Prerequisites

| Component | Required | Notes |
| --- | --- | --- |
| Kubernetes | Yes | v1.25+ |
| `kubectl` | Yes | Any recent version that matches your cluster |
| Ingress controller | No | Only needed when you enable `spec.ingress` |
| cert-manager | No | Only needed for `spec.ingress.tls.certManager` |
| Prometheus Operator | No | Only needed when `spec.prometheus.enabled=true` |

## 1. Install the operator

Install the latest release bundle:

```bash
kubectl apply -f https://github.com/asmogo/cashu-operator/releases/latest/download/install.yaml
```

If you are developing locally instead of using a release:

```bash
make install
make deploy IMG=ghcr.io/asmogo/cashu-operator:dev
```

## 2. Deploy your first mint

Create a namespace and apply the minimal sample:

```bash
kubectl create namespace cashu-mints
kubectl apply -n cashu-mints \
  -f https://raw.githubusercontent.com/asmogo/cashu-operator/main/config/samples/mint_v1alpha1_cashumint_minimal.yaml
```

What this sample does:

- uses SQLite with a PVC
- configures `fakeWallet` for local testing
- auto-generates the mint mnemonic Secret
- exposes the mint through the default `ClusterIP` Service

## 3. Watch the rollout

Check the custom resource:

```bash
kubectl get cashumints -n cashu-mints
kubectl describe cashumint cashumint-minimal -n cashu-mints
```

Useful status signals:

- `status.phase` moves through `Pending`, `Provisioning`, and `Ready`
- `Ready=True` means the Deployment is up
- `DatabaseReady=True` means PostgreSQL is ready when you use auto-provisioning
- `PaymentBackendReady=True` means the configured payment backend dependencies are available
- `ConfigValid=True` means the operator successfully rendered and applied the mint config

## 4. Access the mint

Port-forward the generated Service:

```bash
kubectl -n cashu-mints port-forward svc/cashumint-minimal 8085:8085
curl http://localhost:8085/v1/info
```

The quick-start sample keeps `mintInfo.url` aligned with local port-forwarding. Before exposing a real mint through Ingress, change that field to the public hostname you want clients to use.

## 5. Understand what was created

For a minimal SQLite mint, the operator creates:

| Resource | Name |
| --- | --- |
| `CashuMint` | `cashumint-minimal` |
| `ConfigMap` | `cashumint-minimal-config` |
| `Deployment` | `cashumint-minimal` |
| `Service` | `cashumint-minimal` |
| `PersistentVolumeClaim` | `cashumint-minimal-data` |
| Mnemonic `Secret` | `cashumint-minimal-mnemonic` |

Inspect them with:

```bash
kubectl get all,pvc,configmap,secret -n cashu-mints -l app.kubernetes.io/instance=cashumint-minimal
```

## 6. Pick the next sample

Once the quick start works, move to a sample that matches your real deployment:

| Need | Sample |
| --- | --- |
| Auto-managed PostgreSQL | [`mint_v1alpha1_cashumint_postgres_auto.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_postgres_auto.yaml) |
| Existing PostgreSQL database | [`mint_v1alpha1_cashumint_external_postgres.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_external_postgres.yaml) |
| LND backend | [`mint_v1alpha1_cashumint_lnd.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_lnd.yaml) |
| LNBits backend | [`mint_v1alpha1_cashumint_lnbits.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_lnbits.yaml) |
| External gRPC payment processor | [`mint_v1alpha1_cashumint_grpc_processor_external.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_grpc_processor_external.yaml) |
| Orchard | [`mint_v1alpha1_cashumint_orchard_postgres.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_orchard_postgres.yaml) |

## 7. Clean up

Delete the sample mint:

```bash
kubectl delete cashumint cashumint-minimal -n cashu-mints
```

Remove the operator:

```bash
kubectl delete -f https://github.com/asmogo/cashu-operator/releases/latest/download/install.yaml
```
