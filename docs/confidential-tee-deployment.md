# Confidential Cashu Mint on GCP (Intel TDX + Trustee KBS)

This document explains how the `tee.ucash.space` Cashu mint runs **inside a hardware Trusted
Execution Environment (TEE)** and how its wallet mnemonic is delivered to the mint **only after
the TEE proves its identity via remote attestation** — so the mnemonic never lives in plaintext in
a Kubernetes Secret that an operator/cluster-admin can read.

It is a companion to the operator docs in this repo. The operator itself was scaled to `0` for this
deployment; everything below is applied imperatively so the moving parts are explicit.

---

## 1. What this gives you

- **The mint binary runs in an Intel TDX confidential VM.** Memory is encrypted by the CPU; the
  host/hypervisor/cluster operator cannot read it.
- **The mnemonic is held in a Key Broker Service (KBS).** It is released to the workload **only**
  after the KBS's Attestation Service verifies a genuine TDX quote from the VM.
- **No plaintext mnemonic in the workload's Kubernetes Secret.** The pod spec carries only a
  `sealed.` *reference*; the real secret is fetched + decrypted inside the TEE at container start.
- **No shell access.** `kubectl exec` into the mint is denied by a baked-in kata agent policy.
- **Public HTTPS** via a GKE L7 load balancer + Google-managed certificate.
- **On-chain (BDK) backend on Mutinynet (signet)** for testing.

---

## 2. High-level architecture

```
                        Internet
                           │  https://tee.ucash.space
                           ▼
              ┌─────────────────────────────┐
              │ GKE L7 HTTPS LB             │  static IP + ManagedCertificate
              │ (Ingress, instance-group BE)│
              └──────────────┬──────────────┘
                             │  :8085
        GKE worker node      ▼
   ┌─────────────────────────────────────────────────────────────┐
   │ kata-remote runtime (peer-pods / cloud-api-adaptor)           │
   │                                                               │
   │   tee-mint Pod  ──────────► spawns a remote VM ───────────┐   │
   │   (placeholder on node)                                   │   │
   └───────────────────────────────────────────────────────────┼──┘
                                                                 │
                        Google Compute Engine                    ▼
        ┌──────────────────────────────────────────────────────────────┐
        │ Confidential VM  (c3-standard-4, Intel TDX)                    │
        │                                                                │
        │  systemd: attestation-agent (AA), confidential-data-hub (CDH), │
        │           kata-agent, agent-protocol-forwarder                 │
        │                                                                │
        │  ┌──────────────┐   unseal env    ┌───────────────────────┐    │
        │  │ cdk-mintd     │◄───────────────│ kata-agent            │    │
        │  │ (the mint)    │  plaintext      │  detects "sealed." env │    │
        │  └──────┬───────┘  mnemonic        └─────────┬─────────────┘    │
        │         │ onchain (BDK)                      │ ttRPC            │
        └─────────┼────────────────────────────────────┼─────────────────┘
                  │                                     ▼
                  │ Esplora                   ┌───────────────────┐
                  ▼                           │ CDH (in-VM)       │
         https://mutinynet.com/api            │ vault unseal      │
                                              └─────────┬─────────┘
                                                        │ RCAR attestation + GET resource
                                                        ▼
                                   ┌────────────────────────────────────┐
                                   │ Trustee KBS  (coco-tenant ns)        │
                                   │  - built-in Attestation Service      │
                                   │  - verifies TDX quote                │
                                   │  - resource policy (allow)           │
                                   │  - resource store (PVC)              │
                                   │     reponame/workload_key/key.bin    │
                                   └──────────────────────────────────────┘
```

---

## 3. Component inventory (versions matter)

| Component | Version / identifier | Notes |
|---|---|---|
| cloud-api-adaptor (CAA) | `v0.21.0` | peer-pods controller; creates the remote confidential VM |
| kata-deploy | `3.31.0` | via `CcRuntime ccruntime-peer-pods`; provides `kata-remote` RuntimeClass |
| guest-components (AA + CDH) | commit `f1561038…424a0` | **pinned**; the unseal/attestation logic described here is version-specific |
| Trustee KBS | `key-broker-service:built-in-as-v0.19.0` | KBS with embedded Attestation Service |
| kbs-client | `v0.19.0` | admin CLI used to provision resources/policy |
| cdk-mintd | `cashubtc/mintd:0.17.0-rc.0` | the mint; onchain/BDK backend |
| Pod VM image | `coco-podvm-fedora-mkosi-tee-amd-v0210-tdx` | mkosi Fedora image, TDX, **deny-exec policy baked in** |

GCP: project `project-4d990f4d-…`, zone `us-central1-a`, machine `c3-standard-4`,
`--confidential-compute-type=TDX`, disk `pd-balanced`. Pod-VM machine type is pinned per-pod via the
annotation `io.katacontainers.config.hypervisor.machine_type=c3-standard-4`.

> **Why TDX and not SEV-SNP?** CAA `v0.21.0` ships prebuilt guest-components attesters for `tdx`
> (and `none`) but not `snp`/`az-snp-vtpm` at the pinned commit, and the SNP build path failed
> pulling `attestation-agent:…-snp`. GCP TDX quotes verify cleanly in the KBS built-in AS.

---

## 4. The secret-injection flow (the core idea)

### 4.1 What's in Kubernetes

The mint Deployment (`tee-mint`, `default` ns) sets the mnemonic env vars to a **sealed-secret
reference string**, not a value and not a `secretKeyRef`:

```
CDK_MINTD_MNEMONIC      = sealed.<header>.<payload>.<signature>
CDK_MINTD_BDK_MNEMONIC  = sealed.<header>.<payload>.<signature>
```

The `payload` (base64url JSON) is a **vault** secret that only *points* at a KBS resource:

```json
{
  "version": "0.1.0",
  "type": "vault",
  "name": "kbs:///reponame/workload_key/key.bin",
  "provider": "kbs",
  "provider_settings": {},
  "annotations": {}
}
```

So a cluster admin reading the Deployment sees only `kbs:///reponame/workload_key/key.bin`.

### 4.2 What happens at pod start

1. **VM provisioning.** CAA creates a TDX confidential VM from the pod-VM image. The pod's
   `io.katacontainers.config.hypervisor.cc_init_data` annotation (gzip+base64 TOML) is delivered as
   VM user-data and **measured into a TDX RTMR**.
2. **process-user-data** unpacks the initdata into `/run/peerpod/aa.toml`, `/run/peerpod/cdh.toml`
   (and `policy.rego` if present).
3. **systemd** starts the Attestation Agent and Confidential Data Hub.
   - CDH reads `/run/peerpod/cdh.toml`. Its `Hub::new()` calls `set_configuration_envs()`, which
     exports `AA_KBC_PARAMS=cc_kbc::http://<KBS>:30510` from the `[kbc]` section. This is **required**
     because the CDH "kbs" KMS getter reads the KBS URL from the `AA_KBC_PARAMS` env var.
4. **Container creation.** When the kata-agent creates the mint container, `cdh_handler_sealed_secrets`
   walks the env vars. For any value starting with `sealed.` it calls CDH `UnsealSecret` over ttRPC.
5. **CDH unseal** (`provider = kbs`, vault type): the KMS "kbs" getter builds a `cc_kbc` client and
   performs the **RCAR attestation handshake** with the KBS:
   - `POST /kbs/v0/auth` → `POST /kbs/v0/attest` (sends the TDX quote/evidence).
   - The KBS **built-in Attestation Service verifies the TDX quote** (`Quote DCAP check succeeded`,
     `tee=Tdx`) and issues a short-lived token.
   - `GET /kbs/v0/resource/reponame/workload_key/key.bin` with the token → the **resource policy**
     is evaluated (currently `default allow = true`) → KBS returns the mnemonic (JWE-wrapped to the
     VM's ephemeral key).
6. **Injection.** CDH returns the plaintext to the kata-agent, which replaces the env value
   in-place. `cdk-mintd` then starts with the real 24-word mnemonic — which **only ever exists in
   encrypted TEE memory**.

If the unseal *fails*, the kata-agent logs a warning and leaves the raw `sealed.` string in the env;
`cdk-mintd` then dies with `mnemonic has an invalid word count: 1` (a useful failure signature).

### 4.3 Signature verification mode

The deployed `cdh.toml` sets `skip_sealed_secret_verification = true`, so CDH trusts the reference
payload without checking the JWS signature. Security is still enforced by **attestation** (KBS won't
release the resource to a non-TEE), and only a reference (not the secret) lives in Kubernetes.

A hardened mode is prepared but not enabled: an ES256-signed reference whose `kid` points at a
public JWK stored in KBS (`skip_sealed_secret_verification = false`). That adds tamper-evidence to
the reference string itself.

---

## 5. initdata contents

The `cc_init_data` annotation is `base64(gzip(TOML))` of:

```toml
algorithm = "sha384"
version   = "0.1.0"

[data]
"aa.toml" = """
[token_configs]
[token_configs.kbs]
url = "http://10.128.0.9:30510"

[eventlog_config]
init_pcr = 17
enable_eventlog = false
"""

"cdh.toml" = """
socket = "unix:///run/confidential-containers/cdh.sock"
skip_sealed_secret_verification = true
[kbc]
name = "cc_kbc"
url = "http://10.128.0.9:30510"
"""
```

- `aa.toml.token_configs.kbs.url` — where AA fetches the attestation token.
- `cdh.toml.[kbc].url` — where CDH fetches the resource (and the source for `AA_KBC_PARAMS`).
- `10.128.0.9:30510` is the **GKE worker node InternalIP : KBS NodePort**. The pod VM lives in the
  same VPC and reaches it via the `default-allow-internal` firewall rule.
- `policy.rego` is intentionally **omitted** from initdata so the agent keeps the deny-exec policy
  baked into the image (see §7).

> The KBS URL is currently a node InternalIP, which is **not stable** across node replacement. For
> production, expose KBS via a stable internal LB address and rebuild the initdata.

---

## 6. The Key Broker Service (Trustee KBS)

- Namespace `coco-tenant`; Deployment `kbs`; image `…/key-broker-service:built-in-as-v0.19.0`.
- Reachable from pod VMs at **node InternalIP : NodePort 30510**; in-cluster at
  `kbs.coco-tenant.svc.cluster.local:8080`.
- **Storage is a PVC** (`kbs-storage`, `standard-rwo`) mounted at `/opt/confidential-containers/kbs`,
  with the Deployment strategy set to `Recreate` (RWO single-attach). This makes the resource store
  and the resource policy **survive KBS restarts** (an earlier `emptyDir` did not).
- **Resource layout (v0.19.0 quirk):** the kvstorage LocalFs backend stores a resource under
  `repository/` as a single **URL-escaped flat filename**, e.g.
  `repository/reponame\x2Fworkload_key\x2Fkey.bin`. Mounting a secret as a *directory tree*
  (`repository/reponame/workload_key/key.bin`) does **not** resolve — always write via the admin API.
- Resource policy is currently permissive: `package policy` / `default allow = true`.

### 6.1 Provision / update the mnemonic resource

`set-resource` writes through the same backend the reader uses (layout-agnostic). Source the
mnemonic from a Secret mounted into a one-shot Job:

```bash
# 1. stage the mnemonic into coco-tenant (transient)
MNE=$(kubectl get secret tee-mint-mnemonic -n default -o jsonpath='{.data.mnemonic}')
kubectl apply -n coco-tenant -f - <<EOF
apiVersion: v1
kind: Secret
metadata: { name: kbs-mnemonic-prov, namespace: coco-tenant }
type: Opaque
data: { mnemonic: ${MNE} }
EOF

# 2. write it via the KBS admin API
kubectl apply -f - <<'EOF'
apiVersion: batch/v1
kind: Job
metadata: { name: kbs-setresource, namespace: coco-tenant }
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: kbs-client
          image: quay.io/confidential-containers/kbs-client:v0.19.0
          command: ["kbs-client","--url","http://kbs.coco-tenant.svc.cluster.local:8080",
                    "config","--auth-private-key","/keys/kbs.key",
                    "set-resource","--path","reponame/workload_key/key.bin",
                    "--resource-file","/mnt/mnemonic"]
          volumeMounts: [{name: keys, mountPath: /keys}, {name: mne, mountPath: /mnt}]
      volumes:
        - {name: keys, secret: {secretName: kbs-admin-key}}
        - {name: mne,  secret: {secretName: kbs-mnemonic-prov}}
EOF

# 3. clean up (the Job log prints the mnemonic in base64 — delete it!)
kubectl delete job kbs-setresource -n coco-tenant
kubectl delete secret kbs-mnemonic-prov -n coco-tenant
```

### 6.2 Provision / update the resource policy

```bash
kubectl apply -f - <<'EOF'   # uses configmap kbs-allow-policy + secret kbs-admin-key
apiVersion: batch/v1
kind: Job
metadata: { name: kbs-setpolicy, namespace: coco-tenant }
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: kbs-client
          image: quay.io/confidential-containers/kbs-client:v0.19.0
          command: ["kbs-client","--url","http://kbs.coco-tenant.svc.cluster.local:8080",
                    "config","--auth-private-key","/keys/kbs.key",
                    "set-resource-policy","--policy-file","/policy/allow.rego"]
          volumeMounts: [{name: keys, mountPath: /keys}, {name: policy, mountPath: /policy}]
      volumes:
        - {name: keys, secret: {secretName: kbs-admin-key}}
        - {name: policy, configMap: {name: kbs-allow-policy}}
EOF
```

---

## 7. No-shell hardening (deny exec)

The pod-VM image bakes `allow-all-except-exec-process.rego` as the default kata-agent policy
(`ExecProcessRequest := false`, everything else `true`). Result:

```
$ kubectl exec tee-mint-... -- id
... PermissionDenied: "ExecProcessRequest is blocked by policy"
```

Container **logs still work** (`ReadStreamRequest`, `WaitProcessRequest` allowed). Baking the policy
into the image is more robust than shipping it via initdata (initdata policy can break container
creation if it misses a request type, and initdata delivery itself depends on measured boot).

---

## 8. Networking / ingress

- Static IP `tee-ucash-ip` → Ingress → Service `tee-mint:8085`.
- Google-managed cert `tee-ucash-cert` for `tee.ucash.space`.
- **Container-native NEG is disabled** on the Service:
  `cloud.google.com/neg: '{"ingress": false}'`. NEGs target pod IPs directly, but the mint's pod IP
  is a tunneled peer-pod address the LB can't reach — the instance-group backend (NodePort) works.
- DNS for `ucash.space` is at Namecheap (not Cloud DNS); the A record points at the static IP.

---

## 9. The mint itself (cdk-mintd 0.17)

- `runtimeClassName: kata-remote`.
- Backend: **onchain / BDK on Mutinynet (signet)** — Mutinynet is a custom signet, so
  `network=signet` + the Mutinynet Esplora URL is the correct configuration.

```
CDK_MINTD_LN_BACKEND=none
CDK_MINTD_ONCHAIN_BACKEND=bdk
CDK_MINTD_BDK_NETWORK=signet
CDK_MINTD_BDK_CHAIN_SOURCE_TYPE=esplora
CDK_MINTD_BDK_ESPLORA_URL=https://mutinynet.com/api
CDK_MINTD_BDK_ESPLORA_PARALLEL_REQUESTS=1
CDK_MINTD_BDK_NUM_CONFS=2
CDK_MINTD_MNEMONIC=sealed....            # ← from KBS via attestation
CDK_MINTD_BDK_MNEMONIC=sealed....        # ← from KBS via attestation
CDK_MINTD_MINT_NAME=ucash
CDK_MINTD_MINT_DESCRIPTION=Confidential Cashu mint on Mutinynet ...
```

- `cdk-mintd` env vars are explicit constants (e.g. `CDK_MINTD_MINT_NAME`,
  `CDK_MINTD_MINT_DESCRIPTION`, `CDK_MINTD_MINT_DESCRIPTION_LONG`, `CDK_MINTD_MINT_MOTD`).
- Mint-info from config/env is applied **on every startup** because the management RPC is disabled.
  (If you enable `mint_management_rpc`, mint-info becomes set-once and must be changed via RPC.)
- `/data` (SQLite DB) is a PVC, so mint state persists across pod restarts. The keysets derive
  deterministically from the mnemonic, so the mint pubkey is stable regardless.

---

## 10. End-to-end verification

```bash
# mint is serving
curl -s https://tee.ucash.space/v1/info | jq '{name, description, version}'

# KBS released the resource to an attested TEE (look for 200 on the resource GET)
kubectl logs -n coco-tenant deploy/kbs --tail=20 | grep -E "tee=Tdx|resource/reponame|200|401"

# exec is denied
kubectl exec -n default deploy/tee-mint -- id        # → blocked by policy

# the pod spec carries only a reference, not plaintext
kubectl get deploy tee-mint -n default \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="CDK_MINTD_MNEMONIC")].value}'
```

Diagnostics:

- **Pod-VM serial console** (boot + systemd, incl. AA/CDH startup):
  `gcloud compute instances get-serial-port-output podvm-<pod>-<id> --zone us-central1-a`
- **KBS logs** show every attestation + resource fetch. `resource not found` → the resource isn't at
  the path the backend reads (re-provision via §6.1). `401` on the resource GET → not-found or policy
  denied. `Quote DCAP check succeeded` → attestation is healthy.

---

## 11. Security model & current limitations

**Trust gained**
- The mint runs in HW-encrypted TDX memory; host/operator can't read it.
- The mnemonic is released only to a VM that produces a valid TDX quote.
- No plaintext mnemonic in the workload's Kubernetes Secret.
- No interactive shell into the workload.

**Known gaps / TODO for production**
1. **Plaintext backups still exist** in `tee-mint-mnemonic` (default ns) and the trustee `keys`
   Secret (coco-tenant). They are retained as recovery backups; remove them (after an offline
   backup) to fully meet "no k8s Secret exposes the mnemonic."
2. **Resource policy is `allow`-all.** Attestation is enforced (must be a real TDX TEE) but the
   policy is **not bound to specific measurements** (`mr_td`, RTMRs). The KBS logs
   `No reference value found for mr_td/rtmr_1` — no RVPS reference values are registered. Harden by
   registering reference values and writing a measurement-checking resource policy.
3. **Signature verification disabled** (`skip_sealed_secret_verification = true`). Switch to the
   signed reference (pubkey in KBS) for reference-tamper evidence.
4. **KBS URL is a node InternalIP** baked into initdata — not stable across node replacement. Use a
   stable internal endpoint.
5. **Mutinynet/signet only.** Do **not** switch BDK to mainnet without an explicit review — this is
   real-fund custody.
6. The deny-exec policy and `discard_unpacked_layers=false` containerd setting on the worker node are
   not guaranteed reboot-persistent on GKE; consider a DaemonSet to enforce node config.

---

## 12. Glossary

- **TDX (Trust Domain Extensions)** — Intel CPU feature for confidential VMs (encrypted memory +
  remote attestation via a signed *quote*).
- **Attestation / RCAR** — Request–Challenge–Attestation–Response handshake where the VM proves its
  identity to the KBS before secrets are released.
- **KBS (Key Broker Service)** — Trustee component that verifies attestation evidence and releases
  resources per a Rego policy.
- **AA / CDH** — Attestation Agent (produces evidence) and Confidential Data Hub (fetches/unseals
  secrets), running as systemd services inside the pod VM.
- **peer-pods / cloud-api-adaptor** — runs the workload in a *separate* cloud VM instead of on the
  K8s node, enabling per-pod confidential VMs (`kata-remote` RuntimeClass).
- **Sealed secret (vault)** — a `sealed.<hdr>.<payload>.<sig>` string whose payload references a KBS
  resource; unsealed inside the TEE by CDH.
