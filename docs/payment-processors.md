# Payment processors

Use `spec.paymentBackend.grpcProcessor` when the mint should talk to a gRPC payment processor instead of LND, CLN, LNBits, or `fakeWallet`.

This is the right place for Spark/Breez, Stripe, or custom payment integrations.

## How the operator wires CDK

When `spec.paymentBackend.grpcProcessor` is set, the operator:

- sets CDK's Lightning backend to `grpcprocessor`
- renders the `[grpc_processor]` section in the generated `config.toml`
- mounts client TLS materials when `tlsSecretRef` is provided
- optionally injects a sidecar container named `grpc-processor` into the mint pod

That means you only describe the processor endpoint once in `CashuMint`; the operator handles the pod wiring and config file generation.

## External gRPC processor

Use this when the payment processor already runs as its own Deployment or Service.

```yaml
spec:
  paymentBackend:
    grpcProcessor:
      address: https://payments.cashu.svc.cluster.local
      port: 50051
      supportedUnits:
        - sat
      tlsSecretRef:
        name: grpc-processor-client
        key: client.crt
```

### Important details

- `address` should include the scheme you want CDK to use. Use `https://...` for TLS and `http://...` for plaintext.
- `port` defaults to `50051`.
- `supportedUnits` defaults to `["sat"]`.
- The Secret named in `tlsSecretRef.name` should contain the full client bundle expected by CDK:
  - `client.crt`
  - `client.key`
  - `ca.crt`

The `key` field is still required because the CRD uses `SecretKeySelector`, but the operator mounts the entire Secret by name.

See the full example: [`mint_v1alpha1_cashumint_grpc_processor_external.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_grpc_processor_external.yaml)

## Sidecar gRPC processor

Use a sidecar when the payment processor should live inside the same pod as `mintd`.

```yaml
spec:
  paymentBackend:
    grpcProcessor:
      port: 50051
      supportedUnits:
        - sat
      sidecarProcessor:
        enabled: true
        image: ghcr.io/acme/cdk-custom-processor:1.0.0
        imagePullPolicy: IfNotPresent
        workingDir: /data/processor
        env:
          - name: SERVER_ADDR
            value: "0.0.0.0"
          - name: SERVER_PORT
            value: "50051"
```

### What the sidecar gives you

- a second container named `grpc-processor`
- the same pod lifecycle as the mint
- a shared data volume when `workingDir` is set
- optional sidecar TLS Secret mounts

When `workingDir` is set, the operator mounts the mint's data volume into that directory using the `sidecar-processor` subpath. This is a convenient place to keep processor state separate from the rest of `/data`.

### Address defaults

If `sidecarProcessor.enabled=true` and you do not set `address`, the operator writes:

```toml
[grpc_processor]
addr = "http://127.0.0.1"
port = 50051
```

That is correct for plaintext loopback traffic inside the pod.

If you enable sidecar TLS, override the address explicitly:

```yaml
spec:
  paymentBackend:
    grpcProcessor:
      address: https://127.0.0.1
      port: 50051
      sidecarProcessor:
        enabled: true
        enableTLS: true
        tlsSecretRef:
          name: grpc-sidecar-server
          key: tls.crt
```

Without that override, the default remains `http://127.0.0.1`.

### Sidecar TLS expectations

When `enableTLS=true`, `sidecarProcessor.tlsSecretRef` is required. The operator mounts the named Secret at `/secrets/sidecar-tls`. Your sidecar image is responsible for reading the mounted files and serving TLS correctly.

## CDK + payment processor patterns in this repository

| Pattern | Sample |
| --- | --- |
| External gRPC processor with client TLS | [`mint_v1alpha1_cashumint_grpc_processor_external.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_grpc_processor_external.yaml) |
| Spark/Breez sidecar | [`mint_v1alpha1_cashumint_spark_breez.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_spark_breez.yaml) |
| Spark/Breez sidecar with example Secrets | [`mint_v1alpha1_cashumint_spark_processor.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_spark_processor.yaml) |
| Stripe sidecar | [`mint_v1alpha1_cashumint_stripe_processor.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_stripe_processor.yaml) |
| Production-style external gRPC processor | [`mint_v1alpha1_cashumint_production.yaml`](https://github.com/asmogo/cashu-operator/blob/main/config/samples/mint_v1alpha1_cashumint_production.yaml) |

## Troubleshooting checklist

1. Make sure only one backend is set under `spec.paymentBackend`.
2. If you are not using a sidecar, `spec.paymentBackend.grpcProcessor.address` must be set.
3. If you are using a sidecar, `sidecarProcessor.image` is required.
4. If you enable sidecar TLS, set both `sidecarProcessor.enableTLS=true` and `sidecarProcessor.tlsSecretRef`.
5. If the mint should connect over TLS, make sure the `address` starts with `https://`.
