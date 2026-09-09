output "subscription_id" {
  description = "Subscription the resources live in."
  value       = var.subscription_id
}

output "resource_group_name" {
  description = "Resource group holding every resource this config creates."
  value       = azurerm_resource_group.this.name
}

output "location" {
  description = "Azure region used."
  value       = azurerm_resource_group.this.location
}

output "container_app_environment" {
  description = "Name of the Container Apps environment."
  value       = azurerm_container_app_environment.this.name
}

output "container_app_name" {
  description = "Name of the Container App."
  value       = azurerm_container_app.this.name
}

output "app_fqdn" {
  description = "Public hostname assigned to the app's ingress."
  value       = azurerm_container_app.this.ingress[0].fqdn
}

output "app_url" {
  description = "Base URL of the running service."
  value       = "https://${azurerm_container_app.this.ingress[0].fqdn}"
}

output "health_url" {
  description = "Liveness endpoint."
  value       = "https://${azurerm_container_app.this.ingress[0].fqdn}/health"
}

output "ready_url" {
  description = "Readiness endpoint."
  value       = "https://${azurerm_container_app.this.ingress[0].fqdn}/ready"
}

output "smoke_test" {
  description = "Quick end-to-end check once apply completes."
  value       = <<-EOT
    curl -s https://${azurerm_container_app.this.ingress[0].fqdn}/health
    curl -s https://${azurerm_container_app.this.ingress[0].fqdn}/ready
    curl -s -X POST https://${azurerm_container_app.this.ingress[0].fqdn}/events \
      -H 'Content-Type: application/json' \
      -d '{"event_id":"evt_001","payload":{"hello":"azure"}}'
  EOT
}
