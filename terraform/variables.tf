# GCP project / location
variable "project_id" {
  type        = string
  description = "GCP project ID."
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "zone" {
  type    = string
  default = "us-central1-a"
}

variable "network" {
  type        = string
  default     = "default"
  description = "VPC network name (pod VMs and nodes must share this network)."
}

variable "subnetwork" {
  type        = string
  default     = "default"
  description = "Subnetwork name in the chosen region."
}

# GKE
variable "cluster_name" {
  type    = string
  default = "cluster-1"
}

variable "release_channel" {
  type    = string
  default = "REGULAR"
}

variable "default_pool_machine_type" {
  type    = string
  default = "n2d-standard-2"
}

variable "worker_pool_machine_type" {
  type        = string
  default     = "e2-standard-4"
  description = "Machine type for the peer-pods worker node pool (runs CAA + kata-deploy)."
}

variable "worker_node_label" {
  type        = string
  default     = "cashu-coco-peer"
  description = "Label key set on the worker pool and used by the mint nodeSelector."
}

# Peer-pods / cloud-api-adaptor pod VM settings
variable "podvm_image_name" {
  type        = string
  description = <<-EOT
    Full GCP image resource path for the confidential pod VM image, e.g.
    /projects/<project>/global/images/coco-podvm-fedora-mkosi-tee-amd-v0210-tdx
    This image is built out-of-band with mkosi (see README) and must already exist.
  EOT
}

variable "podvm_machine_type" {
  type    = string
  default = "c3-standard-4"
}

variable "podvm_confidential_type" {
  type    = string
  default = "TDX"
}

variable "podvm_disk_type" {
  type    = string
  default = "pd-balanced"
}

variable "podvm_images_bucket" {
  type        = string
  default     = ""
  description = "Optional GCS bucket name to (re)create for hosting pod VM image tarballs. Empty = skip."
}

# Mint / ingress
variable "domain" {
  type        = string
  description = "Public hostname for the mint, e.g. tee.ucash.space (DNS A record must point at the static IP)."
}

variable "static_ip_name" {
  type    = string
  default = "tee-ucash-ip"
}

variable "managed_cert_name" {
  type    = string
  default = "tee-ucash-cert"
}

variable "mint_image" {
  type    = string
  default = "cashubtc/mintd:0.17.0-rc.0"
}

variable "mint_name" {
  type    = string
  default = "ucash"
}

variable "mint_description" {
  type    = string
  default = "Confidential Cashu mint on Mutinynet (signet) - mnemonic sealed in an Intel TDX TEE, released only via remote attestation. Test sats only."
}

variable "bdk_network" {
  type    = string
  default = "signet"
}

variable "bdk_esplora_url" {
  type    = string
  default = "https://mutinynet.com/api"
}

variable "bdk_num_confs" {
  type    = number
  default = 2
}

# Secret input (sensitive)
variable "mint_mnemonic" {
  type        = string
  sensitive   = true
  description = "BIP39 wallet mnemonic for the mint. Provisioned into KBS; never stored in a workload Secret."
}

# KBS resource path the sealed-secret reference points at.
variable "kbs_resource_path" {
  type    = string
  default = "reponame/workload_key/key.bin"
}
