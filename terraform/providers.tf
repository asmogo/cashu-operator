# ─────────────────────────────────────────────────────────────────────────────
# Providers. The kubernetes/kubectl providers are configured from the GKE
# cluster created in this same configuration. google_client_config supplies a
# short-lived OAuth token for the current credentials (ADC / gcloud).
#
# NOTE: because these providers depend on the cluster, a clean first run may
# require a targeted apply of the cluster first:
#     terraform apply -target=google_container_cluster.primary -target=google_container_node_pool.default -target=google_container_node_pool.worker
#     terraform apply
# ─────────────────────────────────────────────────────────────────────────────

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

data "google_client_config" "default" {}

provider "kubernetes" {
  host                   = "https://${google_container_cluster.primary.endpoint}"
  token                  = data.google_client_config.default.access_token
  cluster_ca_certificate = base64decode(google_container_cluster.primary.master_auth[0].cluster_ca_certificate)
}

provider "kubectl" {
  host                   = "https://${google_container_cluster.primary.endpoint}"
  token                  = data.google_client_config.default.access_token
  cluster_ca_certificate = base64decode(google_container_cluster.primary.master_auth[0].cluster_ca_certificate)
  load_config_file       = false
}
