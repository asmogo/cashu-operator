# Deployment guide

This guide explains how to turn `CashuMint` into a production-ready Kubernetes deployment and how the operator maps the spec to Kubernetes resources.

## Reconciliation model

For every `CashuMint`, the operator continuously reconciles the generated resources instead of treating the manifest as a one-shot install. In practice that means:

- spec changes regenerate the mint config and trigger a rolling Deployment update through a config hash annotation
- optional resources such as PostgreSQL, backups, Ingress, Certificates, Orchard resources, and PodMonitors are created and updated from the same CR
- status conditions tell you which dependency is blocking readiness

## Choose your database

| Database mode | When to use it | Operator-managed resources | Sample |
| --- | --- | --- | --- |
| SQLite | Local development and simple single-node mints | PVC | [`minimal`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_minimal.yaml) |
| redb | Lightweight embedded storage with a PVC | PVC | [`template`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint.yaml) |
| PostgreSQL auto-provisioned | Default production path if you want the operator to manage the database lifecycle | Secret, Service, StatefulSet | [`postgres_auto`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_postgres_auto.yaml) |
| PostgreSQL external | Existing managed database or shared PostgreSQL cluster | none beyond mint Deployment/Service | [`external_postgres`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_external_postgres.yaml) |

### Notes

- Auto-provisioned PostgreSQL always uses in-cluster plaintext transport to the generated StatefulSet, so the operator forces `tlsMode=disable` for that internal URL.
- External PostgreSQL should normally use `tlsMode: require`.
- For embedded engines (`sqlite`, `redb`), the operator creates the PVC when `spec.storage` is present.

## Choose your payment backend

The operator uses `spec.paymentBackend`, not a separate top-level Lightning section.

| Backend | Main fields | Best for | Sample |
| --- | --- | --- | --- |
| `fakeWallet` | `spec.paymentBackend.fakeWallet` | Local testing and demos | [`minimal`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_minimal.yaml) |
| `lnd` | `spec.paymentBackend.lnd.address`, `macaroonSecretRef`, `certSecretRef` | LND-backed production mints | [`lnd`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_lnd.yaml) |
| `cln` | `spec.paymentBackend.cln.rpcPath` | Core Lightning environments | [`cln`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_cln.yaml) |
| `lnbits` | `spec.paymentBackend.lnbits.api`, key Secret refs | Hosted or self-managed LNBits wallets | [`lnbits`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_lnbits.yaml) |
| `grpcProcessor` | `spec.paymentBackend.grpcProcessor` | Custom payment flows, Spark/Breez, Stripe, external processors | [payment processor guide](payment-processors.md) |

### Backend caveats

- **LND** secrets are mounted into the mint pod and translated to CDK paths automatically.
- **LNBits** API keys are injected as environment variables.
- **CLN** only passes the configured `rpcPath` through to CDK. Make sure that socket path is reachable inside the mint container through your image or surrounding platform setup.
- **gRPC processor** can either talk to an external service or run a sidecar inside the mint pod.

## Expose the mint

### Service

`spec.service` controls the main mint Service:

- `type`: `ClusterIP`, `NodePort`, or `LoadBalancer`
- `annotations`
- `loadBalancerIP`

The operator always reconciles a Service for the mint. Orchard gets its own Service when enabled.

### Ingress and TLS

Use `spec.ingress` for the mint and `spec.orchard.ingress` for Orchard.

| Feature | Fields |
| --- | --- |
| Ingress class | `spec.ingress.className` |
| Hostname | `spec.ingress.host` |
| TLS secret | `spec.ingress.tls.secretName` |
| cert-manager issuer | `spec.ingress.tls.certManager.issuerName` and `issuerKind` |
| Extra annotations | `spec.ingress.annotations` |

When cert-manager integration is enabled, the operator also reconciles a `Certificate`.

## Enable Kubernetes-facing operator features

| Feature | Fields | What the operator adds |
| --- | --- | --- |
| Auto-generated mnemonic | `spec.mintInfo.autoGenerateMnemonic` | Creates `<mint>-mnemonic` Secret once and reuses it |
| Management RPC | `spec.managementRPC` | Configures mint management RPC; Orchard can force TLS generation for it |
| Orchard | `spec.orchard` | Orchard container, PVC, Service, optional Ingress and Certificate |
| Metrics | `spec.prometheus` | Exposes metrics port and reconciles a `PodMonitor` |
| Backups | `spec.backup` | `CronJob` for `pg_dump` uploads and restore `Job` when annotated |
| LDK sidecar | `spec.ldkNode` | Adds `ldk-node` container and LDK-related env vars |
| HTTP cache | `spec.httpCache` | Configures CDK cache settings and optional Redis URL injection |
| Auth | `spec.auth` | Renders NUT-21/NUT-22 auth config and optional auth DB URL |
| DoS limits | `spec.limits` | Writes `[limits]` into the CDK config |
| Pod placement | `nodeSelector`, `tolerations`, `affinity` | Pass-through to the pod spec |
| Security context | `podSecurityContext`, `containerSecurityContext` | Pass-through or sensible defaults |

## Orchard-specific notes

Orchard is not just a flag on the mint container. When `spec.orchard.enabled=true`, the operator also:

- enables management RPC if it was not already set
- chooses a default Orchard image that matches the mint database engine
- creates a dedicated Orchard PVC for Orchard application state
- creates a dedicated Orchard Service and optional Ingress/Certificate
- auto-generates a management RPC TLS Secret when Orchard needs mTLS and no Secret exists yet

Use the Orchard samples when you want a complete working pattern instead of building the nested config from scratch.

## Observability and rollout behavior

- `spec.prometheus.enabled=true` exposes the metrics port and reconciles a `PodMonitor`
- if the Prometheus Operator CRD is not installed, reconciliation fails with a clear error instead of silently skipping metrics
- config changes roll the Deployment because the operator hashes the generated ConfigMap into the pod template annotations

## Status conditions

The most useful conditions during rollout are:

| Condition | Meaning |
| --- | --- |
| `Ready` | Overall mint readiness |
| `DatabaseReady` | Auto-provisioned PostgreSQL or database dependency status |
| `PaymentBackendReady` | Payment backend dependency status |
| `ConfigValid` | Generated config rendered and applied successfully |
| `IngressReady` | Ingress and public endpoint status |
| `BackupReady` | Backup CronJob or restore Job status |

## Production checklist

1. Use PostgreSQL, not SQLite or redb.
2. Set a public `mintInfo.url` that matches your Ingress hostname.
3. Store all sensitive inputs in Secrets and reference them from the CR.
4. Enable backups when using auto-provisioned PostgreSQL.
5. Decide whether you need management RPC, Orchard, Prometheus metrics, and authentication before exposing the mint.
6. Size CPU, memory, and storage explicitly instead of relying on defaults.
