# Sample catalog

The sample manifests in `config/samples/` are meant to be copied and adapted, not applied as a single production bundle. This page groups them by use case so it is easier to find the right starting point.

## Quick start and templates

| File | Purpose | Notes |
| --- | --- | --- |
| [`mint_v1alpha1_cashumint_minimal.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_minimal.yaml) | Smallest working mint | SQLite, `fakeWallet`, auto-generated mnemonic |
| [`mint_v1alpha1_cashumint.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint.yaml) | Annotated starter template | Good copy/paste base for a new mint |

## Database-focused samples

| File | Purpose | Highlights |
| --- | --- | --- |
| [`mint_v1alpha1_cashumint_postgres_auto.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_postgres_auto.yaml) | Operator-managed PostgreSQL | StatefulSet, Service, Secret, storage sizing |
| [`mint_v1alpha1_cashumint_external_postgres.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_external_postgres.yaml) | Existing PostgreSQL | `urlSecretRef`, external DB TLS mode |
| [`mint_v1alpha1_cashumint_backup_s3.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_backup_s3.yaml) | S3 backups and restore trigger | Backup `CronJob`, restore annotations |
| [`mint_v1alpha1_cashumint_backup_s3_creds_secret.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_backup_s3_creds_secret.yaml) | Helper Secret for backup credentials | Example only; replace values |
| [`mint_v1alpha1_cashumint_backup_s3_mnemonic_secret.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_backup_s3_mnemonic_secret.yaml) | Helper Secret for backup demo mnemonic | Example only; replace mnemonic |

## Payment backend samples

| File | Backend | Highlights |
| --- | --- | --- |
| [`mint_v1alpha1_cashumint_lnd.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_lnd.yaml) | LND | Macaroon and TLS cert Secret refs |
| [`mint_v1alpha1_cashumint_lnbits.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_lnbits.yaml) | LNBits | Admin and invoice API key Secret refs |
| [`mint_v1alpha1_cashumint_cln.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_cln.yaml) | CLN | Socket path and BOLT12 toggle |
| [`mint_v1alpha1_cashumint_grpc_processor_external.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_grpc_processor_external.yaml) | External gRPC processor | Explicit processor address, port, and client TLS Secret |
| [`mint_v1alpha1_cashumint_spark_breez.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_spark_breez.yaml) | Spark/Breez sidecar | Sidecar image and Breez env wiring |
| [`mint_v1alpha1_cashumint_spark_processor.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_spark_processor.yaml) | Spark/Breez sidecar + example Secrets | Expanded example including Secret resources |
| [`mint_v1alpha1_cashumint_stripe_processor.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_stripe_processor.yaml) | Stripe sidecar | Secret-driven Stripe config |

## Advanced operator features

| File | Feature | Highlights |
| --- | --- | --- |
| [`mint_v1alpha1_cashumint_auth_httpcache.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_auth_httpcache.yaml) | Auth, Redis cache, metrics, limits | OIDC auth, Redis-backed cache, management RPC, Prometheus |
| [`mint_v1alpha1_cashumint_ldk_node.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_ldk_node.yaml) | LDK sidecar | Bitcoin RPC credentials, network and gossip config |
| [`mint_v1alpha1_cashumint_orchard_sqlite.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_orchard_sqlite.yaml) | Orchard with SQLite mint | Orchard PVC, Service, Ingress, AI endpoint |
| [`mint_v1alpha1_cashumint_orchard_postgres.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_orchard_postgres.yaml) | Orchard with PostgreSQL mint | Auto-generated mnemonic, management RPC mTLS, cert-manager |
| [`mint_v1alpha1_cashumint_orchard_setup_secret.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_orchard_setup_secret.yaml) | Helper Secret for Orchard | Example setup key Secret |
| [`mint_v1alpha1_cashumint_production.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_production.yaml) | Production-style reference | PostgreSQL, backups, ingress, Prometheus, resource tuning, external gRPC processor |

## Adapting a sample safely

1. Replace hostnames, URLs, and Secret names with values that exist in your namespace.
2. Decide whether you want `mintInfo.autoGenerateMnemonic=true` or a user-managed mnemonic Secret.
3. Keep only one payment backend under `spec.paymentBackend`.
4. Remove optional blocks you do not need rather than leaving placeholder values in place.
