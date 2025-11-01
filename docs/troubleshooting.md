# Cashu Mint Operator Troubleshooting Guide

_Last updated: 2025-11-01_

This guide lists common operational issues, diagnostic procedures, and remediation steps for the Cashu Mint Kubernetes Operator. Use it alongside `kubectl describe cashumint <name>` to review status conditions and failure messages.

---

## 1. Common Issues

### Issue: CashuMint remains in `Pending` phase

**Possible causes**

- CR validation errors prevented reconciliation.
- Required secrets (mnemonic, database URL, Lightning credentials) missing.
- Resource dependencies (Ingress class, storage class) unavailable.

**Diagnostics**

1. Inspect conditions:
   ```bash
   kubectl describe cashumint <name>
   ```
2. Check operator logs:
   ```bash
   kubectl logs -n cashu-operator-system deployment/cashu-operator-controller-manager
   ```
3. Verify required secrets exist in the same namespace.

**Resolutions**

- Fix validation errors reported in status or webhook output.
- Create missing secrets or update `SecretKeySelector` references.
- Ensure storage class and ingress class names match cluster resources.

---

### Issue: PostgreSQL auto-provisioning fails

**Possible causes**

- Storage class not found or PVC stuck in `Pending`.
- StatefulSet fails readiness probes.
- Generated secret missing (operator lacked permissions).

**Diagnostics**

```bash
kubectl get pvc -l app.kubernetes.io/component=database -n <namespace>
kubectl describe statefulset <mint-name>-postgres -n <namespace>
kubectl logs statefulset/<mint-name>-postgres -c postgres -n <namespace>
```

**Resolutions**

- Confirm `spec.database.postgres.autoProvisionSpec.storageClassName` exists.
- Increase storage size or adjust resource requests in `autoProvisionSpec`.
- Ensure operator has RBAC to manage Secrets and StatefulSets.

---

### Issue: Deployment not rolling out after config change

**Possible causes**

- ConfigMap hash unchanged (no spec change detected).
- Rolling update blocked by readiness/liveness probe failure.
- Image pull failure or crash loops.

**Diagnostics**

```bash
kubectl get deployment <mint-name> -n <namespace>
kubectl describe deployment <mint-name> -n <namespace>
kubectl logs deployment/<mint-name> -c mintd -n <namespace>
```

**Resolutions**

- Confirm spec change updates config hash: reconcile by editing `CashuMint` (`kubectl annotate --overwrite` to force change).
- Check readiness and liveness endpoints (`/v1/info`, `/health`) for errors.
- Ensure container image exists and credentials for private registries are configured via `spec.imagePullSecrets`.

---

### Issue: Ingress not working

**Possible causes**

- Ingress resource not created (`spec.ingress.enabled=false` by mistake).
- cert-manager missing or issuer misconfigured.
- DNS records not updated or pointing to wrong LB IP.

**Diagnostics**

```bash
kubectl get ingress <mint-name> -n <namespace>
kubectl describe ingress <mint-name> -n <namespace>
```

If TLS enabled via cert-manager:
```bash
kubectl describe certificate <mint-name> -n <namespace>
kubectl logs -n cert-manager deployment/cert-manager
```

**Resolutions**

- Set `spec.ingress.enabled=true` and provide `spec.ingress.host`.
- Verify `spec.ingress.tls.certManager.issuerName` and `issuerKind` exist.
- Update DNS to the ingress controller’s external IP.
- If using manual TLS secret, ensure it’s populated with valid cert/key.

---

## 2. Debugging Commands

```bash
# Check CashuMint status and conditions
kubectl get cashumint
kubectl describe cashumint <name>

# List generated resources
kubectl get all -l app.kubernetes.io/instance=<mint-name>

# Operator logs
kubectl logs -n cashu-operator-system deployment/cashu-operator-controller-manager

# Mint application logs
kubectl logs deployment/<mint-name> -c mintd -n <namespace>

# PostgreSQL logs (auto-provisioned)
kubectl logs statefulset/<mint-name>-postgres -n <namespace>

# Inspect ConfigMap
kubectl get configmap <mint-name>-config -o yaml -n <namespace>
```

---

## 3. Status Conditions

`CashuMint` status surfaces the following conditions:

| Condition         | Meaning                                                     | Typical Remediation                                        |
|-------------------|-------------------------------------------------------------|-------------------------------------------------------------|
| `Ready`           | Overall availability. `True` when all components ready.     | Review component-specific conditions if `False`.            |
| `DatabaseReady`   | Database connection or provisioning succeeded.              | Check connection URL, credentials, PVC readiness.           |
| `LightningReady`  | Lightning backend reachable and authenticated.              | Validate backend configuration and secrets.                 |
| `ConfigValid`     | Operator validated spec and generated config successfully.  | Fix spec fields referenced in validation error message.     |
| `IngressReady`    | Ingress resource available and (if TLS) certificates active.| Check ingress status, DNS, cert-manager logs.               |

Use `kubectl get cashumint <name> -o jsonpath='{.status.conditions}'` to get raw condition payloads for automation.

---

## 4. Getting Help

- **Report issues**: open GitHub issues at [asmogo/cashu-operator](https://github.com/asmogo/cashu-operator/issues).
- **Logs and diagnostics**: provide operator logs, `kubectl describe cashumint <name>`, and relevant YAML snippets when filing bugs.
- **Community**: join the Cashu community channels (Matrix, Nostr, etc.) for operational discussions and support.

When seeking assistance, include:
1. Operator version (`kubectl -n cashu-operator-system get deployment cashu-operator-controller-manager -o jsonpath='{.spec.template.spec.containers[0].image}'`)
2. Kubernetes version (`kubectl version --short`)
3. Sanitized `CashuMint` spec (remove secrets) and relevant events (`kubectl get events -n <namespace>`).