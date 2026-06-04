# Terraform: Confidential Cashu Mint (GCP + Intel TDX + Trustee KBS)

This Terraform configuration recreates the full confidential-mint environment described in
[`docs/confidential-tee-deployment.md`](../docs/confidential-tee-deployment.md): a GKE cluster with a
peer-pods worker pool, the Confidential Containers operator + cloud-api-adaptor, a Trustee KBS, and
the `tee-mint` workload whose wallet mnemonic is delivered via remote attestation (never stored in a
workload Secret).

The in-cluster pieces (CoCo operator, CAA, KBS, mint) were **captured from the live cluster** into
`manifests/` and `templates/`, and are applied with the `gavinbunney/kubectl` provider. The GKE
cluster, node pools, IAM, networking, and the KBS keypair are native Terraform resources.

---

## What Terraform manages

| Layer | Resources |
|---|---|
| GCP | project APIs, `peerpods` service account + IAM, GKE cluster `cluster-1`, `default-pool` + `peerpods-worker` node pools, external static IP, reserved internal IP for KBS, forwarder firewall rule, optional GCS bucket |
| Platform | CoCo CRDs, operator core + RBAC, `CcRuntime` CR, cloud-api-adaptor daemonset + SA + `ca-bundle`, `peer-pods-cm` (templated), empty `peer-pods-secret` (ADC) |
| KBS | fresh ed25519 admin keypair, auth/admin Secrets, `kbs-config`, allow-policy, PVC, Deployment, internal LB Service, policy + mnemonic provisioning Jobs |
| Mint | `tee-mint-config`, PVC, Service (NEG disabled), Deployment (initdata + sealed reference), ManagedCertificate, Ingress |

## Prerequisites (NOT created by Terraform)

1. **Confidential pod-VM image.** The TDX pod-VM image (`coco-podvm-fedora-mkosi-...-tdx`) is built
   out-of-band with `mkosi` from cloud-api-adaptor `v0.21.0` (`TEE_PLATFORM=tdx`, deny-exec policy
   baked in) and imported as a GCP image. Set its full resource path in `var.podvm_image_name`. See
   `docs/confidential-tee-deployment.md` §3.
2. **Worker-node containerd setting.** `discard_unpacked_layers=false` in the worker node's
   `/etc/containerd/config.toml` (needed for large pod-VM image pulls). Apply via a DaemonSet or
   node startup script; it is **not** reboot-persistent on GKE by default.
3. **DNS.** Create an A record for `var.domain` pointing at the `mint_external_ip` output. (For
   `ucash.space` this is at Namecheap, not Cloud DNS.)
4. **Credentials.** `gcloud auth application-default login` (or a service-account key) with rights to
   create GKE/compute/IAM resources in the project.
5. **Quotas.** TDX pod VMs use `C3_CPUS`; ensure quota in the region for the expected pod count.
6. The wallet **mnemonic** (`var.mint_mnemonic`).

## Intentional deviations from the captured live state

- **KBS reachability:** the live cluster used a NodePort at the worker node InternalIP (not stable).
  Terraform instead exposes KBS via an **internal L4 load balancer at a reserved internal IP**, which
  is stable and known at apply time, so the initdata can be generated in one pass. Same feature
  (pod VMs reach KBS in-VPC), more robust.
- **KBS keypair:** a **fresh** ed25519 admin keypair is generated (the live one is not reused).
- The legacy `keys` Secret mount on KBS (wrong storage layout) is dropped; the mnemonic is
  provisioned via the admin API into the PVC-backed store.
- The `cashu-operator` itself is **not** managed here (it was scaled to 0 and the mint is applied
  directly). Deploy it separately with `make deploy` if you want the CRD/controller present.

## Usage

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars   # edit: project_id, domain, podvm_image_name, mint_mnemonic
tofu init        # or: terraform init

# Provider config depends on the cluster, so create it first, then the rest:
tofu apply -target=google_container_cluster.primary \
           -target=google_container_node_pool.default \
           -target=google_container_node_pool.worker
tofu apply

# Connect kubectl
$(tofu output -raw get_credentials_command)
```

`mint_mnemonic` is sensitive — keep it out of git. Provide it via an un-committed `terraform.tfvars`,
`TF_VAR_mint_mnemonic`, or a secrets manager.

## Convergence notes

This stack has eventual-consistency steps that Terraform does not block on:

- After the `CcRuntime` CR is applied, the CoCo operator takes a few minutes to install kata-deploy
  and create the `kata-remote` RuntimeClass. The mint Deployment will stay `Pending` until it exists.
- The mint's first pod VM boots, attests to KBS, and unseals the mnemonic. If the KBS provisioning
  Jobs haven't completed yet, the mint container will `CrashLoopBackOff` with
  `mnemonic has an invalid word count: 1` and recover automatically once the resource is present.
- The GKE ManagedCertificate takes several minutes to go `Active` after DNS resolves.

If `tofu apply` reports the mint not ready, wait and re-run, or check:

```bash
kubectl get pods -A | grep -E 'cc-operator|cloud-api-adaptor|kbs|tee-mint'
kubectl logs -n coco-tenant deploy/kbs | grep -E 'tee=Tdx|resource/|200|401'
curl -s https://$(tofu output -raw mint_url)
```

## Layout

```
terraform/
  versions.tf providers.tf variables.tf locals.tf outputs.tf
  gke.tf            # GCP infra: APIs, SA/IAM, cluster, node pools, IPs, firewall, bucket
  platform.tf       # CoCo operator + CAA (captured manifests)
  kbs.tf            # KBS keypair, secrets, manifests, internal LB, provisioning jobs
  mint.tf           # mint config, service/pvc, ingress, deployment
  manifests/        # captured static YAML (platform/, kbs/, mint/)
  templates/        # *.tftpl rendered with variables (initdata, peer-pods-cm, kbs/mint)
  scripts/clean_manifests.py   # how the manifests were captured from the live cluster
```

## Re-capturing manifests

If you change the live cluster and want to refresh the captured manifests:

```bash
kubectl get <kind> <name> -n <ns> -o json | python3 scripts/clean_manifests.py > manifests/<group>/<file>.yaml
```
