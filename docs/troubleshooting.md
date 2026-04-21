# Troubleshooting

This guide focuses on the conditions and failure modes you are most likely to hit while reconciling a `CashuMint`.

## Start with status

```bash
kubectl describe cashumint <name> -n <namespace>
kubectl get cashumint <name> -n <namespace> \
  -o jsonpath='{range .status.conditions[*]}{.type}{"="}{.status}{" reason="}{.reason}{" message="}{.message}{"\n"}{end}'
```

The operator's main condition types are:

- `Ready`
- `DatabaseReady`
- `PaymentBackendReady`
- `ConfigValid`
- `IngressReady`
- `BackupReady`

## Common issues

### `Ready=False` and `ConfigValid=False`

Usually means the operator could not render or apply the generated config.

Check:

```bash
kubectl logs -n cashu-operator-system deployment/cashu-operator-controller-manager
kubectl get configmap <mint-name>-config -n <namespace> -o yaml
```

Typical causes:

- invalid field combination in the CR
- missing PostgreSQL Secret for an external DB
- invalid backup configuration on a non-auto-provisioned PostgreSQL mint

### `DatabaseReady=False`

If you use auto-provisioned PostgreSQL:

```bash
kubectl get statefulset,pvc,secret -n <namespace> | grep <mint-name>-postgres
kubectl describe statefulset <mint-name>-postgres -n <namespace>
kubectl logs statefulset/<mint-name>-postgres -n <namespace>
```

Typical causes:

- storage class does not exist
- PVC is stuck in `Pending`
- PostgreSQL container resources are too small

If you use external PostgreSQL, re-check `spec.database.postgres.urlSecretRef`, `tlsMode`, and the target URL.

### `PaymentBackendReady=False`

This condition usually points to missing dependency material rather than a broken Deployment.

Backend-specific checks:

| Backend | What to verify |
| --- | --- |
| `lnd` | macaroon Secret, cert Secret, reachable `address` |
| `lnbits` | admin and invoice API key Secrets, API URL |
| `cln` | socket path exists inside the container |
| `grpcProcessor` | address, port, TLS Secret, or sidecar image |

### gRPC processor errors

Common mistakes:

1. `address` is missing when not using a sidecar.
2. Sidecar is enabled but `sidecarProcessor.image` is missing.
3. The processor should use TLS but `address` still starts with `http://`.
4. `tlsSecretRef` points to the wrong Secret name.

Inspect the rendered config and pod:

```bash
kubectl get configmap <mint-name>-config -n <namespace> -o jsonpath='{.data.config\.toml}'
kubectl describe pod -n <namespace> -l app.kubernetes.io/instance=<mint-name>
kubectl logs deployment/<mint-name> -c mintd -n <namespace>
kubectl logs deployment/<mint-name> -c grpc-processor -n <namespace>
```

### Ingress not becoming ready

Check:

```bash
kubectl get ingress <mint-name> -n <namespace>
kubectl describe ingress <mint-name> -n <namespace>
```

If cert-manager is enabled:

```bash
kubectl describe certificate <mint-name> -n <namespace>
kubectl logs -n cert-manager deployment/cert-manager
```

Typical causes:

- `spec.ingress.host` missing
- issuer name or kind is wrong
- DNS is not pointed at the ingress controller

### Metrics enabled but reconciliation fails

When `spec.prometheus.enabled=true`, the operator reconciles a `PodMonitor`. If the Prometheus Operator CRD is not installed, reconciliation fails instead of silently skipping it.

Check:

```bash
kubectl api-resources | grep podmonitors
```

### Backups not reconciling

Backups only work when:

- `spec.database.engine=postgres`
- `spec.database.postgres.autoProvision=true`
- `spec.backup.s3` is fully specified

Check:

```bash
kubectl get cronjob,job -n <namespace> | grep <mint-name>
kubectl describe cronjob <mint-name>-backup -n <namespace>
```

To request a restore:

```bash
kubectl annotate cashumint <mint-name> -n <namespace> \
  mint.cashu.asmogo.github.io/restore-object-key=<object-key> \
  mint.cashu.asmogo.github.io/restore-request-id=<request-id> \
  --overwrite
```

### Orchard problems

Orchard adds more moving parts than a plain mint:

- Orchard setup key Secret must exist
- Orchard may depend on management RPC and management RPC TLS
- Orchard has its own Service, Ingress, and PVC

Check:

```bash
kubectl get all,pvc,ingress,secret -n <namespace> | grep <mint-name>-orchard
kubectl logs deployment/<mint-name> -c orchard -n <namespace>
```

## Quick resource inventory

These commands are useful for almost every debugging session:

```bash
# Custom resource and generated resources
kubectl get cashumint <name> -n <namespace> -o yaml
kubectl get all,pvc,configmap,secret,ingress,certificate,cronjob,job,podmonitor \
  -n <namespace> -l app.kubernetes.io/instance=<name>

# Mint logs
kubectl logs deployment/<name> -c mintd -n <namespace>

# Operator logs
kubectl logs -n cashu-operator-system deployment/cashu-operator-controller-manager
```
