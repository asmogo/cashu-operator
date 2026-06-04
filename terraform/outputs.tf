output "cluster_name" {
  value = google_container_cluster.primary.name
}

output "cluster_endpoint" {
  value = google_container_cluster.primary.endpoint
}

output "mint_external_ip" {
  description = "Point the DNS A record for var.domain at this IP."
  value       = google_compute_global_address.mint.address
}

output "kbs_internal_ip" {
  description = "Internal IP the pod VMs use to reach KBS (baked into initdata)."
  value       = google_compute_address.kbs_internal.address
}

output "kbs_admin_private_key_pem" {
  description = "Generated KBS admin private key (used by kbs-client). Sensitive."
  value       = tls_private_key.kbs_admin.private_key_pem
  sensitive   = true
}

output "mint_url" {
  value = "https://${var.domain}/v1/info"
}

output "get_credentials_command" {
  value = "gcloud container clusters get-credentials ${var.cluster_name} --zone ${var.zone} --project ${var.project_id}"
}
