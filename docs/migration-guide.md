# Migration guide

Use this guide when you already run a Cashu mint on Kubernetes and want to move that deployment under operator control without losing track of the underlying CDK settings.

## What changes with the operator

Instead of maintaining separate YAML for the mint Deployment, ConfigMap, Service, Ingress, PostgreSQL, backup jobs, and sidecars, you move those inputs into one `CashuMint` resource.

The operator then owns:

- mint `Deployment`, `ConfigMap`, and `Service`
- optional PostgreSQL `StatefulSet`, `Service`, and credentials `Secret`
- optional PVCs for SQLite, redb, and Orchard
- optional `Ingress`, `Certificate`, `PodMonitor`, backup `CronJob`, and restore `Job`

## Migration workflow

1. Back up the database and all secrets.
2. Export the current mint configuration and list every environment variable or config-file value you still need.
3. Start from the closest sample in [`config/samples`](https://github.com/asmogo/cashu-operator/tree/main/config/samples).
4. Encode the current deployment into `spec.mintInfo`, `spec.database`, `spec.paymentBackend`, and the optional operator feature blocks.
5. Apply the `CashuMint` in a test namespace first, wait for `Ready=True`, and validate API behavior.
6. Switch traffic to the operator-managed Service or Ingress only after the new mint is healthy.

## Field mapping

| Legacy input | `CashuMint` field |
| --- | --- |
| Mint image | `spec.image` |
| Public mint URL | `spec.mintInfo.url` |
| Mnemonic Secret | `spec.mintInfo.mnemonicSecretRef` or `spec.mintInfo.autoGenerateMnemonic` |
| SQLite data path | `spec.database.engine=sqlite` and `spec.database.sqlite.dataDir` |
| PostgreSQL URL env var | `spec.database.postgres.url` or `urlSecretRef` |
| Operator-managed PostgreSQL | `spec.database.postgres.autoProvision=true` |
| LND config | `spec.paymentBackend.lnd` |
| CLN config | `spec.paymentBackend.cln` |
| LNBits config | `spec.paymentBackend.lnbits` |
| Custom payment processor | `spec.paymentBackend.grpcProcessor` |
| Mint management RPC | `spec.managementRPC` |
| Orchard sidecar/app | `spec.orchard` |
| Metrics endpoint | `spec.prometheus` |
| OIDC auth | `spec.auth` |
| Redis HTTP cache | `spec.httpCache` |
| Ingress and TLS | `spec.ingress` |
| Service type or annotations | `spec.service` |
| CPU and memory | `spec.resources` |
| Node selectors, tolerations, affinity | `spec.nodeSelector`, `spec.tolerations`, `spec.affinity` |
| Security context | `spec.podSecurityContext`, `spec.containerSecurityContext` |
| Backup schedule | `spec.backup` |

## Database migration notes

### SQLite or redb

- Copy the database file to a safe location first.
- Make sure the new `CashuMint` uses the same database engine and a PVC with enough space.
- If you need to seed the PVC before starting the operator-managed pod, do that before directing traffic to the new mint.

### External PostgreSQL

- Reuse the same PostgreSQL URL through `spec.database.postgres.urlSecretRef` if you are not moving the database itself.
- Keep `tlsMode` aligned with the target database requirements.

### Auto-provisioned PostgreSQL

- Import the old dump into the operator-managed PostgreSQL instance after the StatefulSet is ready.
- If you need scheduled backups after migration, add `spec.backup`.

## Validation during rollout

Useful checks:

```bash
kubectl describe cashumint <name> -n <namespace>
kubectl get cashumint <name> -n <namespace> -o jsonpath='{range .status.conditions[*]}{.type}{"="}{.status}{" reason="}{.reason}{" message="}{.message}{"\n"}{end}'
kubectl logs deployment/<name> -c mintd -n <namespace>
```

Look especially at:

- `DatabaseReady`
- `PaymentBackendReady`
- `ConfigValid`
- `IngressReady`
- `BackupReady` when backups are enabled

## Rollback plan

Keep the old deployment definitions and verified database backups until the operator-managed mint has passed your smoke tests. If you need to roll back:

1. Scale down or delete the `CashuMint`.
2. Restore the previous Deployment, Service, and Ingress.
3. Restore the database or PVC contents if the migration changed them.
4. Re-point DNS or ingress traffic.

## Recommended approach

Migrate in layers:

1. Move the mint under the operator with the same database and payment backend first.
2. Only after that is stable, consider switching database mode, enabling backups, adding Orchard, or moving to a gRPC payment processor.
