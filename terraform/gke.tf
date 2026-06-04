# ─────────────────────────────────────────────────────────────────────────────
# Project APIs
# ─────────────────────────────────────────────────────────────────────────────
locals {
  apis = [
    "container.googleapis.com",
    "compute.googleapis.com",
    "iam.googleapis.com",
    "artifactregistry.googleapis.com",
    "storage.googleapis.com",
  ]
}

resource "google_project_service" "apis" {
  for_each           = toset(local.apis)
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# ─────────────────────────────────────────────────────────────────────────────
# Service account used by the peer-pods worker node pool. cloud-api-adaptor
# runs on these nodes and uses ADC (the node SA) to create confidential pod VMs.
# ─────────────────────────────────────────────────────────────────────────────
resource "google_service_account" "peerpods" {
  project      = var.project_id
  account_id   = "peerpods"
  display_name = "Peer-pods cloud-api-adaptor"
  depends_on   = [google_project_service.apis]
}

resource "google_project_iam_member" "peerpods_roles" {
  for_each = toset([
    "roles/compute.instanceAdmin.v1",
    "roles/iam.serviceAccountUser",
    "roles/artifactregistry.reader",
  ])
  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.peerpods.email}"
}

# ─────────────────────────────────────────────────────────────────────────────
# GKE cluster (Standard) + node pools
# ─────────────────────────────────────────────────────────────────────────────
resource "google_container_cluster" "primary" {
  name     = var.cluster_name
  location = var.zone
  project  = var.project_id

  # Manage node pools separately.
  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = false

  network    = var.network
  subnetwork = var.subnetwork

  release_channel {
    channel = var.release_channel
  }

  # Container-native NEG / L7 ingress + Workload Identity (as in the live cluster).
  addons_config {
    http_load_balancing {
      disabled = false
    }
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  ip_allocation_policy {}

  depends_on = [google_project_service.apis]
}

resource "google_container_node_pool" "default" {
  name     = "default-pool"
  cluster  = google_container_cluster.primary.id
  location = var.zone

  initial_node_count = 1

  node_config {
    machine_type = var.default_pool_machine_type
    disk_size_gb = 100
    disk_type    = "pd-standard"
    image_type   = "COS_CONTAINERD"
    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }
}

# Worker pool that hosts cloud-api-adaptor + kata-deploy. Uses the peerpods SA so
# CAA can create confidential VMs via ADC. Labeled so the mint can target it.
resource "google_container_node_pool" "worker" {
  name     = "peerpods-worker"
  cluster  = google_container_cluster.primary.id
  location = var.zone

  initial_node_count = 1

  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    machine_type    = var.worker_pool_machine_type
    disk_size_gb    = 100
    disk_type       = "pd-balanced"
    image_type      = "UBUNTU_CONTAINERD" # required for kata-deploy host tooling
    service_account = google_service_account.peerpods.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]

    labels = {
      (var.worker_node_label) = "true"
    }
  }
}

# ─────────────────────────────────────────────────────────────────────────────
# Networking: external static IP for the mint ingress + reserved internal IP for
# the KBS internal load balancer (reachable from pod VMs in the same VPC).
# ─────────────────────────────────────────────────────────────────────────────
resource "google_compute_global_address" "mint" {
  name    = var.static_ip_name
  project = var.project_id
}

resource "google_compute_address" "kbs_internal" {
  name         = "kbs-internal-ip"
  project      = var.project_id
  region       = var.region
  subnetwork   = var.subnetwork
  address_type = "INTERNAL"
}

# Allow the cloud-api-adaptor forwarder port between nodes and pod VMs in the VPC.
resource "google_compute_firewall" "peerpods_forwarder" {
  name      = "allow-peerpods-15150"
  project   = var.project_id
  network   = var.network
  direction = "INGRESS"

  allow {
    protocol = "tcp"
    ports    = ["15150"]
  }
  # Internal VPC ranges (nodes + pod VMs). Adjust if your subnet differs.
  source_ranges = ["10.128.0.0/9"]
}

# ─────────────────────────────────────────────────────────────────────────────
# Optional GCS bucket for pod VM image tarballs (used only when building images).
# ─────────────────────────────────────────────────────────────────────────────
resource "google_storage_bucket" "podvm_images" {
  count                       = var.podvm_images_bucket == "" ? 0 : 1
  name                        = var.podvm_images_bucket
  project                     = var.project_id
  location                    = var.region
  uniform_bucket_level_access = true
  force_destroy               = true
  depends_on                  = [google_project_service.apis]
}
