# Cashu Mint Operator Deployment Guide

_Last updated: 2025-11-01_

This guide helps you plan, size, and deploy Cashu mints using the Cashu Mint Kubernetes Operator. It expands on the quick start instructions and provides best practices for production environments.

---

## 1. Planning Your Deployment

Before creating a `CashuMint` custom resource (CR), evaluate the environment requirements below.

### 1.1 Choose the Database Backend

| Backend     | Use Case                                  | Durability | Notes |
|-------------|-------------------------------------------|------------|-------|
| **PostgreSQL (Auto-Provisioned)** | Production default with automated lifecycle | High | Operator creates StatefulSet, PVC, Service, Secret, and connection URL. Supports configuration of storage class, size, and resource limits. |
| **External PostgreSQL** | Existing managed PostgreSQL or shared cluster | High | Requires secrets containing connection URL or credentials. Ensures TLS via `tlsMode`. |
| **SQLite** | Development, POCs, single-node setups     | Medium     | Stores data on local PVC (`spec.storage`). No built-in HA. |
| **redb**   | Lightweight embedded store                | Medium     | Similar to SQLite but uses `.redb` file. Requires persistent storage for production-like testing. |

### 1.2 Choose the Lightning Backend

Evaluate the Lightning payment processor your mint needs:

| Backend          | Description                                      | Ideal For                | Requirements |
|------------------|--------------------------------------------------|--------------------------|--------------|
| **LND**          | LND gRPC integration                             | Production environments  | TLS cert and macaroon in Kubernetes Secret. `address` must include protocol and port. |
| **CLN**          | Core Lightning RPC socket                        | Self-hosted CLN nodes    | Shared volume or network file system for `rpcPath`. |
| **LNBits**       | LNBits API integration                           | LNBits-managed wallets   | Admin and invoice keys via Secrets, optional retro API compatibility. |
| **FakeWallet**   | In-memory mock wallet                            | Development/testing       | Not for production. Supports artificial latency range. |
| **gRPC Processor** | External or operator-managed gRPC payment processor | Custom enterprise flows  | Use either `grpcProcessor.address`/`port` or `grpcProcessor.processorRef` to `spec.paymentProcessors[]`; TLS secret optional. |

### 1.3 Resource Requirements

- **mintd container**: baseline request ~100m CPU, 128Mi memory. Increase for high traffic.
- **PostgreSQL auto-provisioned**: default requests 100m CPU, 256Mi memory. Size according to throughput and retention.
- **LDK node** (optional): additional container ports (P2P + admin). Allocate memory for gossip synchronization (~512Mi) and CPU for channel operations.
- **Ingress controller**: ensure cluster has an ingress implementation (nginx, traefik, etc.).

### 1.4 Storage Considerations

- **PostgreSQL auto-provisioned**: defaults to `10Gi`. Use production storage class with SSD-backed volumes. Adjust `spec.database.postgres.autoProvisionSpec.storageSize`.
- **SQLite/redb**: set `spec.storage.size` to desired capacity (include growth over time). Ensure ReadWriteOnce volume suits single replica deployment.
- **Orchard**: when enabled, Orchard gets its own PVC for application state even if the mint database is PostgreSQL. Size `spec.orchard.storage` separately from the mint database/storage.
- **Backups**: For auto-provisioned PostgreSQL, you can configure scheduled S3-compatible `pg_dump` backups via `spec.backup`.
- **Backup scope**: Current operator-managed backups target auto-provisioned PostgreSQL and upload dumps to object storage.
- **CDK v0.15 default + migration safety**: The default mint image is `cashubtc/mintd:0.15.0`. For PostgreSQL mints, take a verified database backup immediately before rollout/upgrades because CDK v0.15 performs database migrations.

---

## 2. Database Backend Configuration

### 2.1 SQLite & redb

```yaml
spec:
  database:
    engine: sqlite          # or redb
    sqlite:
      dataDir: /data        # default
  storage:
    size: 5Gi
    storageClassName: fast-ssd
```

**Limitations**

- Single replica only (enforced by CRD).
- PVC is mandatory for durability.
- No schema migrations performed; ensure compatibility between mintd versions.

### 2.2 PostgreSQL Auto-Provisioning

```yaml
spec:
  database:
    engine: postgres
    postgres:
      autoProvision: true
      autoProvisionSpec:
        storageSize: 20Gi
        storageClassName: fast-ssd
        version: "16"
        resources:
          requests:
            cpu: 250m
            memory: 512Mi
```

- Secret `<mint-name>-postgres-secret` contains generated password and URL. Mint pods automatically use `CDK_MINTD_DATABASE_URL` env var.
- StatefulSet runs a single replica with `ClusterIP` headless service (`clusterIP: None`).
- TLS to internal PostgreSQL is disabled (`sslmode=disable`); for strict TLS, bring your own Postgres instance.

**Storage Sizing**

- Estimate 1–2 GiB per million mints (depends on spending patterns).
- Use storage class with high IOPS for minimal latency.

### 2.2.1 Scheduled S3 Backups (Auto-Provisioned PostgreSQL)

```yaml
spec:
  backup:
    enabled: true
    schedule: "0 */6 * * *"
    retentionCount: 14
    s3:
      bucket: cashu-mint-backups
      prefix: cashumint-prod
      region: us-east-1
      endpoint: https://s3.amazonaws.com
      accessKeyIdSecretRef:
        name: cashumint-backup-credentials
        key: AWS_ACCESS_KEY_ID
      secretAccessKeySecretRef:
        name: cashumint-backup-credentials
        key: AWS_SECRET_ACCESS_KEY
```

- Backup CronJobs are currently supported for auto-provisioned PostgreSQL.
- Ensure the backup credentials secret exists in the mint namespace.
- A one-shot restore Job can be requested by setting annotations on the `CashuMint`:
  - `mint.cashu.asmogo.github.io/restore-object-key=<s3-object-key>` (required)
  - `mint.cashu.asmogo.github.io/restore-request-id=<request-id>` (optional, use a new value to create a new restore Job name)
- Check `status.conditions[type=BackupReady]` to confirm backup/restore resource reconciliation.

**Backup/Restore Runbook**

1. Confirm backups are configured and the backup CronJob exists (`<mint-name>-backup`).
2. Select the exact backup object key to restore from your S3-compatible bucket.
3. Trigger restore by annotating the `CashuMint`:
   ```bash
   kubectl annotate cashumint <mint-name> -n <namespace> \
     mint.cashu.asmogo.github.io/restore-object-key=<s3-object-key> \
     mint.cashu.asmogo.github.io/restore-request-id=<unique-request-id> \
     --overwrite
   ```
4. Watch backup Jobs (`app.kubernetes.io/component=backup`) and `BackupReady` condition updates.
5. To request the same object restore again, change `restore-request-id` to a new value.

### 2.3 External PostgreSQL

Provide either connection URL or secret reference:

```yaml
spec:
  database:
    engine: postgres
    postgres:
      tlsMode: require          # disable|prefer|required
      urlSecretRef:
        name: cashumint-db
        key: DATABASE_URL
```

**Secret Example**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cashumint-db
type: Opaque
stringData:
  DATABASE_URL: postgres://user:pass@host:5432/dbname?sslmode=require
```

- Supported TLS modes: `disable`, `prefer`, `require`. Defaults to `require`.
- Mutual exclusivity: only one of `url` or `urlSecretRef`.
- Set `maxConnections` and `connectionTimeoutSeconds` to match DB quotas.

### 2.4 Orchard Companion Deployment

Orchard is deployed as an optional companion container in the same pod as `cdk-mintd`.
That gives Orchard direct access to:

- the mint API over `127.0.0.1:<listenPort>`
- the mint management RPC over `127.0.0.1:<managementRPC.port>`
- the mint work directory for sqlite-backed mints

By default, the operator uses Orchard `1.8.1` from the new `cashubtc` image namespace:
`ghcr.io/cashubtc/orchard-mintdb-sqlite:1.8.1` or `ghcr.io/cashubtc/orchard-mintdb-postgres:1.8.1`.

The operator also creates Orchard-specific Kubernetes resources:

- a dedicated PVC for Orchard application state (`<mint-name>-orchard-data`)
- a dedicated Service (`<mint-name>-orchard`)
- an optional dedicated Ingress and optional cert-manager `Certificate`

Orchard supports sqlite and PostgreSQL **mint** databases:

- for sqlite mints, the operator wires Orchard to `/mnt/mint/cdk-mintd.sqlite`
- for postgres mints, the operator reuses the same PostgreSQL URL or generated secret the mint uses

Orchard itself always persists its own application state in sqlite on its own PVC.

#### 2.4.1 Minimal Orchard example (sqlite mint)

```yaml
spec:
  database:
    engine: sqlite
    sqlite:
      dataDir: /data
  managementRPC:
    enabled: true
  orchard:
    enabled: true
    setupKeySecretRef:
      name: orchard-setup
      key: setup-key
    storage:
      size: 5Gi
    ingress:
      enabled: true
      host: orchard.example.com
```

#### 2.4.2 Orchard with PostgreSQL mint and secure management RPC

```yaml
spec:
  database:
    engine: postgres
    postgres:
      autoProvision: true
  managementRPC:
    enabled: true
  orchard:
    enabled: true
    setupKeySecretRef:
      name: orchard-setup
      key: setup-key
    mint:
      rpc:
        mTLS: true
    ingress:
      enabled: true
      host: orchard.example.com
      tls:
        enabled: true
        certManager:
          enabled: true
          issuerName: letsencrypt-prod
```

When Orchard uses mTLS to the colocated management RPC, the operator ensures a TLS secret exists. By default it uses `<cashumint-name>-management-rpc-tls`; if you set `spec.managementRPC.tlsSecretRef.name`, that name is used instead.

- `ca.pem`
- `server.pem`
- `server.key`
- `client.pem`
- `client.key`

The mint uses the server-side files; Orchard uses the client-side files. If the secret is absent, the operator generates it automatically.

#### 2.4.3 Optional Orchard integrations

You can also configure Orchard’s optional integrations via `spec.orchard`:

- `bitcoin` for Bitcoin Core RPC
- `lightning` for LND or CLN RPC
- `taprootAssets` for `tapd`
- `ai` for Ollama or another compatible AI endpoint
- `extraEnv` for advanced or future Orchard environment variables

Sample manifests:

- `config/samples/mint_v1alpha1_cashumint_orchard_sqlite.yaml`
- `config/samples/mint_v1alpha1_cashumint_orchard_postgres.yaml`

---

## 3. Lightning Backend Configuration

### 3.1 FakeWallet

```yaml
spec:
  lightning:
    backend: fakewallet
    fakeWallet:
      supportedUnits: ["sat"]
      feePercent: 0.02
      minDelayTime: 1
      maxDelayTime: 3
```

- For testing only. Provides predictable latency and fees.

### 3.2 LND

```yaml
spec:
  lightning:
    backend: lnd
    lnd:
      address: https://lnd-service.lightning:10009
      macaroonSecretRef:
        name: lnd-macaroon
        key: admin.macaroon
      certSecretRef:
        name: lnd-cert
        key: tls.cert
```

**Secrets**

- `lnd-macaroon` Secret with base64-encoded macaroon (binary).
- `lnd-cert` Secret with TLS PEM.

Ensure the operator namespace has permissions to read these Secrets (same namespace as mint or refer via RBAC/Secret copy).

### 3.3 CLN

```yaml
spec:
  lightning:
    backend: cln
    cln:
      rpcPath: /mnt/lightning/lightning-rpc
```

Mount the CLN socket via PVC/PV or hostPath. Typically implemented using Kubernetes CSI driver.

### 3.4 LNBits

```yaml
spec:
  lightning:
    backend: lnbits
    lnbits:
      api: https://lnbits.example/api/v1
      adminApiKeySecretRef:
        name: lnbits-keys
        key: admin
      invoiceApiKeySecretRef:
        name: lnbits-keys
        key: invoice
      retroApi: true
```

Ensure Secrets contain the API keys. TLS validation uses standard HTTP client in mintd; provide trusted CA.

### 3.5 gRPC Processor

```yaml
spec:
  paymentProcessors:
    - name: spark-primary
      image: ghcr.io/asmogo/cdk-spark-payment-prcoessor:v0.15.0
      port: 50051
    - name: spark-secondary
      image: ghcr.io/asmogo/cdk-spark-payment-prcoessor:v0.15.0
      port: 50051
  lightning:
    backend: grpcprocessor
    grpcProcessor:
      processorRef: spark-primary
      supportedUnits: ["sat"]
      tlsSecretRef:
        name: grpc-client-cert
        key: client-bundle # Directory mount with client.crt/client.key/ca.crt
```

- `spec.paymentProcessors[]` lets the operator deploy multiple processor workloads per mint.
- `grpcProcessor.processorRef` selects which managed processor is active for the mint.
- For external processors, omit `processorRef` and set `address` + `port`.
- TLS Secret must contain files named `client.crt`, `client.key`, `ca.crt`. Operator mounts the secret at `/secrets/grpc`.
- Use `supportedUnits` to advertise allowed denominations.

Example for the Stripe processor image (`asmogo/cdk-stripe-payment-processor:sha-7512cbe`):

```yaml
spec:
  paymentProcessors:
    - name: stripe-primary
      image: asmogo/cdk-stripe-payment-processor:sha-7512cbe
      port: 50051
      env:
        - name: STRIPE_API_KEY
          valueFrom:
            secretKeyRef:
              name: stripe-credentials
              key: STRIPE_API_KEY
        - name: STRIPE_WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: stripe-credentials
              key: STRIPE_WEBHOOK_SECRET
  lightning:
    backend: grpcprocessor
    grpcProcessor:
      processorRef: stripe-primary
      supportedUnits: ["usd"]
```

---

## 4. Ingress and TLS

### 4.1 Enable Ingress

```yaml
spec:
  ingress:
    enabled: true
    className: nginx
    host: mint.example.com
    annotations:
      nginx.ingress.kubernetes.io/proxy-body-size: "8m"
```

### 4.2 TLS Options

1. **Manually Managed Secret**

   ```yaml
   spec:
     ingress:
       tls:
         enabled: true
         secretName: mint-example-tls
   ```

   Create TLS secret yourself (`kubectl create secret tls`).

2. **cert-manager Integration**

   ```yaml
   spec:
     ingress:
       tls:
         enabled: true
         certManager:
           enabled: true
           issuerName: letsencrypt-prod
           issuerKind: ClusterIssuer
   ```

   - cert-manager must be installed cluster-wide.
   - Operator adds annotations to request certificates. TLS secret defaults to `<mint-name>-tls` unless specified.

### 4.3 DNS and Network

- Ensure DNS A/AAAA records point to the Ingress controller.
- For LoadBalancer Services, request static IPs via annotations and update DNS records accordingly.

---

## 5. Resource Management

### 5.1 Compute Resources

Set resource requests/limits via `spec.resources`:

```yaml
spec:
  resources:
    requests:
      cpu: 250m
      memory: 256Mi
    limits:
      cpu: 1
      memory: 512Mi
```

Follow these guidelines:

- Monitor CPU/memory usage (`kubectl top pods` or Prometheus) and adjust requests.
- Keep requests ≤ limits to avoid CR admission rejection (webhook enforces requests ≤ limits).

### 5.2 Storage Sizing

- **PostgreSQL**: Use `storageSize` to match expected retention. Increase for large channels or high transaction volume.
- **SQLite/redb**: Monitor PVC usage. Expand by resizing PVC (CSI must support expansion).
- Set `storageClassName` to IOPS-optimized storage for production.

### 5.3 Scaling

- Mint replicas are capped at 1 (`replicas` defaults to 1, max 1). Horizontal scaling is not supported due to stateful architecture.
- For high availability, deploy separate mints or consider failover strategies (e.g., active/passive with DNS updates).

---

## 6. Security Best Practices

### 6.1 Secret Management

- Store sensitive values (mnemonics, DB URLs, API keys) in Kubernetes Secrets. Reference via `SecretKeySelector`.
- Use external secret management (Sealed Secrets, External Secrets Operator) if possible.
- Rotate secrets periodically. Update the referenced Secret to trigger mint rolling update.

### 6.2 Network Policies

Use the provided hardening assets:

- `config/network-policy/allow-metrics-traffic.yaml` (via `config/network-policy/kustomization.yaml`) restricts operator metrics ingress (port 8443) to namespaces labeled `metrics=enabled`.
- `config/network-policy/mint/allow-ingress-from-labeled-namespaces.yaml` restricts mint workload ingress to same-namespace pods and explicitly labeled ingress/API gateway namespaces.

Apply the mint workload policy per namespace:

```bash
kubectl apply -n <mint-namespace> -k config/network-policy/mint
kubectl label namespace <ingress-namespace> cashu.asmogo.github.io/allow-mint-ingress=true
```

- If you also apply `config/network-policy/allow-metrics-traffic.yaml`, label scraping namespaces:
  ```bash
  kubectl label namespace <monitoring-namespace> metrics=enabled
  ```
- This template is additive and does not restrict egress by default, avoiding accidental breakage for existing Lightning/database endpoints.
- For stricter production isolation, add egress rules for database host, Lightning service, Bitcoin RPC nodes (LDK), and telemetry endpoints.

### 6.3 RBAC

- Operator runs with ClusterRole `manager-role`. Review and tighten RBAC rules if required (e.g., namespace-scoped RBAC via Kustomize overlays).
- Avoid running mints in operator namespace; use dedicated namespace per mint for isolation.

### 6.4 Pod Security

- Mint pods run as non-root and drop Linux capabilities by default.
- If LND macaroons/certs are stored as files, ensure Secrets have minimal privileges and `defaultMode` set to 0400.

---

## 7. Operational Tips

- Monitor `kubectl describe cashumint <name>` for status conditions.
- Use `kubectl get all -l app.kubernetes.io/instance=<mint>` to inspect managed resources.
- Set `spec.prometheus.enabled: true` to expose mint metrics and have the operator create a same-namespace `PodMonitor` automatically. This requires the Prometheus Operator `PodMonitor` CRD to be installed in the cluster.
- When updating configuration, edit the `CashuMint` CR. Operator triggers rolling deployment if config hash changes.
- Rollout dependency gating blocks Deployment/Service/Ingress reconciliation until required Secret references exist and auto-provisioned PostgreSQL (if enabled) is ready.
- While blocked, `Ready=False` and `LightningReady=False` with reason `DependenciesNotReady`, and reconciliation retries every 10 seconds.
- For zero-downtime upgrades, plan redeployments during maintenance windows in case the new configuration requires initialization.

---

## 8. References

- CRD schema: [`api/v1alpha1/cashumint_types.go`](../api/v1alpha1/cashumint_types.go)
- Controller implementation: [`internal/controller/cashumint_controller.go`](../internal/controller/cashumint_controller.go)
- Resource generators directory: [`internal/controller/generators`](../internal/controller/generators/deployment.go)
- Sample manifests: [`config/samples`](../config/samples/kustomization.yaml)
- Troubleshooting: [`docs/troubleshooting.md`](troubleshooting.md)
- Migration steps: [`docs/migration-guide.md`](migration-guide.md)
