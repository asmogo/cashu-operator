# Cashu Mint Operator Migration Guide

_Last updated: 2025-11-01_

This guide helps teams migrate existing manually deployed Cashu mints (container manifests, Helm charts, plain Kubernetes YAML) to the Cashu Mint Kubernetes Operator. The operator provides a declarative and automated lifecycle, reducing manual maintenance and configuration drift.

---

## 1. Pre-Migration Checklist

Before starting, ensure the following tasks are complete:

- [ ] **Backup data**  
  - Export database dumps (PostgreSQL, SQLite files, redb storage).
  - Archive Lightning-related secrets (macaroons, TLS certs, API keys).
- [ ] **Document current configuration**  
  - Record environment variables, command-line flags, configuration files, Secrets, Service definitions, Ingresses, and PersistentVolumeClaims.
  - Note image versions and customizations.
- [ ] **Identify resources to migrate**  
  - Deployments/StatefulSets, Services, Ingresses, ConfigMaps, Secrets, PVCs, CronJobs (for backups), network policies.
  - Determine whether database is self-managed or external.

---

## 2. Migration Steps

Follow this order to ensure zero data loss and minimal downtime.

1. **Export existing configuration**
   - Retrieve current container spec: `kubectl get deployment mint -o yaml`.
   - Save ConfigMaps, Secrets, Services, Ingresses.
   - Document volume mounts and storage classes.

2. **Create equivalent `CashuMint` custom resource**
   - Map existing configuration to CR spec (see [Section 3](#3-configuration-mapping)).
   - Start with a sample manifest from [`config/samples`](../config/samples/kustomization.yaml), then customize.

3. **Backup and preserve data**
   - Ensure database dumps are stored safely.
   - For SQLite/redb, copy `.db` or `.redb` files to external storage.
   - If migrating to operator-managed PostgreSQL, prepare data import steps.

4. **Deploy operator**
   - Install CRDs: `make install`.
  - Deploy controller: `make deploy IMG=ghcr.io/<org>/cashu-operator:<tag>`.

5. **Apply the `CashuMint` resource**
   - `kubectl apply -f cashumint-migrated.yaml`.
   - Wait for status phase `Ready`. Check conditions via `kubectl describe cashumint <name>`.

6. **Verify migration**
   - Validate minted tokens, API responses (e.g., `/v1/info`).
   - Ensure Lightning backend connectivity (check `LightningReady` condition).
   - Confirm Ingress routing, TLS certificates, Service endpoints, metrics.

7. **Clean up old resources**
   - Delete legacy Deployments/StatefulSets, Services, ConfigMaps, Ingresses no longer needed.
   - Remove redundant Secrets (after verifying new references).
   - Decommission old monitoring stack only after confirming new setup.

---

## 3. Configuration Mapping

Use the table below to convert manual configuration into `CashuMint` spec fields.

| Manual Component                                  | Location (Legacy)            | `CashuMint` Field                                        |
|--------------------------------------------------|------------------------------|----------------------------------------------------------|
| `mintd` container image                           | Deployment.spec.template     | `spec.image`                                             |
| Container args/env vars                           | Deployment                    | `spec.mintInfo`, `spec.logging`, `spec.resources`, etc. |
| Mnemonic secret (`CASHU_MNEMONIC`)                | Secret                        | `spec.mintInfo.mnemonicSecretRef`                       |
| Config TOML file                                  | ConfigMap / volume            | Operator auto-generates from spec; remove manual ConfigMap |
| Database URL env (`CDK_MINTD_DATABASE_URL`)       | Secret/env var                | `spec.database.postgres.urlSecretRef` or `spec.database.postgres.url` |
| Auto-provisioned Postgres                         | StatefulSet + PVC             | `spec.database.postgres.autoProvision=true` + `autoProvisionSpec` |
| SQLite/redb data path                             | Volume mount                  | `spec.database.engine=sqlite` + `spec.storage`          |
| Lightning backend configs                         | Env vars / config entries     | `spec.lightning` (choose backend-specific struct)       |
| LND macaroon/cert secrets                         | Secret volume mounts          | `spec.lightning.lnd.macaroonSecretRef` / `certSecretRef` |
| LNBits API keys                                   | Secret env vars               | `spec.lightning.lnbits.adminApiKeySecretRef`, `invoiceApiKeySecretRef` |
| gRPC processor TLS certs                          | Secret volume                 | `spec.lightning.grpcProcessor.tlsSecretRef`             |
| LDK sidecar container                             | Secondary container           | `spec.ldkNode` (enables sidecar automatically)          |
| Service definition                                | Service YAML                  | `spec.service` (type, annotations, loadBalancerIP)      |
| Ingress rules                                     | Ingress YAML                  | `spec.ingress` (host, annotations, TLS, cert-manager)   |
| Resource requests/limits                          | Pod spec                      | `spec.resources` and optional `spec.database.postgres.autoProvisionSpec.resources` |
| Node selectors / tolerations / affinity           | Pod spec                      | `spec.nodeSelector`, `spec.tolerations`, `spec.affinity` |
| Management RPC port                               | Manual env/arg                | `spec.managementRPC`                                    |
| HTTP cache configuration                          | Manual env/arg                | `spec.httpCache`                                        |
| Logging level/format                              | Env vars (e.g., `RUST_LOG`)   | `spec.logging`                                          |
| Auth configuration (OIDC)                         | Env vars / config             | `spec.auth` (enabled, discovery URL, client ID, toggles) |
| Secrets (macaroons, API keys, DB credentials)     | Kubernetes Secret             | Refer via `SecretKeySelector` fields in spec            |
| PVC for SQLite                                    | PersistentVolumeClaim         | Operator auto-creates from `spec.storage`               |
| Health probes                                     | Deployment spec               | Operator sets defaults; tune by editing generator if required |

---

## 4. Data Migration

### 4.1 SQLite → Operator-managed SQLite

1. Scale down old deployment (`kubectl scale deployment mint --replicas=0`).
2. Copy SQLite file from PVC to safe location.
3. Mount file into the new pod:
   - Create temporary pod with new PVC, copy data (`kubectl cp`).
   - Alternatively, build initContainers for data import.
4. Apply new `CashuMint` referencing the same storage class/size.

### 4.2 PostgreSQL (External)

- If the database remains the same, point `CashuMint` to existing DSN via Secret.
- Ensure the operator-managed deployment runs with identical schema. In most cases, no additional migrations are needed.
- For new database, import dump before switching DNS/Ingress.

### 4.3 PostgreSQL (Auto-Provisioned)

- Export data from old Postgres.
- After new StatefulSet becomes ready, connect via port-forward to import:
  ```bash
  kubectl port-forward statefulset/cashumint-postgres 5432:5432
  psql postgres://cdk:<password>@localhost:5432/cdk_mintd -f dump.sql
  ```
- Verify data integrity before routing traffic.

### 4.4 redb

- Locate redb file and copy it to a secure location.
- Populate new PVC with file before starting new mint pod.

### 4.5 Secrets

- Re-create secrets in destination namespace.
- Use `kubectl get secret old -o yaml | ...` to transform if needed.
- Confirm key names match `SecretKeySelector.key` fields in CR.

### 4.6 Testing

- After seeding data, run smoke tests (API endpoints, Lightning interactions).
- Monitor operator logs for errors (`kubectl logs -n cashu-operator-system deployment/cashu-operator-controller-manager`).

---

## 5. Rollback Procedure

If issues occur, follow these steps:

1. **Scale down operator-managed mint**
   ```bash
   kubectl scale deployment cashumint-<name> --replicas=0
   ```
2. **Restore original deployment**
   ```bash
   kubectl apply -f legacy-mint.yaml
   kubectl apply -f legacy-service.yaml
   kubectl apply -f legacy-ingress.yaml
   ```
3. **Restore data**
   - Import backups (PostgreSQL dump, SQLite file, etc.).
   - Re-link Secrets to old pods.

4. **Update DNS / ingress**
   - Point traffic back to original Service endpoints.

5. **Investigate operator issues**
   - Inspect `CashuMint` status conditions and controller logs.
   - Resolve validation errors or missing secrets before retrying migration.

6. **Cleanup failed resources**
   - Delete partially applied `CashuMint` (`kubectl delete cashumint <name>`).
   - Remove operator-managed PVCs/Secrets if no longer needed.

---

## 6. Additional Tips

- Use namespaces to separate old and new deployments during migration.
- Validate webhook admissions by running `kubectl apply --dry-run=server`.
- Leverage `kubectl diff -f cashumint.yaml` to preview changes.
- Document the final `CashuMint` spec in version control for future updates.
- Consider blue/green migration by deploying new mint alongside old one on distinct hostnames for testing.

---

## 7. References

- CRD schema: [`cashumint_types.go`](../api/v1alpha1/cashumint_types.go)
- Webhook validation: [`cashumint_webhook.go`](../api/v1alpha1/cashumint_webhook.go)
- Operator architecture: [`OPERATOR_ARCHITECTURE.md`](../OPERATOR_ARCHITECTURE.md)
- Deployment guide: [`deployment-guide.md`](deployment-guide.md)
- Troubleshooting: [`troubleshooting.md`](troubleshooting.md)
- Sample manifests: [`config/samples`](../config/samples/kustomization.yaml)