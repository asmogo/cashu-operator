locals {
  # The KBS is exposed to pod VMs via an internal L4 load balancer at a reserved
  # internal IP (stable across node churn). The in-VM CDH/AA reach it here.
  kbs_internal_ip = google_compute_address.kbs_internal.address
  kbs_url         = "http://${local.kbs_internal_ip}:8080"
  kbs_cluster_dns = "http://kbs.coco-tenant.svc.cluster.local:8080"

  # initdata TOML (aa.toml + cdh.toml). Encoded as base64(gzip(toml)) which is
  # exactly what cloud-api-adaptor's initdata.Encode expects, and what the
  # `io.katacontainers.config.hypervisor.cc_init_data` annotation consumes.
  initdata_toml = templatefile("${path.module}/templates/initdata.toml.tftpl", {
    kbs_url = local.kbs_url
  })
  initdata_b64 = base64gzip(local.initdata_toml)

  # Sealed-secret "vault" reference. Only a *reference* to the KBS resource —
  # no plaintext. skip_sealed_secret_verification=true in cdh.toml means CDH
  # reads section[2] (the payload) without verifying the JWS signature, so the
  # header/signature segments are placeholders.
  sealed_payload = jsonencode({
    version           = "0.1.0"
    type              = "vault"
    name              = "kbs:///${var.kbs_resource_path}"
    provider          = "kbs"
    provider_settings = {}
    annotations       = {}
  })
  # base64url, no padding (CDH decodes section[2] as URL_SAFE_NO_PAD).
  sealed_payload_b64 = replace(replace(replace(base64encode(local.sealed_payload), "+", "-"), "/", "_"), "=", "")
  sealed_secret      = "sealed.fakejwsheader.${local.sealed_payload_b64}.fakesignature"
}
