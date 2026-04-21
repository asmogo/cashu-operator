# API reference

This page summarizes the `CashuMint` API as it exists in `api/v1alpha1/cashumint_types.go`. Defaults listed here reflect the webhook defaulting behavior, not just the raw Go zero values.

Authoritative source: [`api/v1alpha1/cashumint_types.go`](https://github.com/asmogo/cashu-operator/blob/main/api/v1alpha1/cashumint_types.go)

## Resource shape

```yaml
apiVersion: mint.cashu.asmogo.github.io/v1alpha1
kind: CashuMint
metadata:
  name: cashumint-sample
spec:
  mintInfo:
    url: https://mint.example.com
    autoGenerateMnemonic: true
  database:
    engine: postgres
    postgres:
      autoProvision: true
  paymentBackend:
    fakeWallet: {}
```

## Top-level `spec`

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `image` | string | `cashubtc/mintd:0.15.0` | Mint container image |
| `imagePullPolicy` | string | `IfNotPresent` | Standard Kubernetes pull policy |
| `imagePullSecrets` | `[]LocalObjectReference` | none | For private registries |
| `replicas` | int32 | `1` | Valid range is `1..1` |
| `mintInfo` | `MintInfo` | required | Public URL, metadata, mnemonic handling |
| `database` | `DatabaseConfig` | required | `postgres`, `sqlite`, or `redb` |
| `paymentBackend` | `PaymentBackendConfig` | required | Exactly one backend must be set |
| `ldkNode` | `LDKNodeConfig` | disabled | Optional sidecar |
| `auth` | `AuthConfig` | disabled | NUT-21/NUT-22 auth config |
| `httpCache` | `HTTPCacheConfig` | disabled | Memory or Redis cache |
| `managementRPC` | `ManagementRPCConfig` | disabled | Optional management RPC endpoint |
| `orchard` | `OrchardConfig` | disabled | Optional Orchard companion |
| `prometheus` | `PrometheusConfig` | disabled | Metrics endpoint and PodMonitor |
| `limits` | `LimitsConfig` | none | Transaction input/output caps |
| `ingress` | `IngressConfig` | disabled | Public mint ingress |
| `service` | `ServiceConfig` | `ClusterIP` when set | Main Service config |
| `resources` | `ResourceRequirements` | operator defaults | Mint container resources |
| `nodeSelector` / `tolerations` / `affinity` | Kubernetes types | none | Pod placement |
| `logging` | `LoggingConfig` | `info` / `json` when set | CDK log settings |
| `storage` | `StorageConfig` | `10Gi` when set | PVC config for embedded DBs and shared data |
| `backup` | `BackupConfig` | disabled | Auto-provisioned PostgreSQL only |
| `podSecurityContext` | `PodSecurityContext` | non-root defaults | Pod-level security |
| `containerSecurityContext` | `SecurityContext` | restrictive defaults | Mint container security |

## `spec.mintInfo`

| Field | Default | Notes |
| --- | --- | --- |
| `url` | required | Public mint URL; must start with `http://` or `https://` |
| `listenHost` | `0.0.0.0` | Mint bind address inside the container |
| `listenPort` | `8085` | Mint API port |
| `mnemonicSecretRef` | none | Secret containing `CDK_MINTD_MNEMONIC` |
| `autoGenerateMnemonic` | `false` | Creates `<mint>-mnemonic` if no mnemonic Secret ref is provided |
| `name`, `description`, `descriptionLong`, `motd` | none | Public mint metadata |
| `pubkeyHex`, `iconUrl`, `contactEmail`, `contactNostrPubkey`, `tosUrl` | none | Additional mint info fields |
| `inputFeePpk` | none | Input fee in parts per thousand |
| `enableSwaggerUi` | `false` | Enables Swagger UI in CDK |
| `useKeysetV2` | unset | When unset, existing keysets are preserved and new ones use V2 |
| `quoteTtl.mintTtl` | `600` | Quote TTL in seconds |
| `quoteTtl.meltTtl` | `120` | Quote TTL in seconds |

## `spec.database`

### Common

| Field | Notes |
| --- | --- |
| `engine` | `postgres`, `sqlite`, or `redb` |
| `postgres` | Required when `engine=postgres` |
| `sqlite` | Used when `engine=sqlite` |

### PostgreSQL

| Field | Default | Notes |
| --- | --- | --- |
| `url` | none | Direct URL; mutually exclusive with `urlSecretRef` |
| `urlSecretRef` | none | Preferred for external PostgreSQL |
| `tlsMode` | `require` for external DBs | `disable`, `prefer`, or `require` |
| `maxConnections` | `20` | |
| `connectionTimeoutSeconds` | `10` | |
| `autoProvision` | `false` | Enables operator-managed PostgreSQL |
| `autoProvisionSpec.storageSize` | `10Gi` | StatefulSet PVC size |
| `autoProvisionSpec.storageClassName` | none | |
| `autoProvisionSpec.resources` | none | PostgreSQL container resources |
| `autoProvisionSpec.version` | `"15"` | PostgreSQL major version |

### SQLite

| Field | Default | Notes |
| --- | --- | --- |
| `sqlite.dataDir` | `/data` | SQLite file directory inside the pod |

## `spec.paymentBackend`

Exactly one backend must be set.

### Shared fields

| Field | Notes |
| --- | --- |
| `minMint`, `maxMint` | Mint amount limits |
| `minMelt`, `maxMelt` | Melt amount limits |

### `lnd`

| Field | Default | Notes |
| --- | --- | --- |
| `address` | required | Must include protocol and port |
| `macaroonSecretRef` | none | Mounted into the pod if set |
| `certSecretRef` | none | Mounted into the pod if set |
| `feePercent` | `0.04` | |
| `reserveFeeMin` | `4` | |

### `cln`

| Field | Default | Notes |
| --- | --- | --- |
| `rpcPath` | required | Socket path passed to CDK |
| `bolt12` | unset | Enables BOLT12 support |
| `feePercent` | `0.04` | |
| `reserveFeeMin` | `4` | |

### `lnbits`

| Field | Default | Notes |
| --- | --- | --- |
| `api` | required | LNBits API base URL |
| `adminApiKeySecretRef` | required | Injected as env var |
| `invoiceApiKeySecretRef` | required | Injected as env var |
| `retroApi` | `false` | Enables v0 compatibility mode |
| `feePercent` | `0.02` | |
| `reserveFeeMin` | `2` | |

### `fakeWallet`

| Field | Default | Notes |
| --- | --- | --- |
| `supportedUnits` | `["sat"]` | |
| `feePercent` | `0.02` | |
| `reserveFeeMin` | `1` | |
| `minDelayTime` | `1` | Seconds |
| `maxDelayTime` | `3` | Seconds |

### `grpcProcessor`

| Field | Default | Notes |
| --- | --- | --- |
| `address` | none for external, loopback implied for sidecar | Include `http://` or `https://` when you need a specific scheme |
| `port` | `50051` | |
| `supportedUnits` | `["sat"]` | |
| `tlsSecretRef` | none | Secret is mounted by name; use a key such as `client.crt` |
| `sidecarProcessor` | disabled | Runs a generic processor sidecar in the mint pod |

### `grpcProcessor.sidecarProcessor`

| Field | Default | Notes |
| --- | --- | --- |
| `enabled` | `false` | Must be true to inject the sidecar |
| `image` | none | Required when enabled |
| `imagePullPolicy` | `IfNotPresent` | |
| `command`, `args`, `env` | none | Passed straight to the sidecar container |
| `workingDir` | none | Mounted from the shared data volume under `sidecar-processor` |
| `resources` | none | Sidecar container resources |
| `enableTLS` | `false` | Sidecar serves TLS |
| `tlsSecretRef` | none | Required when `enableTLS=true` |

## `spec.ldkNode`

`ldkNode` adds an `ldk-node` sidecar and writes `[ldk_node]` into the mint config.

| Field | Default |
| --- | --- |
| `enabled` | `false` |
| `image` | `ghcr.io/cashubtc/ldk-node:latest` |
| `feePercent` | `0.04` |
| `reserveFeeMin` | `4` |
| `bitcoinNetwork` | `signet` |
| `chainSourceType` | `esplora` |
| `esploraUrl` | none |
| `bitcoinRpc` | none |
| `storageDirPath`, `logDirPath` | none |
| `mnemonicSecretRef` | none |
| `host` | `0.0.0.0` |
| `port` | `8090` |
| `announceAddresses` | none |
| `gossipSourceType` | `rgs` |
| `rgsUrl` | none |
| `webserverHost` | `127.0.0.1` |
| `webserverPort` | `8888` |

`bitcoinRpc` contains `host`, `port`, `userSecretRef`, and `passwordSecretRef`.

## `spec.auth`

When `auth.enabled=true`, the webhook defaults the per-endpoint auth levels to `clear` unless you override them.

| Field | Default | Notes |
| --- | --- | --- |
| `enabled` | `false` | |
| `openidDiscovery` | none | OIDC discovery URL |
| `openidClientId` | none | |
| `mintMaxBat` | `50` | |
| `mint`, `getMintQuote`, `checkMintQuote`, `melt`, `getMeltQuote`, `checkMeltQuote`, `swap`, `restore`, `checkProofState` | `clear` when auth is enabled | `clear`, `blind`, or `none` |
| `database.postgres` | none | Optional auth DB config using the same PostgreSQL struct style |

## `spec.httpCache`

| Field | Default | Notes |
| --- | --- | --- |
| `backend` | `memory` | `memory` or `redis` |
| `ttl` | `60` | Seconds |
| `tti` | `60` | Seconds |
| `redis.keyPrefix` | none | Required for Redis |
| `redis.connectionString` | none | |
| `redis.connectionStringSecretRef` | none | Injected as `REDIS_CONNECTION_STRING` when set |

## `spec.managementRPC`

| Field | Default | Notes |
| --- | --- | --- |
| `enabled` | `false` | |
| `address` | `127.0.0.1` | |
| `port` | `8086` | |
| `tlsSecretRef.name` | `<mint>-management-rpc-tls` when needed | If the Secret does not exist and TLS is required, the operator generates it |

Generated or user-provided management RPC TLS Secrets should include:

- `ca.pem`
- `server.pem`
- `server.key`
- `client.pem`
- `client.key`

## `spec.orchard`

### Core settings

| Field | Default | Notes |
| --- | --- | --- |
| `enabled` | `false` | |
| `image` | derived from DB engine | Uses Orchard 1.8.1 image from `ghcr.io/cashubtc` |
| `imagePullPolicy` | `IfNotPresent` | |
| `host` | `0.0.0.0` | |
| `port` | `3321` | |
| `basePath` | `api` | |
| `logLevel` | `warn` | |
| `setupKeySecretRef` | required | Orchard setup key |
| `throttleTTL` | `60000` | Milliseconds |
| `throttleLimit` | `20` | |
| `proxy` | none | |
| `compression` | unset | |
| `service` | `ClusterIP` when set/defaulted | Orchard Service config |
| `ingress` | disabled | Orchard Ingress config |
| `storage.size` | `10Gi` | Orchard PVC |
| `resources` | none | Orchard container resources |
| `containerSecurityContext` | Orchard-friendly default | |
| `extraEnv` | none | Extra Orchard env vars |

### `orchard.mint`

| Field | Default | Notes |
| --- | --- | --- |
| `type` | `cdk` | `cdk` or `nutshell` |
| `api` | none | Overrides mint API endpoint |
| `database` | none | Overrides mint DB URL or sqlite path |
| `databaseCaSecretRef`, `databaseCertSecretRef`, `databaseKeySecretRef` | none | Optional PostgreSQL TLS materials |
| `rpc.host` | `127.0.0.1` when RPC block is set | |
| `rpc.port` | `8086` when RPC block is set | |
| `rpc.mTLS` | inferred | Defaults from management RPC TLS state |

### Optional Orchard integrations

| Block | Notes |
| --- | --- |
| `orchard.bitcoin` | Bitcoin Core connectivity (`rpcHost`, `rpcPort`, user/password Secret refs) |
| `orchard.lightning` | LND or CLN connectivity for Orchard |
| `orchard.taprootAssets` | Taproot Assets connectivity |
| `orchard.ai` | Optional AI API endpoint |

## Networking, logging, storage, and backups

### `spec.prometheus`

| Field | Default |
| --- | --- |
| `enabled` | `false` |
| `address` | `0.0.0.0` |
| `port` | `9090` |

### `spec.limits`

| Field | Notes |
| --- | --- |
| `maxInputs` | Maximum transaction inputs |
| `maxOutputs` | Maximum transaction outputs |

### `spec.ingress`

| Field | Default | Notes |
| --- | --- | --- |
| `enabled` | `false` | |
| `className` | `nginx` | |
| `host` | required when enabled | |
| `annotations` | none | |
| `tls.enabled` | `true` when TLS block exists | |
| `tls.secretName` | none | |
| `tls.certManager.enabled` | `false` | |
| `tls.certManager.issuerName` | required when cert-manager is enabled | |
| `tls.certManager.issuerKind` | `ClusterIssuer` | `Issuer` or `ClusterIssuer` |

### `spec.service`

| Field | Default |
| --- | --- |
| `type` | `ClusterIP` |
| `annotations` | none |
| `loadBalancerIP` | none |

### `spec.logging`

| Field | Default |
| --- | --- |
| `level` | `info` |
| `fileLevel` | none |
| `format` | `json` |

### `spec.storage`

| Field | Default |
| --- | --- |
| `size` | `10Gi` |
| `storageClassName` | none |

### `spec.backup`

Backups currently require:

- `spec.database.engine=postgres`
- `spec.database.postgres.autoProvision=true`

Fields:

| Field | Default | Notes |
| --- | --- | --- |
| `enabled` | `false` | |
| `schedule` | `0 */6 * * *` | Cron expression |
| `retentionCount` | `14` | |
| `s3.bucket` | required | |
| `s3.prefix` | mint name when defaulted | |
| `s3.region`, `s3.endpoint` | none | |
| `s3.accessKeyIdSecretRef` | required | |
| `s3.secretAccessKeySecretRef` | required | |

## Status

### `status.phase`

- `Pending`
- `Provisioning`
- `Updating`
- `Ready`
- `Failed`

### `status.conditions`

| Condition type | Meaning |
| --- | --- |
| `Ready` | Overall readiness |
| `DatabaseReady` | Database dependency state |
| `PaymentBackendReady` | Payment backend dependency state |
| `ConfigValid` | Config generation/application status |
| `IngressReady` | Ingress/public endpoint state |
| `BackupReady` | Backup or restore resource status |

### Other status fields

| Field | Notes |
| --- | --- |
| `observedGeneration` | Last spec generation seen by the controller |
| `backendType` | Active payment backend name |
| `deploymentName`, `serviceName`, `ingressName`, `configMapName` | Managed resource names |
| `databaseStatus`, `paymentBackendStatus` | Connection state summaries |
| `url` | Effective public URL |
| `readyReplicas` | Ready Deployment replicas |
