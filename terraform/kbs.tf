# ─────────────────────────────────────────────────────────────────────────────
# Trustee Key Broker Service (KBS) in the coco-tenant namespace.
#
# - Generates a FRESH ed25519 admin keypair (public PEM mounted into KBS for the
#   "Simple" admin persona; private PEM used by the provisioning Jobs).
# - Storage is a PVC so the mnemonic resource + resource policy survive restarts.
# - Exposed to pod VMs via an internal L4 load balancer at a reserved IP.
# - The mnemonic is provisioned into KBS via `kbs-client set-resource` (admin
#   API) — never mounted into the workload.
# ─────────────────────────────────────────────────────────────────────────────

resource "tls_private_key" "kbs_admin" {
  algorithm = "ED25519"
}

resource "kubernetes_secret_v1" "kbs_auth_public_key" {
  metadata {
    name      = "kbs-auth-public-key"
    namespace = "coco-tenant"
  }
  data = {
    "kbs.pem" = tls_private_key.kbs_admin.public_key_pem
  }
  depends_on = [kubectl_manifest.ns_crds]
}

resource "kubernetes_secret_v1" "kbs_admin_key" {
  metadata {
    name      = "kbs-admin-key"
    namespace = "coco-tenant"
  }
  data = {
    "kbs.key" = tls_private_key.kbs_admin.private_key_pem
  }
  depends_on = [kubectl_manifest.ns_crds]
}

# Provisioning source for the mnemonic (KBS namespace only; the mint never
# references this — it is read once by the set-resource Job).
resource "kubernetes_secret_v1" "kbs_mnemonic_prov" {
  metadata {
    name      = "kbs-mnemonic-prov"
    namespace = "coco-tenant"
  }
  data = {
    mnemonic = var.mint_mnemonic
  }
  depends_on = [kubectl_manifest.ns_crds]
}

# KBS PVC + config + allow-policy + deployment (static manifests).
data "kubectl_path_documents" "kbs" {
  pattern = "${path.module}/manifests/kbs/*.yaml"
}

resource "kubectl_manifest" "kbs" {
  for_each  = data.kubectl_path_documents.kbs.manifests
  yaml_body = each.value

  depends_on = [
    kubectl_manifest.ns_crds,
    kubernetes_secret_v1.kbs_auth_public_key,
    kubernetes_secret_v1.kbs_admin_key,
  ]
}

# Internal LB service at the reserved internal IP.
resource "kubectl_manifest" "kbs_service" {
  yaml_body = templatefile("${path.module}/templates/kbs-service.yaml.tftpl", {
    kbs_ip = local.kbs_internal_ip
  })
  depends_on = [kubectl_manifest.kbs]
}

# Provisioning Jobs: set the permissive resource policy and store the mnemonic.
# Job name suffix changes when the mnemonic changes (Jobs are immutable).
resource "kubectl_manifest" "kbs_provision" {
  for_each = {
    for i, doc in split("\n---\n", templatefile("${path.module}/templates/kbs-jobs.yaml.tftpl", {
      kbs_dns       = local.kbs_cluster_dns
      resource_path = var.kbs_resource_path
      # Truncated hash (not reversible) so the Jobs get a new name when the
      # mnemonic changes (Jobs are immutable). nonsensitive() is required for
      # use as a for_each key below.
      suffix = nonsensitive(substr(sha256(var.mint_mnemonic), 0, 8))
    })) : tostring(i) => doc
  }
  yaml_body = each.value

  depends_on = [
    kubectl_manifest.kbs,
    kubectl_manifest.kbs_service,
    kubernetes_secret_v1.kbs_admin_key,
    kubernetes_secret_v1.kbs_mnemonic_prov,
  ]
}
