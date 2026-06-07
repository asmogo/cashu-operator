# Encrypted PVC Seed Mode

`encryptedPVC` key management is a provider-agnostic way to avoid storing a mint
mnemonic in a Kubernetes Secret. The seed is generated inside the mint pod by a
small init container, stored only as an encrypted envelope on the mint data PVC,
and injected into `cdk-mintd` by rendering the final `/data/config.toml` before
the mint container starts.

This mode is intended for native confidential-node deployments where you want a
simple model without a seed broker or KBS.

## Flow

1. The operator creates a non-secret ConfigMap containing the base `config.toml`.
2. The pod starts a `seed-init` init container before `cdk-mintd`.
3. `seed-init` checks `/data/seed.enc` on the mint PVC.
4. If absent, `seed-init` generates a 24-word BIP39 mnemonic inside the pod using `github.com/cashubtc/cdk-go`.
5. `seed-init` encrypts the mnemonic with a random data-encryption key (DEK).
6. The selected provider wraps/unwraps the DEK.
7. `seed-init` writes only the encrypted envelope to the PVC.
8. `seed-init` renders `/data/config.toml` with `info.mnemonic` and, when BDK is enabled, `bdk.mnemonic`.
9. The no-shell `cdk-mintd` container starts and reads `/data/config.toml`.

No Kubernetes Secret contains the mint mnemonic.

`seed-init` uses `github.com/cashubtc/cdk-go` `v0.17.0-rc.3` for mnemonic
generation. That package is backed by CDK's native FFI library, so the init
image is CGO-based and includes `libcdk_ffi.so` at runtime.

## Providers

The seed-init binary supports a provider interface. Current providers:

- `local`: development/test only. Wraps the DEK with a 32-byte key from a Kubernetes Secret.
- `vaultTransit`: production-oriented. Uses HashiCorp Vault Transit to wrap/unwrap the DEK. Vault stores keys, not mint mnemonics.
- `googleKMS`: Google Cloud KMS provider. Uses Workload Identity/metadata server by default and stores keys, not mint mnemonics.

Provider keys are **not** created by the operator or seed-init. Create the Vault
Transit key, Google KMS key, or local wrapping key before deploying the mint.
This keeps the operator from needing broad key-administration privileges.

Example:

```yaml
spec:
  keyManagement:
    mode: encryptedPVC
    encryptedPVC:
      initImage: ghcr.io/asmogo/cashu-seed-init:latest
      serviceAccountName: encrypted-pvc-mint
      provider:
        type: vaultTransit
        vaultTransit:
      address: https://vault.example.test
          mount: transit
          keyName: cashu-mints/encrypted-pvc-mint
          auth:
            method: kubernetes
            kubernetes:
              role: cashu-mint-encrypted-pvc-mint
```

Google KMS example:

```yaml
spec:
  keyManagement:
    mode: encryptedPVC
    encryptedPVC:
      initImage: ghcr.io/asmogo/cashu-seed-init:latest
      serviceAccountName: encrypted-pvc-mint
      provider:
        type: googleKMS
        googleKMS:
          keyName: projects/my-project/locations/global/keyRings/cashu/cryptoKeys/mint-seeds
```

When running on GKE, grant the mint's workload identity permission to use the
configured key for encrypt/decrypt operations. For local testing, seed-init also
accepts `GOOGLE_OAUTH_ACCESS_TOKEN` in the environment.

## Key-To-Mint Binding

The key provider does not automatically know which mint a key belongs to. Bind a
key to a mint with provider-side policy:

- Vault Transit: create a per-mint or per-tenant Transit key and a Vault policy
  that lets only that mint's Vault Kubernetes auth role use the key.
- Google KMS: grant only that mint's workload identity principal permission to
  encrypt/decrypt with the configured CryptoKey.
- Local: development only; the Kubernetes Secret containing the local wrapping
  key is the binding.

In addition, seed-init binds the encrypted PVC envelope to the mint identity with
AES-GCM associated data. The operator sets this associated data to
`<namespace>/<mint-name>`. If the encrypted envelope is moved to another mint or
namespace, decryption fails unless the associated data matches.

This AAD binding prevents accidental cross-mint reuse. Provider IAM/Vault policy
is still the authorization boundary.

## Security Boundary

This mode improves native GKE deployments by removing the mnemonic from
Kubernetes Secrets, Terraform state, and operator-managed resources. It does not
provide the same per-pod TEE boundary as peer-pods/KBS. On native Confidential
GKE Nodes, the TEE boundary is still the node VM, not each mint pod.

This mode does not currently perform hardware attestation before unwrapping the
seed. It relies on provider authentication, provider policy, Kubernetes
scheduling, and admission/RBAC controls. Adding quote-based attestation would
require a provider that verifies TEE evidence or a node-attestation agent that
issues short-lived attestations for the confidential node.

Use this mode with:

- a no-shell `cdk-mintd` image
- restricted Pod Security
- RBAC that denies exec/attach/port-forward/ephemeral containers
- node selectors/taints for confidential nodes
- admission policy that prevents arbitrary pods from using mint service accounts or mounting mint PVCs

## Recovery

The encrypted envelope on the PVC is required to preserve the mint identity. If
the PVC is lost, the mnemonic is lost unless you have a backup. Back up the PVC
and the provider wrapping key material according to your provider's recovery
process.
