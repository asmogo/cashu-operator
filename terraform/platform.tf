# ─────────────────────────────────────────────────────────────────────────────
# Platform layer: Confidential Containers operator + cloud-api-adaptor.
#
# Captured from the live cluster (terraform/manifests/platform). We apply the
# operator *inputs* (CRDs, operator core, RBAC, the CcRuntime CR, CAA) and let
# the operator regenerate the derived resources (kata-deploy daemonsets, the
# `kata-remote` RuntimeClass, etc.).
#
# Applied in dependency-ordered groups via the kubectl provider.
# ─────────────────────────────────────────────────────────────────────────────

# Group 1: namespaces + CoCo CRDs
data "kubectl_path_documents" "ns_crds" {
  pattern = "${path.module}/manifests/platform/0[01]-*.yaml"
}

resource "kubectl_manifest" "ns_crds" {
  for_each          = data.kubectl_path_documents.ns_crds.manifests
  yaml_body         = each.value
  server_side_apply = true # large CRDs exceed the client-side apply annotation limit

  depends_on = [
    google_container_node_pool.default,
    google_container_node_pool.worker,
  ]
}

# Group 2: operator core (deploy/sa/config/metrics svc) + cluster RBAC
data "kubectl_path_documents" "operator" {
  pattern = "${path.module}/manifests/platform/0[23]-*.yaml"
}

resource "kubectl_manifest" "operator" {
  for_each  = data.kubectl_path_documents.operator.manifests
  yaml_body = each.value

  depends_on = [kubectl_manifest.ns_crds]
}

# Group 3: peer-pods config (templated) + empty secret
resource "kubectl_manifest" "peer_pods_cm" {
  yaml_body = templatefile("${path.module}/templates/peer-pods-cm.yaml.tftpl", {
    project_id        = var.project_id
    zone              = var.zone
    network           = var.network
    machine_type      = var.podvm_machine_type
    confidential_type = var.podvm_confidential_type
    disk_type         = var.podvm_disk_type
    podvm_image_name  = var.podvm_image_name
  })
  depends_on = [kubectl_manifest.ns_crds]
}

resource "kubectl_manifest" "peer_pods_secret" {
  yaml_body  = file("${path.module}/manifests/platform/06-peer-pods-secret.yaml")
  depends_on = [kubectl_manifest.ns_crds]
}

# Group 4: CcRuntime CR — triggers the operator to install kata + the runtime.
data "kubectl_path_documents" "ccruntime" {
  pattern = "${path.module}/manifests/platform/04-*.yaml"
}

resource "kubectl_manifest" "ccruntime" {
  for_each  = data.kubectl_path_documents.ccruntime.manifests
  yaml_body = each.value

  depends_on = [kubectl_manifest.operator]
}

# Group 5: cloud-api-adaptor daemonset + SA + ca-bundle
data "kubectl_path_documents" "caa" {
  pattern = "${path.module}/manifests/platform/05-*.yaml"
}

resource "kubectl_manifest" "caa" {
  for_each  = data.kubectl_path_documents.caa.manifests
  yaml_body = each.value

  depends_on = [
    kubectl_manifest.operator,
    kubectl_manifest.peer_pods_cm,
    kubectl_manifest.peer_pods_secret,
  ]
}
