# ─────────────────────────────────────────────────────────────────────────────
# The mint workload (tee-mint) running on the kata-remote (peer-pods) runtime.
#
# The mnemonic env vars carry only a sealed *reference*; the value is fetched
# from KBS and unsealed inside the TEE at container start (see locals.tf and the
# initdata template). No plaintext mnemonic is stored in a workload Secret.
# ─────────────────────────────────────────────────────────────────────────────

# Static: PVC + Service.
data "kubectl_path_documents" "mint_static" {
  pattern = "${path.module}/manifests/mint/*.yaml"
}

resource "kubectl_manifest" "mint_static" {
  for_each   = data.kubectl_path_documents.mint_static.manifests
  yaml_body  = each.value
  depends_on = [google_container_node_pool.worker]
}

# Mint config file (ConfigMap).
resource "kubectl_manifest" "mint_config" {
  yaml_body = templatefile("${path.module}/templates/mint-config.yaml.tftpl", {
    domain           = var.domain
    mint_name        = var.mint_name
    mint_description = var.mint_description
    bdk_network      = var.bdk_network
    bdk_esplora_url  = var.bdk_esplora_url
    bdk_num_confs    = var.bdk_num_confs
  })
  depends_on = [google_container_node_pool.worker]
}

# ManagedCertificate + Ingress (external HTTPS).
resource "kubectl_manifest" "mint_ingress" {
  for_each = {
    for i, doc in split("\n---\n", templatefile("${path.module}/templates/mint-ingress.yaml.tftpl", {
      domain         = var.domain
      cert_name      = var.managed_cert_name
      static_ip_name = google_compute_global_address.mint.name
    })) : tostring(i) => doc
  }
  yaml_body = each.value
  depends_on = [
    kubectl_manifest.mint_static,
    google_compute_global_address.mint,
  ]
}

# The Deployment. Depends on:
#  - the CcRuntime being applied (so the operator can provide kata-remote),
#  - KBS + its provisioning Jobs (so the mnemonic resource exists to unseal).
resource "kubectl_manifest" "mint_deployment" {
  yaml_body = templatefile("${path.module}/templates/mint-deployment.yaml.tftpl", {
    initdata_b64       = local.initdata_b64
    podvm_machine_type = var.podvm_machine_type
    sealed_secret      = local.sealed_secret
    mint_image         = var.mint_image
    bdk_network        = var.bdk_network
    bdk_esplora_url    = var.bdk_esplora_url
    bdk_num_confs      = var.bdk_num_confs
    mint_name          = var.mint_name
    mint_description   = var.mint_description
    worker_label       = var.worker_node_label
  })

  depends_on = [
    kubectl_manifest.mint_static,
    kubectl_manifest.mint_config,
    kubectl_manifest.ccruntime,
    kubectl_manifest.caa,
    kubectl_manifest.kbs_provision,
  ]
}
