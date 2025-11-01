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
| **gRPC Processor** | External gRPC payment processor (custom)      | Custom enterprise flows  | Host/port, TLS secret (optional). Mounts cert/key pair if provided. |

### 1.3 Resource Requirements

- **mintd container**: baseline request ~100m CPU, 128Mi memory. Increase for high traffic.
- **PostgreSQL auto-provisioned**: default requests 100m CPU, 256Mi memory. Size according to throughput and retention.
- **LDK node** (optional): additional container ports (P2P + admin). Allocate memory for gossip synchronization (~512Mi) and CPU for channel operations.
- **Ingress controller**: ensure cluster has an ingress implementation (nginx, traefik, etc.).

### 1.4 Storage Considerations

- **PostgreSQL auto-provisioned**: defaults to `10Gi`. Use production storage class with SSD-backed volumes. Adjust `spec.database.postgres.autoProvisionSpec.storageSize`.
- **SQLite/redb**: set `spec.storage.size` to desired capacity (include growth over time). Ensure ReadWriteOnce volume suits single replica deployment.
- **Backups**: For auto-provisioned PostgreSQL, setup external backup solution (e.g., Velero, snapshots). Operator does not handle backups.

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
  lightning:
    backend: grpcprocessor
    grpcProcessor:
      address: processor-service.cashu:443
      port: 443
      supportedUnits: ["sat"]
      tlsSecretRef:
        name: grpc-client-cert
        key: client-bundle # Directory mount with client.crt/client.key/ca.crt
```

- TLS Secret must contain files named `client.crt`, `client.key`, `ca.crt`. Operator mounts the secret at `/secrets/grpc`.
- Use `supportedUnits` to advertise allowed denominations.

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

Implement NetworkPolicies to restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cashumint-allow-operator
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: cashu-mint
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/component: ingress-controller
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: lightning-ns
```

- Limit egress to database host, Lightning service, Bitcoin RPC nodes (LDK), and telemetry endpoints.
- Limit ingress to ingress controllers or API gateways.

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
- When updating configuration, edit the `CashuMint` CR. Operator triggers rolling deployment if config hash changes.
- For zero-downtime upgrades, plan redeployments during maintenance windows in case the new configuration requires initialization.

---

## 8. References

- CRD schema: [`api/v1alpha1/cashumint_types.go`](../api/v1alpha1/cashumint_types.go)
- Controller implementation: [`internal/controller/cashumint_controller.go`](../internal/controller/cashumint_controller.go)
- Resource generators directory: [`internal/controller/generators`](../internal/controller/generators/deployment.go)
- Sample manifests: [`config/samples`](../config/samples/kustomization.yaml)
- Troubleshooting: [`docs/troubleshooting.md`](troubleshooting.md)
- Migration steps: [`docs/migration-guide.md`](migration-guide.md)