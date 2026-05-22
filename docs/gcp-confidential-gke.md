# GKE Confidential Cashu Mint

This deployment path runs `mintd` on GKE Standard Confidential GKE Nodes using AMD SEV and a vTPM. The operator still reconciles ordinary Kubernetes resources; GCP IAM, ComputeClass, Secret Manager, and Workload Identity Federation stay outside the operator and are configured explicitly.

## Architecture

1. The `CashuMint` pod is scheduled with `cloud.google.com/compute-class: cashu-confidential-sev`.
2. The mint container requests `google.com/cc: "1"` so the Google `cc-device-plugin` exposes `/dev/tpmrm0`.
3. The wrapper entrypoint gets a Google Cloud Attestation token from the vTPM.
4. The wrapper rejects the token unless local claims match the configured issuer, audience, `hwmodel=GCP_AMD_SEV`, secure boot, service account, project, and optional zone.
5. The wrapper exchanges the attestation token with Google STS through a Workload Identity Federation provider.
6. The wrapper reads the mint mnemonic from Secret Manager and execs `cdk-mintd` with `CDK_MINTD_MNEMONIC` set only in the child process environment.

## Requirements

- GKE Standard with workload-level Confidential GKE Nodes support.
- AMD SEV nodes. GKE vTPM workload support is currently limited to Confidential GKE Nodes that use AMD SEV.
- Container-Optimized OS with containerd.
- The Google `cc-device-plugin` DaemonSet.
- A Secret Manager secret version containing the mint mnemonic.
- A Workload Identity Federation provider that trusts Google Cloud Attestation tokens.
- External PostgreSQL over TLS. Do not use operator-managed PostgreSQL as the secure production default for this path.

Google references:

- [Confidential GKE Nodes](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/confidential-gke-nodes)
- [vTPM in GKE workloads](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/vtpms)
- [Google Cloud Attestation](https://docs.cloud.google.com/confidential-computing/docs/attestation)
- [Confidential VM token claims](https://docs.cloud.google.com/confidential-computing/confidential-vm/docs/token-claims)

## Build the wrapper image

Build the image with the upstream `cashubtc/mintd` image pinned by digest:

```sh
make mintd-confidential-build \
  MINTD_BASE_IMAGE='cashubtc/mintd@sha256:1551b1b56f8670942164d3831ad00b54d662310bc811458b413a59ffcc7a152e' \
  MINTD_CONFIDENTIAL_IMG='REGION-docker.pkg.dev/PROJECT_ID/cashu/cashu-mintd-gke-confidential:0.1.0'
```

Push and use the pushed digest in the `CashuMint.spec.image` field:

```sh
make mintd-confidential-push \
  MINTD_CONFIDENTIAL_IMG='REGION-docker.pkg.dev/PROJECT_ID/cashu/cashu-mintd-gke-confidential:0.1.0'
```

The image is intentionally thin: it copies a static `cashu-attested-entrypoint` binary into the existing CDK `mintd` image and sets that wrapper as the entrypoint.

## Configure GKE

Install the vTPM device plugin:

```sh
kubectl create -f https://raw.githubusercontent.com/google/cc-device-plugin/main/manifests/cc-device-plugin.yaml
```

Create the ComputeClass and mint resources from the sample after replacing placeholders:

```sh
kubectl apply -f config/samples/mint_v1alpha1_cashumint_gke_confidential.yaml
```

The sample includes:

- `ComputeClass/cashu-confidential-sev` with `nodePoolConfig.confidentialNodeType: SEV`
- `ServiceAccount/cashumint-gke-confidential`
- `CashuMint.spec.serviceAccountName: cashumint-gke-confidential`
- `CashuMint.spec.nodeSelector.cloud.google.com/compute-class: cashu-confidential-sev`
- `CashuMint.spec.nodeSelector.cloud.google.com/gke-confidential-nodes-instance-type: SEV`
- `CashuMint.spec.resources.limits.google.com/cc: "1"`
- pod and container security context overrides that run the mint container as UID `0`, because TPM device nodes are commonly root-owned
- `CashuMint.spec.extraEnv` for the wrapper configuration

## Wrapper environment

Required variables:

| Variable | Purpose |
| --- | --- |
| `CASHU_ATTESTATION_PROJECT_ID` | Project used for Google Cloud Attestation API calls. |
| `CASHU_ATTESTATION_LOCATION` | Attestation API location, for example `us-central1`. |
| `CASHU_ATTESTATION_WORKLOAD_IDENTITY_PROVIDER` | Full STS audience for the Workload Identity Federation provider. |
| `CASHU_ATTESTATION_EXPECTED_SERVICE_ACCOUNT` | Expected GCP service account claim in the attestation token. |
| `CASHU_MNEMONIC_SECRET_VERSION` | Secret Manager version path, for example `projects/PROJECT_ID/secrets/cashumint-mnemonic/versions/latest`. |

Optional variables:

| Variable | Default |
| --- | --- |
| `CASHU_ATTESTATION_EXPECTED_AUDIENCE` | `https://sts.googleapis.com` |
| `CASHU_ATTESTATION_EXPECTED_HW_MODEL` | `GCP_AMD_SEV` |
| `CASHU_ATTESTATION_EXPECTED_PROJECT_ID` | `CASHU_ATTESTATION_PROJECT_ID` |
| `CASHU_ATTESTATION_EXPECTED_ZONE` | unset |
| `CASHU_MINTD_BINARY` | `cdk-mintd` |
| `CASHU_TPM_PATH` | `/dev/tpmrm0` |

## IAM model

Grant the Kubernetes service account enough identity to call the Attestation API, but grant Secret Manager access to the attested identity through the Workload Identity Federation provider. The secret access policy should be scoped to the principal or principal set that represents the attested token attributes you validate locally.

At minimum, create:

```sh
gcloud secrets create cashumint-mnemonic --replication-policy=automatic
printf '%s' 'twenty four word mnemonic here' | gcloud secrets versions add cashumint-mnemonic --data-file=-
```

Then configure the Workload Identity Federation pool/provider and bind `roles/secretmanager.secretAccessor` on the mnemonic secret only to that provider principal. Keep the pod's regular Workload Identity service account out of the secret accessor role.

## Verify

Confirm scheduling and vTPM exposure:

```sh
kubectl get pod -l app.kubernetes.io/instance=cashumint-gke-confidential -o wide
kubectl get pod -l app.kubernetes.io/instance=cashumint-gke-confidential -o jsonpath='{.items[0].spec.nodeSelector.cloud\.google\.com/compute-class}'
kubectl get pod -l app.kubernetes.io/instance=cashumint-gke-confidential -o jsonpath='{.items[0].spec.containers[?(@.name=="mintd")].resources.limits.google\.com/cc}'
kubectl get node NODE_NAME -o jsonpath='{.metadata.labels.cloud\.google\.com/gke-confidential-nodes-instance-type}'
```

The node label should be `SEV`, the container should have `google.com/cc: 1`, and `GET /v1/info` should return the mint info. As a negative test, set an incorrect `CASHU_ATTESTATION_EXPECTED_PROJECT_ID` or `CASHU_ATTESTATION_EXPECTED_SERVICE_ACCOUNT`; the pod should crash before `cdk-mintd` starts.

## Local development

You can test the wrapper decision logic locally without a cloud cluster:

```sh
go test ./internal/attestedentrypoint
```

These tests use fake attestation, STS, Secret Manager, and exec implementations. They prove that the wrapper fails closed and only starts `cdk-mintd` after attestation, claim validation, token exchange, and Secret Manager retrieval succeed.

Tilt also has local resources for this path:

```sh
make tilt-up
```

In the Tilt UI:

- `confidential-wrapper-tests` runs the wrapper unit tests and builds a static Linux entrypoint binary.
- `confidential-wrapper-image` builds `cashu-mintd-gke-confidential:dev` locally.
- `confidential-wrapper-smoke` runs the built image and verifies it fails closed without required config and without `/dev/tpmrm0`.

Tilt does not run a successful attested mint locally, because k3d and Kind don't provide GKE Confidential Nodes, Google Cloud Attestation, or `/dev/tpmrm0` through `google.com/cc`. A local Kubernetes run can only prove build behavior and fail-closed behavior; the positive attestation path still requires GKE AMD SEV hardware.
